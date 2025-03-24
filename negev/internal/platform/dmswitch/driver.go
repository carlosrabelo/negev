package dmswitch

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

var (
	vlanLineRegex  = regexp.MustCompile(`(?i)^\s*(?:vlan\s+)?(\d{1,4})\b`)
	infoPortRegex  = regexp.MustCompile(`(?i)^Information of Eth\s+(\d+\s*/\s*\d+)`)
	macLineRegex   = regexp.MustCompile(`(?i)^\s*\d+\s+(Eth\s+\d+\s*/\s*\d+)\s+([0-9a-f]{2}(?:-[0-9a-f]{2}){5})\s+(\d+)\s+(Static|Dynamic)\b`)
	ethCompactRe   = regexp.MustCompile(`^eth(?:ernet)?(\d+/\d+)$`)
	poCompactRe    = regexp.MustCompile(`^(?:port-channel|po)(\d+)$`)
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dmswitch"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "dm1200") || strings.Contains(lower, "1200e") {
		return false, nil
	}
	if strings.Contains(lower, "dmswitch") {
		return true, nil
	}
	if strings.Contains(lower, "datacom") && (strings.Contains(lower, "3324") || strings.Contains(lower, "3224")) {
		return true, nil
	}
	return false, nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func normalizeInterface(name string) string {
	compact := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	lower := strings.ToLower(compact)
	if m := ethCompactRe.FindStringSubmatch(lower); m != nil {
		return "ethernet" + m[1]
	}
	if m := poCompactRe.FindStringSubmatch(lower); m != nil {
		return "port-channel" + m[1]
	}
	return compact
}

func isEthernetInterface(name string) bool {
	return strings.HasPrefix(name, "ethernet") && ethCompactRe.MatchString(name)
}

func isSwitchInterface(name string) bool {
	return ethCompactRe.MatchString(name) || poCompactRe.MatchString(name)
}

func compareInterfaceNames(a, b string) bool {
	rank := func(s string) int {
		if strings.HasPrefix(s, "ethernet") {
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
	path := func(s string) (int, int) {
		s = strings.TrimPrefix(s, "ethernet")
		s = strings.TrimPrefix(s, "port-channel")
		parts := strings.Split(s, "/")
		if len(parts) == 1 {
			n, _ := strconv.Atoi(parts[0])
			return n, 0
		}
		if len(parts) < 2 {
			return 0, 0
		}
		u, _ := strconv.Atoi(parts[0])
		p, _ := strconv.Atoi(parts[1])
		return u, p
	}
	u1, p1 := path(a)
	u2, p2 := path(b)
	if u1 != u2 {
		return u1 < u2
	}
	return p1 < p2
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan")
	if err != nil || out == "" {
		out, err = repo.ExecuteCommand("show vlan brief")
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
	out, err := getSwitchportOutput(repo)
	if err != nil {
		return nil, err
	}
	return parseTrunkInterfaces(out), nil
}

func parseTrunkInterfaces(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	add := func(name string) {
		if name == "" || !isSwitchInterface(name) || seen[name] {
			return
		}
		seen[name] = true
		trunks = append(trunks, name)
	}

	var currentIface string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "interface ethernet") || strings.HasPrefix(lower, "interface eth") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 {
				currentIface = normalizeInterface(fields[1] + " " + fields[2])
			} else if len(fields) >= 2 {
				currentIface = normalizeInterface(fields[1])
			}
			continue
		}
		if strings.HasPrefix(trimmed, "Allowed VLANs:") && strings.Contains(trimmed, "(t)") {
			add(currentIface)
			continue
		}
		if strings.Contains(lower, "trunk") && currentIface != "" {
			// mode line mentioning trunking
			if strings.Contains(lower, "mode") || strings.Contains(lower, "trunking") {
				add(currentIface)
			}
		}
	}
	return trunks
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	statusOut, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, err
	}
	swOut, err := getSwitchportOutput(repo)
	if err != nil {
		return nil, err
	}
	return parseActivePorts(statusOut, parseSwitchportVLANs(swOut)), nil
}

func parseSwitchportVLANs(output string) map[string]string {
	result := make(map[string]string)
	var currentIface string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "interface ethernet") || strings.HasPrefix(lower, "interface eth") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 {
				currentIface = normalizeInterface(fields[1] + " " + fields[2])
			} else if len(fields) >= 2 {
				currentIface = normalizeInterface(fields[1])
			}
			continue
		}
		if strings.HasPrefix(trimmed, "Native VLAN:") || strings.HasPrefix(trimmed, "PVID:") {
			vlan := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1])
			if currentIface != "" && isAccessVLAN(vlan) {
				result[currentIface] = vlan
			}
		}
	}
	return result
}

func parseActivePorts(statusOutput string, switchportVLANs map[string]string) []entities.Port {
	var ports []entities.Port
	seen := make(map[string]bool)
	add := func(iface, vlan string) {
		if iface == "" || !isEthernetInterface(iface) || seen[iface] {
			return
		}
		if vlan == "" || strings.EqualFold(vlan, "trunk") || !isAccessVLAN(vlan) {
			return
		}
		seen[iface] = true
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}

	var currentIface string
	for _, line := range strings.Split(statusOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := infoPortRegex.FindStringSubmatch(trimmed); m != nil {
			currentIface = normalizeInterface("Eth " + m[1])
			continue
		}
		if strings.HasPrefix(trimmed, "Link status:") {
			status := strings.TrimSpace(strings.TrimPrefix(trimmed, "Link status:"))
			if strings.EqualFold(status, "Up") && currentIface != "" {
				add(currentIface, switchportVLANs[currentIface])
			}
			currentIface = ""
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isEthernetInterface(iface) {
			continue
		}
		statusIdx := -1
		for i, f := range fields[1:] {
			lf := strings.ToLower(f)
			if lf == "up" || lf == "connected" || lf == "down" || lf == "notconnect" || lf == "disabled" {
				statusIdx = i + 1
				break
			}
		}
		if statusIdx < 0 || statusIdx+1 >= len(fields) {
			continue
		}
		status := strings.ToLower(fields[statusIdx])
		if status != "up" && status != "connected" {
			continue
		}
		vlan := fields[statusIdx+1]
		add(iface, vlan)
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

	out, err := repo.ExecuteCommand("show mac-address-table")
	if err != nil {
		return nil, err
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
		iface := normalizeInterface(m[1])
		if trunkSet[iface] {
			continue
		}
		macPlain := strings.ToLower(strings.ReplaceAll(m[2], "-", ""))
		devices = append(devices, entities.Device{
			Vlan:      m[3],
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
		})
	}
	return devices
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return nil
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return nil
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return nil
}

func (d *Driver) SaveCommands() []string {
	return nil
}

func (d *Driver) ClearCache() {
	clearSwitchportCache()
}

func (d *Driver) IsCommandError(output string) bool {
	return false
}
