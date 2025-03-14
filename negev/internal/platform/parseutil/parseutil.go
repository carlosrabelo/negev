package parseutil

import "strings"

func FormatPlainMac(mac string) string {
	if len(mac) != 12 {
		return mac
	}
	var b strings.Builder
	for i := 0; i < len(mac); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(mac[i : i+2])
	}
	return b.String()
}

func IsSeparatorLine(line string) bool {
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
