package ios

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

var (
	vlanLineRegex   = regexp.MustCompile(`(?i)^\s*(?:vlan\s+)?(\d{1,4})\b`)
	macTableRegex   = regexp.MustCompile(`(?m)^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)\s*$`)
	interfaceRegex  = regexp.MustCompile(`^[A-Za-z]+\d+(?:/\d+){0,2}$`)
	macPlain        = regexp.MustCompile(`^[0-9a-f]{12}$`)
	commandErrHints = []string{
		"invalid input",
		"unknown command",
		"incomplete command",
		"ambiguous command",
		"unrecognized command",
		"invalid command",
		"syntax error",
		"cannot find command",
	}
)

func parseIOSVLANList(output string) map[string]bool {
	vlans := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		match := vlanLineRegex.FindStringSubmatch(trimmed)
		if len(match) < 2 {
			continue
		}
		vlans[match[1]] = true
	}
	return vlans
}

func parseIOSTrunks(output string) map[string]bool {
	trunks := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		candidate := fields[0]
		if interfaceRegex.MatchString(candidate) {
			trunks[strings.ToLower(candidate)] = true
		}
	}
	return trunks
}

func parseIOSInterfaceStatus(output string) []entities.Port {
	ports := make([]entities.Port, 0)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}
		iface := fields[0]
		if !interfaceRegex.MatchString(iface) {
			continue
		}
		statusIdx := -1
		for i := 1; i < len(fields); i++ {
			lower := strings.ToLower(fields[i])
			if lower == "connected" || lower == "up" || lower == "forward" || lower == "monitor" || lower == "active" || lower == "link-up" {
				statusIdx = i
				break
			}
		}
		if statusIdx == -1 {
			continue
		}
		statusToken := strings.ToLower(fields[statusIdx])
		if statusToken == "notconnect" {
			continue
		}
		if !(statusToken == "connected" || statusToken == "up" || strings.Contains(statusToken, "forward") || strings.Contains(statusToken, "active") || strings.Contains(statusToken, "link-up") || strings.Contains(statusToken, "monitor")) {
			continue
		}
		vlanIdx := statusIdx + 1
		if vlanIdx >= len(fields) {
			continue
		}
		vlanField := fields[vlanIdx]
		if strings.EqualFold(vlanField, "trunk") || vlanLineRegex.MatchString(vlanField) {
			ports = append(ports, entities.Port{Interface: iface, Vlan: strings.ToLower(vlanField)})
		}
	}
	sort.SliceStable(ports, func(i, j int) bool {
		return strings.ToLower(ports[i].Interface) < strings.ToLower(ports[j].Interface)
	})
	return ports
}

func parseIOSMACTable(output string, trunks map[string]bool) []entities.Device {
	devices := make([]entities.Device, 0)
	matches := macTableRegex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		if _, err := strconv.Atoi(vlan); err != nil {
			continue
		}
		if !interfaceRegex.MatchString(iface) {
			continue
		}
		macPlainValue := strings.ToLower(strings.ReplaceAll(mac, ".", ""))
		if !macPlain.MatchString(macPlainValue) {
			continue
		}
		if trunks[strings.ToLower(iface)] {
			continue
		}
		devices = append(devices, entities.Device{
			Vlan:      vlan,
			Mac:       macPlainValue,
			MacFull:   formatPlainMac(macPlainValue),
			Interface: iface,
		})
	}
	return devices
}

func formatPlainMac(mac string) string {
	var builder strings.Builder
	for i := 0; i < len(mac); i += 2 {
		if i > 0 {
			builder.WriteByte(':')
		}
		builder.WriteString(mac[i : i+2])
	}
	return builder.String()
}

func isIOSCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, keyword := range commandErrHints {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isSeparatorLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if len(trimmed) < 3 {
		return false
	}
	for _, ch := range trimmed {
		if ch != '-' && ch != '=' && ch != '+' && ch != '*' {
			return false
		}
	}
	return true
}
