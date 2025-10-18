package dmos

import (
	"regexp"
	"sort"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

var (
	vlanPrefixRegex = regexp.MustCompile(`(?i)^\s*(\d{1,4})\b`)
	dmosPortRegex   = regexp.MustCompile(`(?i)ethernet\s+\d+/\d+(?:/\d+)?`)
	columnSplitRe   = regexp.MustCompile(`\s{2,}`)
	macFieldRegex   = regexp.MustCompile(`(?i)^\s*(\d{1,4})\s+([0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\s+dynamic\s+(ethernet\s+\d+/\d+(?:/\d+)?)`)
	cmdErrorHints   = []string{"unknown command", "invalid", "incomplete", "syntax error"}
)

func parseDmOSVLANList(output string) map[string]bool {
	vlans := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		match := vlanPrefixRegex.FindStringSubmatch(trimmed)
		if len(match) < 2 {
			continue
		}
		vlans[match[1]] = true
	}
	return vlans
}

func parseDmOSTrunks(output string) map[string]bool {
	trunks := make(map[string]bool)
	rows := strings.Split(output, "\n")
	for _, row := range rows {
		trimmed := strings.TrimSpace(row)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		if trimmed[0] < '0' || trimmed[0] > '9' {
			continue
		}
		parts := columnSplitRe.Split(trimmed, -1)
		if len(parts) < 2 {
			continue
		}
		taggedField := parts[1]
		matches := dmosPortRegex.FindAllString(strings.ToLower(taggedField), -1)
		for _, port := range matches {
			trunks[strings.TrimSpace(port)] = true
		}
	}
	return trunks
}

func parseDmOSInterfaceStatus(output string) []entities.Port {
	ports := make([]entities.Port, 0)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		if !strings.Contains(strings.ToLower(trimmed), "ethernet") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 4 {
			continue
		}
		iface := strings.ToLower(fields[0] + " " + fields[1])
		status := strings.ToLower(strings.Join(fields[2:], " "))
		if !(strings.Contains(status, "up") || strings.Contains(status, "forward") || strings.Contains(status, "online")) {
			continue
		}
		vlan := fields[len(fields)-1]
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].Interface < ports[j].Interface
	})
	return ports
}

func parseDmOSMACTable(output string, trunks map[string]bool) []entities.Device {
	devices := make([]entities.Device, 0)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		match := macFieldRegex.FindStringSubmatch(strings.ToLower(line))
		if len(match) < 4 {
			continue
		}
		vlan := match[1]
		macPlain := normalizeMac(match[2])
		iface := strings.TrimSpace(match[3])
		if trunks[strings.TrimSpace(iface)] {
			continue
		}
		devices = append(devices, entities.Device{
			Vlan:      vlan,
			Mac:       macPlain,
			MacFull:   formatPlainMac(macPlain),
			Interface: iface,
		})
	}
	return devices
}

func normalizeMac(mac string) string {
	clean := strings.ToLower(strings.ReplaceAll(mac, ".", ""))
	return clean
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

func isDmOSCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, keyword := range cmdErrorHints {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func normalizePort(iface string) string {
	trimmed := strings.TrimSpace(strings.ToLower(iface))
	if strings.HasPrefix(trimmed, "ethernet") {
		return trimmed
	}
	return "ethernet " + trimmed
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
