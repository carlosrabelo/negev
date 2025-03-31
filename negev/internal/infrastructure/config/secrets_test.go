package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("SWITCH_USER", "netops")
	t.Setenv("SWITCH_PASS", "s3cret")
	t.Setenv("SWITCH_ENABLE", "enables3cret")

	got, err := expandEnv("user=${SWITCH_USER}")
	if err != nil || got != "user=netops" {
		t.Fatalf("expandEnv mixed = %q, %v", got, err)
	}

	got, err = expandEnv("$${SWITCH_USER}")
	if err != nil || got != "${SWITCH_USER}" {
		t.Fatalf("expandEnv escape = %q, %v; want ${SWITCH_USER}", got, err)
	}

	got, err = expandEnv("${SWITCH_USER}:${SWITCH_PASS}")
	if err != nil || got != "netops:s3cret" {
		t.Fatalf("expandEnv multi = %q, %v", got, err)
	}

	yaml := `
username: "${SWITCH_USER}"
password: "${SWITCH_PASS}"
enable_password: "$${SWITCH_ENABLE}"
default_vlan: "10"
switches:
  - target: "192.0.2.1"
    platform: ios
    password: "${SWITCH_PASS}"
    enable_password: "${SWITCH_ENABLE}"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "192.0.2.1", 0)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Username != "netops" || cfg.Password != "s3cret" {
		t.Fatalf("global credentials = %q/%q", cfg.Username, cfg.Password)
	}
	if cfg.EnablePassword != "${SWITCH_ENABLE}" {
		t.Fatalf("escaped enable_password = %q; want literal ${SWITCH_ENABLE}", cfg.EnablePassword)
	}
	sw := cfg.Switches[0]
	if sw.Password != "s3cret" || sw.EnablePassword != "enables3cret" {
		t.Fatalf("switch credentials = %q/%q", sw.Password, sw.EnablePassword)
	}
}

func TestEnvVarMissingError(t *testing.T) {
	_ = os.Unsetenv("NEGEV_MISSING_SECRET")
	_, err := expandEnv("x=${NEGEV_MISSING_SECRET}")
	if err == nil || !strings.Contains(err.Error(), `environment variable "NEGEV_MISSING_SECRET" not set`) {
		t.Fatalf("expandEnv missing = %v", err)
	}

	yaml := `
username: "${NEGEV_MISSING_SECRET}"
password: "p"
enable_password: "e"
default_vlan: "10"
switches:
  - target: "192.0.2.1"
    platform: ios
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, "", 0); err == nil || !strings.Contains(err.Error(), `environment variable "NEGEV_MISSING_SECRET" not set`) {
		t.Fatalf("Load missing env = %v", err)
	}
}
