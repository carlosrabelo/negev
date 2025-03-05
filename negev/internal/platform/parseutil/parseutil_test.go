package parseutil

import "testing"

func TestFormatPlainMacTwelveHex(t *testing.T) {
	got := FormatPlainMac("aabbccddeeff")
	if got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("FormatPlainMac = %q", got)
	}
}

func TestIsSeparatorLine(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", true},
		{"   ", true},
		{"---", true},
		{"====", true},
		{"++", false},
		{"**", false},
		{"***", true},
		{"abc", false},
		{"--a--", false},
	}
	for _, tc := range tests {
		got := IsSeparatorLine(tc.input)
		if got != tc.expected {
			t.Errorf("IsSeparatorLine(%q) = %t; expected %t", tc.input, got, tc.expected)
		}
	}
}
