package entities

import "testing"

func TestIsDebugEnabled(t *testing.T) {
	if (SwitchConfig{VerbosityLevel: 0}).IsDebugEnabled() {
		t.Fatal("verbosity 0 should not be debug")
	}
	if !(SwitchConfig{VerbosityLevel: 1}).IsDebugEnabled() {
		t.Fatal("verbosity 1 should be debug")
	}
	if !(SwitchConfig{VerbosityLevel: 3}).IsDebugEnabled() {
		t.Fatal("verbosity 3 should be debug")
	}
}
