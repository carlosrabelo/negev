package dmos

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

var vlanTableRegex = regexp.MustCompile(`^VLAN\s+(\d+)\s*(?:\[.*?\])?:\s*`)
var dmosPortRegex = regexp.MustCompile(`^Ethernet\d+/\d+$`)
var infoPortRegex = regexp.MustCompile(`^Information of Eth\s+(\d+/\d+)`)
var macLineRegex = regexp.MustCompile(`^\s*\d+\s+\w*\s+(Eth\s+\d+/\d+)\s+([0-9A-F:]+)\s+(\d+)\s+.*Learned`)

type Driver struct{}

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dmos"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	return strings.Contains(lower, "dmos") || strings.Contains(lower, "datacom"), nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan table")
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
		m := vlanTableRegex.FindStringSubmatch(line)
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
	out, err := getSwitchportOutput(repo)
	if err != nil {
		return nil, err
	}
	return parseDmOSTrunksFromSwitchport(out), nil
}

func parseDmOSTrunksFromSwitchport(output string) []string {
	lines := strings.Split(output, "\n")
	var trunks []string
	var currentIface string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "interface ethernet") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentIface = parts[1]
			}
		} else if strings.HasPrefix(trimmed, "Allowed VLANs:") && strings.Contains(trimmed, "(t)") {
			if currentIface != "" {
				trunks = append(trunks, currentIface)
			}
		}
	}
	return trunks
}

func parseDmOSTrunks(output string) []string {
	lines := strings.Split(output, "\n")
	var trunks []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if dmosPortRegex.MatchString(fields[0]) {
			trunks = append(trunks, fields[0])
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
	switchportVLANs := parseDmOSSwitchportVLANs(swOut)
	return parseActivePorts(statusOut, switchportVLANs), nil
}

func parseActivePorts(statusOutput string, switchportVLANs map[string]string) []entities.Port {
	lines := strings.Split(statusOutput, "\n")
	var ports []entities.Port
	var currentIface string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := infoPortRegex.FindStringSubmatch(trimmed)
		if m != nil {
			currentIface = "Ethernet" + m[1]
			continue
		}
		if strings.HasPrefix(trimmed, "Link status:") {
			status := strings.TrimPrefix(trimmed, "Link status:")
			status = strings.TrimSpace(status)
			if strings.EqualFold(status, "Up") && currentIface != "" {
				vlan := switchportVLANs[currentIface]
				ports = append(ports, entities.Port{Interface: currentIface, Vlan: vlan})
			}
			currentIface = ""
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func parseDmOSSwitchportVLANs(output string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")
	var currentIface string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "interface ethernet") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentIface = parts[1]
			}
		} else if strings.HasPrefix(trimmed, "Native VLAN:") {
			vlan := strings.TrimSpace(strings.TrimPrefix(trimmed, "Native VLAN:"))
			if currentIface != "" {
				result[currentIface] = vlan
			}
		}
	}
	return result
}

func compareInterfaceNames(a, b string) bool {
	extract := func(s string) (int, int) {
		parts := strings.Split(s, "/")
		if len(parts) < 2 {
			return 0, 0
		}
		unit, _ := strconv.Atoi(strings.TrimLeft(parts[0], "Ethernet"))
		port, _ := strconv.Atoi(parts[1])
		return unit, port
	}
	u1, p1 := extract(a)
	u2, p2 := extract(b)
	if u1 != u2 {
		return u1 < u2
	}
	return p1 < p2
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo)
	if err != nil {
		return nil, err
	}
	trunkSet := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		trunkSet[strings.ToLower(t)] = true
	}

	out, err := repo.ExecuteCommand("show mac-address-table")
	if err != nil {
		return nil, err
	}
	return parseMacTable(out, trunkSet), nil
}

func normalizeMAC(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.NewReplacer(":", "", ".", "").Replace(mac)
	return mac
}

func normalizePort(port string) string {
	port = strings.ToLower(port)
	if !strings.HasPrefix(port, "ethernet") {
		port = "ethernet" + strings.TrimPrefix(port, "Eth")
		port = strings.ReplaceAll(port, " ", "")
	}
	return strings.ReplaceAll(port, " ", "")
}

func parseMacTable(output string, trunkSet map[string]bool) []entities.Device {
	lines := strings.Split(output, "\n")
	var devices []entities.Device
	for _, line := range lines {
		m := macLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port := normalizePort(m[1])
		if trunkSet[port] {
			continue
		}
		macRaw := m[2]
		macPlain := normalizeMAC(macRaw)
		macFull := formatMAC(macPlain)
		devices = append(devices, entities.Device{
			Vlan:      m[3],
			Mac:       macPlain,
			MacFull:   macFull,
			Interface: port,
		})
	}
	return devices
}

func formatMAC(mac string) string {
	if len(mac) != 12 {
		return mac
	}
	return mac[0:2] + ":" + mac[2:4] + ":" + mac[4:6] + ":" + mac[6:8] + ":" + mac[8:10] + ":" + mac[10:12]
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return []string{
		"configure",
		"interface vlan " + vlan,
		"set-member untagged " + port.Interface,
		"exit",
		"interface " + port.Interface,
		"switchport native vlan " + vlan,
		"switchport acceptable-frame-type all",
		"exit",
		"end",
	}
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"configure",
		"interface vlan " + vlan,
		"exit",
		"end",
	}
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure",
		"no interface vlan " + vlan,
		"end",
	}
}

func (d *Driver) SaveCommands() []string {
	return []string{
		"copy running-config startup-config",
		"save",
	}
}

func (d *Driver) ClearCache() {
	clearSwitchportCache()
}

var dmosErrorPatterns = []string{
	"unknown command",
	"invalid",
	"incomplete",
	"syntax error",
}

func isDmOSCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range dmosErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

func (d *Driver) IsCommandError(output string) bool {
	return isDmOSCommandError(output)
}
