package entities

import "testing"

func TestSwitchConfigHelpers(t *testing.T) {
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

	if (SwitchConfig{}).PlatformID() != "ios" {
		t.Fatal("empty platform should default to ios")
	}
	if (SwitchConfig{LegacyPlatform: "dmos"}).PlatformID() != "dmos" {
		t.Fatal("legacy vendor should be used when platform empty")
	}
	if (SwitchConfig{Platform: "auto", LegacyPlatform: "ios"}).PlatformID() != "auto" {
		t.Fatal("platform should take precedence over legacy")
	}
}
