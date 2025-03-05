package parseutil

import "testing"

func TestFormatPlainMacTwelveHex(t *testing.T) {
	got := FormatPlainMac("aabbccddeeff")
	if got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("FormatPlainMac = %q", got)
	}
}
