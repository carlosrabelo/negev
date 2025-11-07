package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "colon separated",
			input:    "AA:BB:CC:DD:EE:FF",
			expected: "aabbccddeeff",
		},
		{
			name:     "dot separated",
			input:    "aabb.ccdd.eeff",
			expected: "aabbccddeeff",
		},
		{
			name:     "mixed case",
			input:    "Aa:Bb:Cc:Dd:Ee:Ff",
			expected: "aabbccddeeff",
		},
		{
			name:     "already normalized",
			input:    "aabbccddeeff",
			expected: "aabbccddeeff",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMAC(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeMAC(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidatePlatform(t *testing.T) {
	tests := []struct {
		name      string
		platform  string
		expectErr bool
	}{
		{
			name:      "valid ios",
			platform:  "ios",
			expectErr: false,
		},
		{
			name:      "valid dmos",
			platform:  "dmos",
			expectErr: false,
		},
		{
			name:      "valid auto",
			platform:  "auto",
			expectErr: false,
		},
		{
			name:      "invalid platform",
			platform:  "invalid",
			expectErr: true,
		},
		{
			name:      "empty platform",
			platform:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlatform(tt.platform)
			if (err != nil) != tt.expectErr {
				t.Errorf("validatePlatform(%q) error = %v, expectErr %v", tt.platform, err, tt.expectErr)
			}
		})
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	yamlContent := `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
allowed_vlans:
  - 10
  - 20
  - 30
protected_vlans:
  - 1
  - 100
exclude_macs:
  - "aa:bb:cc:dd:ee:ff"
  - "11:22:33:44:55:66"
mac_to_vlan:
  "aabbcc": "20"
  "112233": "30"
switches:
  - target: "192.168.1.1"
    platform: ios
    username: switch_admin
    password: switch_pass
    enable_password: switch_enable
    default_vlan: 10
    no_data_vlan: 99
    exclude_ports:
      - "Gi1/0/1"
      - "Gi1/0/2"
    mac_to_vlan:
      "aabbcc": "25"
    allowed_vlans:
      - 10
      - 25
`

	// Create temporary file
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config, err := Load(tmpFile, "", false, 0, false)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if config.Platform != "ios" {
		t.Errorf("Expected platform 'ios', got '%s'", config.Platform)
	}

	if config.Transport != "telnet" {
		t.Errorf("Expected transport 'telnet', got '%s'", config.Transport)
	}

	if len(config.Switches) != 1 {
		t.Fatalf("Expected 1 switch, got %d", len(config.Switches))
	}

	sw := config.Switches[0]
	if sw.Target != "192.168.1.1" {
		t.Errorf("Expected target '192.168.1.1', got '%s'", sw.Target)
	}

	if sw.Platform != "ios" {
		t.Errorf("Expected switch platform 'ios', got '%s'", sw.Platform)
	}

	// Check MAC normalization
	expectedMACs := []string{"aabbccddeeff", "112233445566"}
	if len(sw.ExcludeMacs) != len(expectedMACs) {
		t.Errorf("Expected %d exclude MACs, got %d", len(expectedMACs), len(sw.ExcludeMacs))
	}

	// Check MAC to VLAN mapping
	if sw.MacToVlan["aabbcc"] != "25" {
		t.Errorf("Expected MAC aabbcc to map to VLAN 25, got '%s'", sw.MacToVlan["aabbcc"])
	}

	// Check allowed VLANs merging
	expectedAllowedVlans := []string{"10", "20", "25", "30"}
	if len(sw.AllowedVlans) != len(expectedAllowedVlans) {
		t.Errorf("Expected %d allowed VLANs, got %d", len(expectedAllowedVlans), len(sw.AllowedVlans))
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectErr   bool
		errContains string
	}{
		{
			name: "missing default_vlan",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
no_data_vlan: 99
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "global default_vlan is required",
		},
		{
			name: "missing no_data_vlan",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "global no_data_vlan is required",
		},
		{
			name: "missing username",
			yamlContent: `
platform: ios
transport: telnet
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "global username is required",
		},
		{
			name: "no switches defined",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
`,
			expectErr:   true,
			errContains: "no switches defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "config.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			_, err = Load(tmpFile, "", false, 0, false)
			if (err != nil) != tt.expectErr {
				t.Errorf("Load() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errContains, err.Error())
				}
			}
		})
	}
}

func TestLoad_InvalidVLANs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectErr   bool
		errContains string
	}{
		{
			name: "invalid default_vlan range",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 5000
no_data_vlan: 99
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "must be between 1 and 4094",
		},
		{
			name: "invalid no_data_vlan range",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 0
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "must be between 1 and 4094",
		},
		{
			name: "invalid allowed_vlan",
			yamlContent: `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
allowed_vlans:
  - "invalid"
switches:
  - target: "192.168.1.1"
`,
			expectErr:   true,
			errContains: "must be a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "config.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			_, err = Load(tmpFile, "", false, 0, false)
			if (err != nil) != tt.expectErr {
				t.Errorf("Load() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errContains, err.Error())
				}
			}
		})
	}
}

func TestLoad_InvalidTransport(t *testing.T) {
	yamlContent := `
platform: ios
transport: invalid
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
switches:
  - target: "192.168.1.1"
`

	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err = Load(tmpFile, "", false, 0, false)
	if err == nil {
		t.Fatal("Expected error for invalid transport")
	}

	expectedErr := "transport invalid is invalid, must be 'telnet' or 'ssh'"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestLoad_SwitchSpecificOverrides(t *testing.T) {
	yamlContent := `
platform: ios
transport: telnet
username: global_user
password: global_pass
enable_password: global_enable
default_vlan: 10
no_data_vlan: 99
allowed_vlans:
  - 10
  - 20
protected_vlans:
  - 1
exclude_macs:
  - "aa:bb:cc:dd:ee:ff"
mac_to_vlan:
  "aabbcc": "20"
switches:
  - target: "192.168.1.1"
    platform: dmos
    transport: ssh
    username: switch_user
    password: switch_pass
    enable_password: switch_enable
    default_vlan: 15
    no_data_vlan: 98
    allowed_vlans:
      - 15
      - 25
    protected_vlans:
      - 2
    exclude_macs:
      - "11:22:33:44:55:66"
    mac_to_vlan:
      "112233": "25"
`

	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config, err := Load(tmpFile, "", false, 0, false)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	sw := config.Switches[0]

	// Check switch-specific overrides
	if sw.Platform != "dmos" {
		t.Errorf("Expected switch platform 'dmos', got '%s'", sw.Platform)
	}

	if sw.Transport != "ssh" {
		t.Errorf("Expected switch transport 'ssh', got '%s'", sw.Transport)
	}

	if sw.Username != "switch_user" {
		t.Errorf("Expected switch username 'switch_user', got '%s'", sw.Username)
	}

	if sw.DefaultVlan != "15" {
		t.Errorf("Expected switch default_vlan '15', got '%s'", sw.DefaultVlan)
	}

	// Check MAC merging (should include both global and switch-specific)
	expectedMACs := []string{"aabbccddeeff", "112233445566"}
	if len(sw.ExcludeMacs) != len(expectedMACs) {
		t.Errorf("Expected %d exclude MACs, got %d", len(expectedMACs), len(sw.ExcludeMacs))
	}

	// Check MAC to VLAN mapping (switch-specific should override global)
	if sw.MacToVlan["aabbcc"] != "20" {
		t.Errorf("Expected MAC aabbcc to map to VLAN 20 (global), got '%s'", sw.MacToVlan["aabbcc"])
	}

	if sw.MacToVlan["112233"] != "25" {
		t.Errorf("Expected MAC 112233 to map to VLAN 25 (switch-specific), got '%s'", sw.MacToVlan["112233"])
	}

	// Check allowed VLANs merging
	expectedAllowedVlans := []string{"10", "15", "20", "25"}
	if len(sw.AllowedVlans) != len(expectedAllowedVlans) {
		t.Errorf("Expected %d allowed VLANs, got %d", len(expectedAllowedVlans), len(sw.AllowedVlans))
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	_, err := Load("nonexistent.yaml", "", false, 0, false)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	if !contains(err.Error(), "failed to read YAML file") {
		t.Errorf("Expected file read error, got: %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	yamlContent := `
platform: ios
transport: telnet
username: admin
password: password
enable_password: enable
default_vlan: 10
no_data_vlan: 99
invalid yaml content: [
  - missing proper indentation
    - broken
switches:
  - target: "192.168.1.1"
`

	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err = Load(tmpFile, "", false, 0, false)
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}

	if !contains(err.Error(), "failed to parse YAML") {
		t.Errorf("Expected YAML parse error, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}