package nxos

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
	"github.com/carlosrabelo/negev/negev/internal/platform/parseutil"
)

var vlanLineRegex = regexp.MustCompile(`^\s*(?:vlan\s+)?(\d{1,4})\b`)
var macLineRegex = regexp.MustCompile(`(?i)^\s*[*+GC]?\s*(\d+)\s+([0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\s+(dynamic|static)\s+\S+\s+\S+\s+\S+\s+(\S+)`)

var statusTokens = map[string]bool{
	"connected": true,
	"notconnec": true,
	"disabled":  true,
	"sfpAbsent": true,
	"xcvrAbsen": true,
	"linkFlapE": true,
	"errDisabl": true,
	"noOperMem": true,
	"down":      true,
	"inactive":  true,
	"up":        true,
	"unknown":   true,
}

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "nxos"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	return strings.Contains(lower, "nx-os") || strings.Contains(lower, "nxos"), nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func normalizeInterface(name string) string {
	compact := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	if compact == "" {
		return compact
	}
	lower := strings.ToLower(compact)

	tryPrefix := func(prefix, canonical string) (string, bool) {
		if !strings.HasPrefix(lower, prefix) {
			return "", false
		}
		rest := compact[len(prefix):]
		if rest == "" {
			return "", false
		}
		return canonical + rest, true
	}

	if out, ok := tryPrefix("ethernet", "Ethernet"); ok {
		return out
	}
	if out, ok := tryPrefix("eth", "Ethernet"); ok {
		return out
	}
	if out, ok := tryPrefix("port-channel", "port-channel"); ok {
		return out
	}
	if out, ok := tryPrefix("po", "port-channel"); ok {
		return out
	}
	return compact
}

func isSwitchInterface(name string) bool {
	if strings.HasPrefix(name, "Ethernet") {
		path := strings.TrimPrefix(name, "Ethernet")
		return isEthernetPath(path)
	}
	if strings.HasPrefix(name, "port-channel") {
		path := strings.TrimPrefix(name, "port-channel")
		_, err := strconv.Atoi(path)
		return err == nil && path != ""
	}
	return false
}

func isEthernetPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func compareInterfaceNames(a, b string) bool {
	rank := func(s string) int {
		if strings.HasPrefix(s, "Ethernet") {
			return 0
		}
		if strings.HasPrefix(s, "port-channel") {
			return 1
		}
		return 2
	}
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra < rb
	}
	pathA := a
	pathB := b
	switch ra {
	case 0:
		pathA = strings.TrimPrefix(a, "Ethernet")
		pathB = strings.TrimPrefix(b, "Ethernet")
	case 1:
		pathA = strings.TrimPrefix(a, "port-channel")
		pathB = strings.TrimPrefix(b, "port-channel")
	}
	pa := strings.Split(pathA, "/")
	pb := strings.Split(pathB, "/")
	n := len(pa)
	if len(pb) < n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan brief")
	if err != nil || out == "" {
		out, err = repo.ExecuteCommand("show vlan")
		if err != nil {
			return nil, err
		}
	}
	return parseVLANs(out), nil
}

func parseVLANs(output string) []string {
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range strings.Split(output, "\n") {
		if parseutil.IsSeparatorLine(line) {
			continue
		}
		m := vlanLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := m[1]
		if !seen[id] {
			seen[id] = true
			vlans = append(vlans, id)
		}
	}
	return vlans
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show interface trunk")
	if err != nil {
		return nil, err
	}
	return parseTrunkInterfaces(out), nil
}

func parseTrunkInterfaces(output string) []string {
	seen := make(map[string]bool)
	var ifaces []string
	for _, line := range strings.Split(output, "\n") {
		for _, field := range strings.Fields(line) {
			name := normalizeInterface(field)
			if !isSwitchInterface(name) {
				continue
			}
			if !seen[name] {
				seen[name] = true
				ifaces = append(ifaces, name)
			}
		}
	}
	return ifaces
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	out, err := repo.ExecuteCommand("show interface status")
	if err != nil {
		return nil, err
	}
	return parseActivePorts(out), nil
}

func parseActivePorts(output string) []entities.Port {
	var ports []entities.Port
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isSwitchInterface(iface) {
			continue
		}
		statusIdx := -1
		for i, f := range fields[1:] {
			if statusTokens[f] {
				statusIdx = i + 1
				break
			}
		}
		if statusIdx < 0 || statusIdx+1 >= len(fields) {
			continue
		}
		status := fields[statusIdx]
		vlan := fields[statusIdx+1]
		if status != "connected" && status != "up" {
			continue
		}
		if !isAccessVLAN(vlan) {
			continue
		}
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}
	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func isAccessVLAN(s string) bool {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return err == nil && n >= 1 && n <= 4094
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo)
	if err != nil {
		return nil, err
	}
	trunkSet := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		trunkSet[t] = true
	}

	out, err := repo.ExecuteCommand("show mac address-table")
	if err != nil || out == "" {
		out, err = repo.ExecuteCommand("show mac address-table dynamic")
		if err != nil {
			return nil, err
		}
	}
	return parseMacTable(out, trunkSet), nil
}

func parseMacTable(output string, trunkSet map[string]bool) []entities.Device {
	var devices []entities.Device
	for _, line := range strings.Split(output, "\n") {
		m := macLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		iface := normalizeInterface(m[4])
		if !isSwitchInterface(iface) {
			continue
		}
		if trunkSet[iface] {
			continue
		}
		macPlain := strings.ToLower(strings.ReplaceAll(m[2], ".", ""))
		devices = append(devices, entities.Device{
			Vlan:      m[1],
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
		})
	}
	return devices
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return []string{
		"configure terminal",
		"interface " + port.Interface,
		"switchport",
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"configure terminal",
		"vlan " + vlan,
		"exit",
		"end",
	}
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure terminal",
		"no vlan " + vlan,
		"end",
	}
}

func (d *Driver) SaveCommands() []string {
	return []string{"copy running-config startup-config"}
}

func (d *Driver) ClearCache() {}

var nxosErrorPatterns = []string{
	"invalid command",
	"incomplete command",
	"ambiguous command",
	"invalid range",
	"error:",
}

func (d *Driver) IsCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range nxosErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
