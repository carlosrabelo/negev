package dmos

import (
	"fmt"
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
	out, err := repo.ExecuteCommand("show interfaces switchport")
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
	swOut, err := repo.ExecuteCommand("show interfaces switchport")
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
	return nil, fmt.Errorf("not implemented")
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
