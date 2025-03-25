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

func TestRunResultHasChanges(t *testing.T) {
	if (RunResult{}).HasChanges() {
		t.Fatal("empty RunResult should report no changes")
	}
	if !(RunResult{PortsChanged: 1}).HasChanges() {
		t.Fatal("PortsChanged should count as changes")
	}
	if !(RunResult{VLANsCreated: 1}).HasChanges() {
		t.Fatal("VLANsCreated should count as changes")
	}
	if !(RunResult{VLANsDeleted: 1}).HasChanges() {
		t.Fatal("VLANsDeleted should count as changes")
	}
	if (RunResult{PortsFailed: 2}).HasChanges() {
		t.Fatal("PortsFailed alone should not count as changes")
	}
}
