package entities

import "testing"

func TestSwitchConfigDebugAndRaw(t *testing.T) {
	cases := []struct {
		verbosity int
		debug     bool
		raw       bool
	}{
		{0, false, false},
		{1, true, false},
		{2, false, true},
		{3, true, true},
	}
	for _, tc := range cases {
		sc := SwitchConfig{VerbosityLevel: tc.verbosity}
		if sc.IsDebugEnabled() != tc.debug || sc.IsRawOutputEnabled() != tc.raw {
			t.Fatalf("verbosity %d: debug=%v raw=%v", tc.verbosity, sc.IsDebugEnabled(), sc.IsRawOutputEnabled())
		}
	}
}
