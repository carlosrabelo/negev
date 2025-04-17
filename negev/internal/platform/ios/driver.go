package ios

import (
	"regexp"
	"sort"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

var vlanLineRegex = regexp.MustCompile(`^\s*(?:vlan\s+)?(\d{1,4})\b`)
var interfaceRegex = regexp.MustCompile(`^[A-Za-z]+\d+(?:/\d+){0,2}$`)

type Driver struct{}

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
	var ifaces []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if interfaceRegex.MatchString(fields[0]) {
			ifaces = append(ifaces, fields[0])
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
				status = f
				if i+1 < len(fields) {
					vlan = fields[i+1]
				}
				break
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

func isStatusKeyword(s string) bool {
	for _, kw := range statusKeywords {
		if s == kw {
			return true
		}
	}
	return s == "notconnect"
}
