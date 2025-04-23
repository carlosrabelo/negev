package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigLoadMergeDefaultAndVlans(t *testing.T) {
	yamlData := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
allowed_vlans:
  - "10"
  - "20"
protected_vlans:
  - "999"

switches:
  - target: 192.168.1.10
    platform: ios
    transport: ssh
    username: switchadmin
    password: switchpass
    enable_password: enablepass

  - target: 192.168.1.20
    platform: auto
    default_vlan: "100"
    no_data_vlan: "200"
    allowed_vlans:
      - "20"
      - "30"
    protected_vlans:
      - "100"
    exclude_ports:
      - "Gi1/0/24 "
      - "gi1/0/24"
      - "Gi1/0/23"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_config.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write temp yaml config: %v", err)
	}

	cfg, err := Load(tmpFile, "192.168.1.10", false, 0, false)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.Switches) != 2 {
		t.Fatalf("expected 2 switches, got %d", len(cfg.Switches))
	}

	// Validar Switch 1 (herança total)
	sw1 := cfg.Switches[0]
	if sw1.DefaultVlan != "1" {
		t.Errorf("sw1.DefaultVlan = %q; expected \"1\"", sw1.DefaultVlan)
	}
	if sw1.NoDataVlan != "999" {
		t.Errorf("sw1.NoDataVlan = %q; expected \"999\"", sw1.NoDataVlan)
	}
	if !reflect.DeepEqual(sw1.AllowedVlans, []string{"10", "20"}) {
		t.Errorf("sw1.AllowedVlans = %v; expected [\"10\", \"20\"]", sw1.AllowedVlans)
	}
	if !reflect.DeepEqual(sw1.ProtectedVlans, []string{"999"}) {
		t.Errorf("sw1.ProtectedVlans = %v; expected [\"999\"]", sw1.ProtectedVlans)
	}

	// Validar Switch 2 (sobrescrita e mesclagem)
	sw2 := cfg.Switches[1]
	if sw2.DefaultVlan != "100" {
		t.Errorf("sw2.DefaultVlan = %q; expected \"100\"", sw2.DefaultVlan)
	}
	if sw2.NoDataVlan != "200" {
		t.Errorf("sw2.NoDataVlan = %q; expected \"200\"", sw2.NoDataVlan)
	}
	// "10", "20" (global) + "20", "30" (local) -> "10", "20", "30"
	expectedAllowed := []string{"10", "20", "30"}
	if !reflect.DeepEqual(sw2.AllowedVlans, expectedAllowed) {
		t.Errorf("sw2.AllowedVlans = %v; expected %v", sw2.AllowedVlans, expectedAllowed)
	}
	// "999" (global) + "100" (local) -> "999", "100"
	expectedProtected := []string{"999", "100"}
	if !reflect.DeepEqual(sw2.ProtectedVlans, expectedProtected) {
		t.Errorf("sw2.ProtectedVlans = %v; expected %v", sw2.ProtectedVlans, expectedProtected)
	}

	expectedExcludePorts := []string{"gi1/0/24", "gi1/0/23"}
	if !reflect.DeepEqual(sw2.ExcludePorts, expectedExcludePorts) {
		t.Errorf("sw2.ExcludePorts = %v; expected %v", sw2.ExcludePorts, expectedExcludePorts)
	}
}

func TestConfigLoadInvalidVlans(t *testing.T) {
	yamlDataInvalidDefault := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
switches:
  - target: 192.168.1.10
    default_vlan: "5000"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_config_invalid.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlDataInvalidDefault), 0644); err != nil {
		t.Fatalf("failed to write temp yaml config: %v", err)
	}

	_, err := Load(tmpFile, "", false, 0, false)
	if err == nil {
		t.Error("expected error due to invalid default_vlan (5000), got nil")
	}
}

func TestConfigLoadMergeMacToVlan(t *testing.T) {
	yamlData := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
mac_to_vlan:
  "AA:BB:CC": "10"
  "112233445566": "20"
  "deadbe": "100"

switches:
  - target: 192.168.1.10
    platform: ios
    mac_to_vlan:
      "001122": "30"
      "11:22:33": "0"  # Remove o prefixo 112233
      "deadbe": "200"  # Sobrescreve o global
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_config_mactovlan.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write temp yaml config: %v", err)
	}

	cfg, err := Load(tmpFile, "192.168.1.10", false, 0, false)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	sw1 := cfg.Switches[0]
	expectedMacToVlan := map[string]string{
		"aabbcc": "10",
		"deadbe": "200",
		"001122": "30",
	}

	if !reflect.DeepEqual(sw1.MacToVlan, expectedMacToVlan) {
		t.Errorf("sw1.MacToVlan = %v; expected %v", sw1.MacToVlan, expectedMacToVlan)
	}
}
