package config

import (
	"os"
	"reflect"
	"path/filepath"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestLoadInheritsGlobalWhenSwitchOmitsFields(t *testing.T) {
	yamlData := `
platform: ios
transport: telnet
username: admin
password: secret
enable_password: enable
default_vlan: "1"
no_data_vlan: "999"
switches:
  - target: 192.168.1.10
`
	tmpFile := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmpFile, "192.168.1.10", 1)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Switches) != 1 {
		t.Fatalf("switches = %d", len(cfg.Switches))
	}
	sw := cfg.Switches[0]
	if sw.Username != "admin" || sw.Transport != "telnet" || sw.DefaultVlan != "1" || sw.Platform != "ios" {
		t.Fatalf("unexpected merge: %+v", sw)
	}
}

func TestLoadRequiresSwitches(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(tmpFile, []byte("platform: ios\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(tmpFile, "", 0); err == nil {
		t.Fatal("expected error when no switches are defined")
	}
}

func TestLoadLegacyVendorFallback(t *testing.T) {
	yamlData := `
vendor: ios
transport: telnet
username: admin
password: secret
enable_password: enable
default_vlan: "1"
no_data_vlan: "999"
switches:
  - target: 192.168.1.10
    vendor: ios
`
	tmpFile := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmpFile, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Platform != "ios" || cfg.Switches[0].Platform != "ios" {
		t.Fatalf("legacy vendor fallback failed: global=%q switch=%q", cfg.Platform, cfg.Switches[0].Platform)
	}
}

func TestNormalizeMAC(t *testing.T) {
	got := NormalizeMAC("AA:BB:CC:DD:EE:FF")
	if got != "aabbccddeeff" {
		t.Fatalf("NormalizeMAC colon = %q", got)
	}
	got = NormalizeMAC("aabb.ccdd.eeff")
	if got != "aabbccddeeff" {
		t.Fatalf("NormalizeMAC dotted = %q", got)
	}
}

func TestMACPrefix(t *testing.T) {
	if got := macPrefix("AA:BB:CC:DD:EE:FF"); got != "aabbcc" {
		t.Fatalf("macPrefix = %q", got)
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
	tmpFile := filepath.Join(t.TempDir(), "test_config_invalid.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlDataInvalidDefault), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(tmpFile, "", 0); err == nil {
		t.Error("expected error due to invalid default_vlan (5000), got nil")
	}
}

func TestValidatePlatformRejectsUnknown(t *testing.T) {
	yamlData := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
switches:
  - target: 192.168.1.10
    platform: junos
`
	tmpFile := filepath.Join(t.TempDir(), "junos.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(tmpFile, "", 0); err == nil {
		t.Fatal("expected unknown platform to be rejected")
	}
}

func TestConfigLoadMergeMacToVlanNaive(t *testing.T) {
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
  "deadbe": "100"
switches:
  - target: 192.168.1.10
    platform: ios
    mac_to_vlan:
      "deadbe": "200"
`
	tmpFile := filepath.Join(t.TempDir(), "mactovlan.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmpFile, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Switches[0].MacToVlan
	if got["aabbcc"] != "10" || got["deadbe"] != "200" {
		t.Fatalf("MacToVlan = %v", got)
	}
}

func TestApplySandboxInvertsWrite(t *testing.T) {
	cfg := &Config{Switches: []entities.SwitchConfig{{Target: "192.168.1.10"}}}
	ApplySandbox(cfg, false)
	if !cfg.Switches[0].Sandbox {
		t.Fatal("expected sandbox when write is false")
	}
	ApplySandbox(cfg, true)
	if cfg.Switches[0].Sandbox {
		t.Fatal("expected sandbox disabled when write is true")
	}
}

func TestConfigLoadRejectsInvalidGlobalAllowedVlan(t *testing.T) {
	yamlData := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
allowed_vlans:
  - "5000"
switches:
  - target: 192.168.1.10
    platform: ios
`
	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(tmpFile, "", 0); err == nil {
		t.Fatal("expected invalid global allowed_vlans to fail")
	}
}

func TestConfigLoadCanonicalizesVLANIDs(t *testing.T) {
	yamlData := `
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "010"
no_data_vlan: "0999"
allowed_vlans:
  - "010"
switches:
  - target: 192.168.1.10
    platform: ios
    default_vlan: "0100"
`
	tmpFile := filepath.Join(t.TempDir(), "canon.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmpFile, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultVlan != "10" || cfg.NoDataVlan != "999" {
		t.Fatalf("global vlans = %q / %q", cfg.DefaultVlan, cfg.NoDataVlan)
	}
	if cfg.Switches[0].DefaultVlan != "100" {
		t.Fatalf("switch default = %q", cfg.Switches[0].DefaultVlan)
	}
	if len(cfg.Switches[0].AllowedVlans) != 1 || cfg.Switches[0].AllowedVlans[0] != "10" {
		t.Fatalf("allowed = %v", cfg.Switches[0].AllowedVlans)
	}
	_ = reflect.DeepEqual
}
