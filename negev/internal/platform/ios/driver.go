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
var interfaceRegex = regexp.MustCompile(`^[A-Za-z]+\d+(?:/\d+){0,2}$`)
var macTableRegex = regexp.MustCompile(`^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)`)

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
	return strings.Contains(strings.ToLower(out), "cisco ios"), nil
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

var statusKeywords = []string{"connected", "up", "forward", "monitor", "active", "link-up"}

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
		if status == "notconnect" {
			continue
		}
		if status != "" && vlan != "" {
			ports = append(ports, entities.Port{Interface: fields[0], Vlan: vlan})
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
		trunkSet[t] = true
	}

	out, err := repo.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, err
	}
	return parseMacTable(out, trunkSet), nil
}

func parseMacTable(output string, trunkSet map[string]bool) []entities.Device {
	lines := strings.Split(output, "\n")
	var devices []entities.Device
	for _, line := range lines {
		m := macTableRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		iface := m[3]
		if trunkSet[iface] {
			continue
		}
		if !interfaceRegex.MatchString(iface) {
			continue
		}
		macDotted := m[2]
		macPlain := strings.ReplaceAll(macDotted, ".", "")
		macFull := formatMac(macPlain)
		devices = append(devices, entities.Device{
			Vlan:      m[1],
			Mac:       macPlain,
			MacFull:   macFull,
			Interface: iface,
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
