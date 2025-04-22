package dmos

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

var vlanTableRegex = regexp.MustCompile(`^VLAN\s+(\d+)\s*(?:\[.*?\])?:\s*`)

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

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	return nil, fmt.Errorf("not implemented")
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
