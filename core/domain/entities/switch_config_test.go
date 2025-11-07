package entities

import (
	"testing"
)

func TestSwitchConfig_IsDebugEnabled(t *testing.T) {
	tests := []struct {
		name          string
		verbosityLevel int
		expected       bool
	}{
		{
			name:          "verbosity level 0",
			verbosityLevel: 0,
			expected:      false,
		},
		{
			name:          "verbosity level 1",
			verbosityLevel: 1,
			expected:      true,
		},
		{
			name:          "verbosity level 2",
			verbosityLevel: 2,
			expected:      false,
		},
		{
			name:          "verbosity level 3",
			verbosityLevel: 3,
			expected:      true,
		},
		{
			name:          "verbosity level 4",
			verbosityLevel: 4,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SwitchConfig{
				VerbosityLevel: tt.verbosityLevel,
			}

			result := config.IsDebugEnabled()
			if result != tt.expected {
				t.Errorf("IsDebugEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSwitchConfig_IsRawOutputEnabled(t *testing.T) {
	tests := []struct {
		name          string
		verbosityLevel int
		expected       bool
	}{
		{
			name:          "verbosity level 0",
			verbosityLevel: 0,
			expected:      false,
		},
		{
			name:          "verbosity level 1",
			verbosityLevel: 1,
			expected:      false,
		},
		{
			name:          "verbosity level 2",
			verbosityLevel: 2,
			expected:      true,
		},
		{
			name:          "verbosity level 3",
			verbosityLevel: 3,
			expected:      true,
		},
		{
			name:          "verbosity level 4",
			verbosityLevel: 4,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SwitchConfig{
				VerbosityLevel: tt.verbosityLevel,
			}

			result := config.IsRawOutputEnabled()
			if result != tt.expected {
				t.Errorf("IsRawOutputEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSwitchConfig_PlatformID(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		legacyPlatform string
		expected      string
	}{
		{
			name:          "ios platform",
			platform:      "ios",
			legacyPlatform: "",
			expected:      "ios",
		},
		{
			name:          "dmos platform",
			platform:      "dmos",
			legacyPlatform: "",
			expected:      "dmos",
		},
		{
			name:          "uppercase platform",
			platform:      "IOS",
			legacyPlatform: "",
			expected:      "ios",
		},
		{
			name:          "platform with spaces",
			platform:      "  ios  ",
			legacyPlatform: "",
			expected:      "ios",
		},
		{
			name:          "empty platform, legacy vendor",
			platform:      "",
			legacyPlatform: "dmos",
			expected:      "dmos",
		},
		{
			name:          "empty platform, uppercase legacy vendor",
			platform:      "",
			legacyPlatform: "DMOS",
			expected:      "dmos",
		},
		{
			name:          "empty platform, legacy vendor with spaces",
			platform:      "",
			legacyPlatform: "  ios  ",
			expected:      "ios",
		},
		{
			name:          "both empty",
			platform:      "",
			legacyPlatform: "",
			expected:      "ios",
		},
		{
			name:          "platform takes precedence",
			platform:      "ios",
			legacyPlatform: "dmos",
			expected:      "ios",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SwitchConfig{
				Platform:       tt.platform,
				LegacyPlatform: tt.legacyPlatform,
			}

			result := config.PlatformID()
			if result != tt.expected {
				t.Errorf("PlatformID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSwitchConfig_DefaultValues(t *testing.T) {
	var config SwitchConfig

	// Test default values
	if config.Sandbox != false {
		t.Errorf("Expected Sandbox to be false by default, got %v", config.Sandbox)
	}

	if config.VerbosityLevel != 0 {
		t.Errorf("Expected VerbosityLevel to be 0 by default, got %d", config.VerbosityLevel)
	}

	if config.CreateVLANs != false {
		t.Errorf("Expected CreateVLANs to be false by default, got %v", config.CreateVLANs)
	}
}

func TestSwitchConfig_MapFields(t *testing.T) {
	config := SwitchConfig{
		MacToVlan: map[string]string{
			"aabbcc": "10",
			"ddeeff": "20",
		},
		AllowedVlans:   []string{"10", "20", "30"},
		ProtectedVlans: []string{"1", "100"},
		ExcludeMacs:    []string{"aa:bb:cc:dd:ee:ff"},
		ExcludePorts:   []string{"Gi1/0/1", "Gi1/0/2"},
	}

	// Test map access
	if config.MacToVlan["aabbcc"] != "10" {
		t.Errorf("Expected MAC aabbcc to map to VLAN 10, got %s", config.MacToVlan["aabbcc"])
	}

	if len(config.AllowedVlans) != 3 {
		t.Errorf("Expected 3 allowed VLANs, got %d", len(config.AllowedVlans))
	}

	if len(config.ProtectedVlans) != 2 {
		t.Errorf("Expected 2 protected VLANs, got %d", len(config.ProtectedVlans))
	}

	if len(config.ExcludeMacs) != 1 {
		t.Errorf("Expected 1 exclude MAC, got %d", len(config.ExcludeMacs))
	}

	if len(config.ExcludePorts) != 2 {
		t.Errorf("Expected 2 exclude ports, got %d", len(config.ExcludePorts))
	}
}