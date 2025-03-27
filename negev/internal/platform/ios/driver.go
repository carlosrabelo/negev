package ios

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

var vlanLineRegex = regexp.MustCompile(`^\s*(?:vlan\s+)?(\d{1,4})\b`)
var interfaceRegex = regexp.MustCompile(`^[A-Za-z-]+\d+(?:/\d+){0,2}$`)
var macTableRegex = regexp.MustCompile(`(?i)^\s*(\d+)\s+([0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\s+(DYNAMIC|STATIC)\s+(\S+)`)
var sviNameRegex = regexp.MustCompile(`(?i)^Vlan(\d+)$`)
var iosPromptPattern = regexp.MustCompile(`(?m)^[A-Za-z0-9_.-]+(?:-[A-Za-z0-9_.-]+)*[>#]\s*$`)
var iosPagerPattern = regexp.MustCompile(`(?m)--More--`)

type Driver struct{}

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "ios"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "nx-os") || strings.Contains(lower, "nxos") {
		return false, nil
	}
	return strings.Contains(lower, "cisco ios"), nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "enable\n"},
		{WaitFor: "Password:", SendCmd: "ENABLE_PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
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
	lines := strings.Split(output, "\n")
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range lines {
		m := vlanLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v := m[1]
		if !seen[v] {
			seen[v] = true
			vlans = append(vlans, v)
		}
	}
	return vlans
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show interfaces trunk")
	if err != nil {
		return nil, err
	}
	return parseTrunkInterfaces(out), nil
}

func parseTrunkInterfaces(output string) []string {
	lines := strings.Split(output, "\n")
	seen := make(map[string]bool)
	var ifaces []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if interfaceRegex.MatchString(fields[0]) {
			name := fields[0]
			if !seen[name] {
				seen[name] = true
				ifaces = append(ifaces, name)
			}
		}
	}
	return ifaces
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	out, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, err
	}
	return parseActivePorts(out), nil
}

var statusKeywords = []string{"connected", "up", "forward", "active", "link-up"}

func parseActivePorts(output string) []entities.Port {
	lines := strings.Split(output, "\n")
	var ports []entities.Port
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if !interfaceRegex.MatchString(fields[0]) {
			continue
		}
		status := ""
		vlan := ""
		for i, f := range fields {
			if isStatusKeyword(f) {
				if i+1 < len(fields) && isVlanOrMode(fields[i+1]) {
					status = f
					vlan = fields[i+1]
					break
				}
			}
		}
		lowerStatus := strings.ToLower(status)
		if lowerStatus == "notconnect" || lowerStatus == "monitor" {
			continue
		}
		if status != "" && vlan != "" {
			lowerVlan := strings.ToLower(vlan)
			port := entities.Port{Interface: fields[0], Vlan: vlan}
			if lowerVlan == "routed" || lowerVlan == "routed-port" {
				port.IsRouted = true
			}
			if lowerVlan == "trunk" {
				port.IsTrunk = true
			}
			ports = append(ports, port)
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Interface < ports[j].Interface
	})
	return ports
}

func isVlanOrMode(s string) bool {
	s = strings.ToLower(s)
	if s == "trunk" || s == "routed" || s == "routed-port" || s == "access" {
		return true
	}
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return err == nil && n >= 1 && n <= 4094
}

func isStatusKeyword(s string) bool {
	for _, kw := range statusKeywords {
		if s == kw {
			return true
		}
	}
	return s == "notconnect"
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo)
	if err != nil {
		return nil, err
	}
	trunkSet := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		trunkSet[strings.ToLower(d.NormalizeInterface(t))] = true
	}

	out, err := repo.ExecuteCommand("show mac address-table")
	if err != nil || out == "" {
		out, err = repo.ExecuteCommand("show mac address-table dynamic")
		if err != nil {
			return nil, err
		}
	}
	return parseMacTable(out, trunkSet, d), nil
}

func parseMacTable(output string, trunkSet map[string]bool, d *Driver) []entities.Device {
	lines := strings.Split(output, "\n")
	var devices []entities.Device
	for _, line := range lines {
		m := macTableRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		iface := m[4]
		canon := d.NormalizeInterface(iface)
		if trunkSet[strings.ToLower(canon)] {
			continue
		}
		if !interfaceRegex.MatchString(iface) && !interfaceRegex.MatchString(canon) {
			continue
		}
		macDotted := m[2]
		macPlain := strings.ToLower(strings.ReplaceAll(macDotted, ".", ""))
		macFull := formatMac(macPlain)
		devices = append(devices, entities.Device{
			Vlan:      m[1],
			Mac:       macPlain,
			MacFull:   macFull,
			Interface: canon,
			IsStatic:  strings.EqualFold(m[3], "STATIC"),
		})
	}
	return devices
}

func formatMac(mac string) string {
	if len(mac) != 12 {
		return mac
	}
	return mac[0:2] + ":" + mac[2:4] + ":" + mac[4:6] + ":" + mac[6:8] + ":" + mac[8:10] + ":" + mac[10:12]
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return []string{
		"configure terminal",
		"interface " + port.Interface,
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
		"interface vlan " + vlan,
		"no shutdown",
		"end",
	}
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure terminal",
		"interface vlan " + vlan,
		"shutdown",
		"exit",
		"no interface vlan " + vlan,
		"exit",
		"no vlan " + vlan,
		"end",
	}
}

func (d *Driver) SaveCommands() []string {
	return []string{"write memory"}
}

func (d *Driver) ClearCache() {}

func (d *Driver) GetSessionProfile() entities.SessionProfile {
	return entities.SessionProfile{
		PromptPattern: iosPromptPattern,
		PagerPattern:  iosPagerPattern,
		PagerResponse: " ",
	}
}

func (d *Driver) NormalizeInterface(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	lower := strings.ToLower(name)
	type repl struct {
		prefix string
		full   string
	}
	// Longer prefixes first so "gi" does not match inside "gigabitethernet".
	for _, r := range []repl{
		{"gigabitethernet", "GigabitEthernet"},
		{"tengigabitethernet", "TenGigabitEthernet"},
		{"fastethernet", "FastEthernet"},
		{"port-channel", "Port-channel"},
		{"gi", "GigabitEthernet"},
		{"te", "TenGigabitEthernet"},
		{"fa", "FastEthernet"},
		{"po", "Port-channel"},
	} {
		if !strings.HasPrefix(lower, r.prefix) {
			continue
		}
		rest := name[len(r.prefix):]
		if rest == "" {
			return r.full
		}
		if rest[0] >= '0' && rest[0] <= '9' {
			return r.full + rest
		}
	}
	return name
}

func (d *Driver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show ip interface brief")
	if err != nil {
		return nil, err
	}
	return parseL3Interfaces(out), nil
}

func parseL3Interfaces(output string) []string {
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		m := sviNameRegex.FindStringSubmatch(fields[0])
		if m == nil {
			continue
		}
		v := m[1]
		if !seen[v] {
			seen[v] = true
			vlans = append(vlans, v)
		}
	}
	sort.Slice(vlans, func(i, j int) bool {
		return atoiSafe(vlans[i]) < atoiSafe(vlans[j])
	})
	return vlans
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (d *Driver) IsCommandError(output string) bool {
	return isIOSCommandError(output)
}

var iosErrorPatterns = []string{
	"invalid input",
	"unknown command",
	"incomplete command",
	"ambiguous command",
	"unrecognized command",
	"invalid command",
	"syntax error",
	"cannot find command",
}

func isIOSCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range iosErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
