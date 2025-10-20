package dmos

import (
	"fmt"
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
	// Match lines like "VLAN 1 [DefaultVlan]: static, active" or "VLAN 10: static, active"
	vlanLineRegex := regexp.MustCompile(`(?i)^VLAN\s+(\d+)\s*(?:\[.*?\])?:\s*`)

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Try to match VLAN line format from "show vlan table"
		match := vlanLineRegex.FindStringSubmatch(trimmed)
		if len(match) >= 2 {
			vlans[match[1]] = true
			continue
		}

		// Fallback to original simple format (for other commands)
		match = vlanPrefixRegex.FindStringSubmatch(trimmed)
		if len(match) >= 2 {
			vlans[match[1]] = true
		}
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

func parseDmOSSwitchportVLANs(output string) map[string]string {
	vlanMap := make(map[string]string)
	lines := strings.Split(output, "\n")

	var currentInterface string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match "Information of Eth 1/4:"
		if strings.HasPrefix(trimmed, "Information of Eth") {
			// Extract interface name
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				// "Information of Eth 1/4:" -> parts[2]="Eth" parts[3]="1/4:"
				iface := strings.TrimSuffix(parts[3], ":")
				currentInterface = strings.ToLower("ethernet " + iface)
			}
			continue
		}

		// Match "Native VLAN:                   12"
		if strings.Contains(trimmed, "Native VLAN:") && currentInterface != "" {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				vlan := parts[2]
				vlanMap[currentInterface] = vlan
			}
		}
	}

	return vlanMap
}

func parseDmOSTrunksFromSwitchport(output string) map[string]bool {
	trunks := make(map[string]bool)
	lines := strings.Split(output, "\n")

	var currentInterface string
	var inAllowedVLANs bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match "Information of Eth 1/4:"
		if strings.HasPrefix(trimmed, "Information of Eth") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				iface := strings.TrimSuffix(parts[3], ":")
				currentInterface = strings.ToLower("ethernet " + iface)
				inAllowedVLANs = false
			}
			continue
		}

		// Match "Allowed VLANs:" section
		if strings.Contains(trimmed, "Allowed VLANs:") && currentInterface != "" {
			inAllowedVLANs = true
			// Check if this line already contains tagged VLANs
			if strings.Contains(trimmed, "(s,t)") {
				// Tagged VLANs = trunk
				trunks[currentInterface] = true
			}
			continue
		}

		// Handle continuation lines of Allowed VLANs
		if inAllowedVLANs && currentInterface != "" {
			// If we see "Forbidden VLANs:" we're done with Allowed VLANs
			if strings.Contains(trimmed, "Forbidden VLANs:") {
				inAllowedVLANs = false
				continue
			}
			// If line contains VLAN numbers, check for tagged or multiple VLANs
			if strings.Contains(trimmed, "(s,t)") {
				trunks[currentInterface] = true
				inAllowedVLANs = false
			}
		}
	}

	return trunks
}

func parseDmOSInterfaceStatus(output string) []entities.Port {
	ports := make([]entities.Port, 0)
	lines := strings.Split(output, "\n")

	var currentInterface string
	var isUp bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match "Information of  Eth 1/4"
		if strings.HasPrefix(trimmed, "Information of") && strings.Contains(trimmed, "Eth") {
			// Save previous interface if it was up
			if currentInterface != "" && isUp {
				ports = append(ports, entities.Port{Interface: currentInterface, Vlan: ""})
			}

			// Extract interface name (e.g., "Eth 1/4" -> "ethernet 1/4")
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				// "Information of Eth 1/4" -> parts[2]="Eth" parts[3]="1/4"
				currentInterface = strings.ToLower("ethernet " + parts[3])
				isUp = false
			}
			continue
		}

		// Match "Link status:            Up" or "Link status:            Down"
		if strings.Contains(trimmed, "Link status:") {
			if strings.Contains(strings.ToLower(trimmed), "up") {
				isUp = true
			} else {
				isUp = false
			}
		}
	}

	// Don't forget the last interface
	if currentInterface != "" && isUp {
		ports = append(ports, entities.Port{Interface: currentInterface, Vlan: ""})
	}

	// Sort by interface number properly (ethernet 1/4 < ethernet 1/25)
	sort.SliceStable(ports, func(i, j int) bool {
		// Extract numbers from "ethernet X/Y"
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func compareInterfaceNames(a, b string) bool {
	// Extract numbers from "ethernet X/Y" format
	// e.g., "ethernet 1/4" -> [1, 4]
	extractNumbers := func(s string) []int {
		parts := strings.Split(s, " ")
		if len(parts) < 2 {
			return []int{}
		}
		// parts[1] should be "X/Y"
		nums := strings.Split(parts[1], "/")
		if len(nums) != 2 {
			return []int{}
		}
		var result []int
		for _, n := range nums {
			num := 0
			fmt.Sscanf(n, "%d", &num)
			result = append(result, num)
		}
		return result
	}

	aNums := extractNumbers(a)
	bNums := extractNumbers(b)

	if len(aNums) != 2 || len(bNums) != 2 {
		return a < b // fallback to string comparison
	}

	// Compare unit first, then port
	if aNums[0] != bNums[0] {
		return aNums[0] < bNums[0]
	}
	return aNums[1] < bNums[1]
}

func parseDmOSMACTable(output string, trunks map[string]bool) []entities.Device {
	devices := make([]entities.Device, 0)
	lines := strings.Split(output, "\n")

	// DmOS format: "   1       Eth  1/25 5E:15:F4:01:9A:57   20   - Learned"
	// Fields: Unit Block Interface MAC VLAN VPN Type
	macLineRegex := regexp.MustCompile(`(?i)^\s*\d+\s+\w*\s+(Eth\s+\d+/\d+)\s+([0-9A-F:]+)\s+(\d+)\s+.*Learned`)

	for _, line := range lines {
		match := macLineRegex.FindStringSubmatch(line)
		if len(match) < 4 {
			continue
		}

		// match[1] = "Eth  1/25"
		// match[2] = "5E:15:F4:01:9A:57"
		// match[3] = "20"
		ifaceParts := strings.Fields(match[1])
		if len(ifaceParts) < 2 {
			continue
		}
		iface := strings.ToLower("ethernet " + ifaceParts[1])

		// Skip trunk ports
		if trunks[iface] {
			continue
		}

		macRaw := match[2]
		vlan := match[3]

		// Convert MAC from "5E:15:F4:01:9A:57" to plain format
		macPlain := normalizeMac(macRaw)

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
	// Remove dots and colons: "0011.2233.4455" or "00:11:22:33:44:55" -> "001122334455"
	clean := strings.ToLower(mac)
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, ":", "")
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
