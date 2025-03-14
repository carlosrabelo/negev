package parseutil

import "testing"

func TestFormatPlainMac(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"aabbccddeeff", "aa:bb:cc:dd:ee:ff"},
		{"001122334455", "00:11:22:33:44:55"},
		{"short", "short"},
		{"", ""},
		{"aabbccddeeffg", "aabbccddeeffg"},
	}

	for _, tc := range tests {
		got := FormatPlainMac(tc.input)
		if got != tc.expected {
			t.Errorf("FormatPlainMac(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
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
