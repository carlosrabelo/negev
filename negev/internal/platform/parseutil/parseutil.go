package parseutil

import "strings"

func FormatPlainMac(mac string) string {
	var b strings.Builder
	for i := 0; i < len(mac); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(mac[i : i+2])
	}
	return b.String()
}
