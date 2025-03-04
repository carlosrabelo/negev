package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInheritsGlobalWhenSwitchOmitsFields(t *testing.T) {
	yamlData := `
platform: ios
transport: telnet
username: admin
password: secret
enable_password: enable
default_vlan: "1"
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
