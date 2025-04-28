package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadJumpHostGlobalMappingAndSecrets(t *testing.T) {
	t.Setenv("JUMP_HOST", "192.0.2.5")
	t.Setenv("JUMP_USER", "bastion-ops")
	t.Setenv("JUMP_PASS", "jump-secret")
	t.Setenv("JUMP_KEY_PASS", "key-secret")
	t.Setenv("JUMP_KH", "/etc/negev/known_hosts")

	path := writeTempConfig(t, `
platform: ios
username: admin
password: sw-password
default_vlan: "10"
jump_host:
  host: "${JUMP_HOST}"
  port: 2222
  username: "${JUMP_USER}"
  password: "${JUMP_PASS}"
  key_file: ~/.ssh/id_ed25519
  key_passphrase: "${JUMP_KEY_PASS}"
  known_hosts: "${JUMP_KH}"
switches:
  - target: 192.0.2.10
`)

	cfg, err := Load(path, "", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.JumpHost == nil {
		t.Fatal("expected global JumpHost to be set")
	}
	if cfg.JumpHost.Host != "192.0.2.5" || cfg.JumpHost.Port != 2222 || cfg.JumpHost.Username != "bastion-ops" {
		t.Fatalf("unexpected global JumpHost: %+v", cfg.JumpHost)
	}
	if cfg.JumpHost.Password != "jump-secret" || cfg.JumpHost.KeyPassphrase != "key-secret" {
		t.Fatalf("unexpected JumpHost secrets: %+v", cfg.JumpHost)
	}
	if cfg.JumpHost.KeyFile != "~/.ssh/id_ed25519" || cfg.JumpHost.KnownHosts != "/etc/negev/known_hosts" {
		t.Fatalf("unexpected JumpHost paths: %+v", cfg.JumpHost)
	}

	swJump := cfg.Switches[0].JumpHost
	if swJump == nil || *swJump != *cfg.JumpHost {
		t.Fatalf("switch JumpHost = %+v; want %+v", swJump, cfg.JumpHost)
	}
	if swJump == cfg.JumpHost {
		t.Fatal("switch JumpHost should be cloned from global")
	}
}

func TestLoadJumpHostScalarAndEnvExpansion(t *testing.T) {
	t.Setenv("JUMP_SPEC", "ops@198.51.100.5:2202")
	path := writeTempConfig(t, `
platform: ios
username: admin
password: sw-password
default_vlan: "10"
jump_host: "${JUMP_SPEC}"
switches:
  - target: 192.0.2.10
  - target: 192.0.2.11
    jump_host: direct@198.51.100.9
`)

	cfg, err := Load(path, "", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.JumpHost == nil || cfg.JumpHost.Username != "ops" || cfg.JumpHost.Host != "198.51.100.5" || cfg.JumpHost.Port != 2202 {
		t.Fatalf("unexpected global scalar JumpHost: %+v", cfg.JumpHost)
	}
	if sw0 := cfg.Switches[0].JumpHost; sw0 == nil || sw0.Host != "198.51.100.5" || sw0.Port != 2202 {
		t.Fatalf("unexpected sw0 JumpHost: %+v", sw0)
	}
	if sw1 := cfg.Switches[1].JumpHost; sw1 == nil || sw1.Username != "direct" || sw1.Host != "198.51.100.9" || sw1.Port != 0 {
		t.Fatalf("unexpected sw1 JumpHost override: %+v", sw1)
	}
}

func TestLoadJumpHostPasswordCommands(t *testing.T) {
	path := writeTempConfig(t, `
platform: ios
username: admin
password: sw-password
default_vlan: "10"
jump_host:
  host: 192.0.2.5
  username: bastion
  password_command: "printf 'cmd-jump-pass\n'"
  key_passphrase_command: "printf 'cmd-key-pass\n'"
switches:
  - target: 192.0.2.10
  - target: 192.0.2.11
    jump_host:
      username: override-user
      password_command: "printf 'sw-jump-pass\n'"
`)

	cfg, err := Load(path, "", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.JumpHost.Password != "cmd-jump-pass" || cfg.JumpHost.KeyPassphrase != "cmd-key-pass" {
		t.Fatalf("unexpected global JumpHost command secrets: %+v", cfg.JumpHost)
	}
	if sw1 := cfg.Switches[1].JumpHost; sw1 == nil || sw1.Host != "192.0.2.5" || sw1.Username != "override-user" || sw1.Password != "sw-jump-pass" || sw1.KeyPassphrase != "cmd-key-pass" {
		t.Fatalf("unexpected sw1 merged JumpHost: %+v", sw1)
	}
}

func TestLoadJumpHostInheritanceAndOptOut(t *testing.T) {
	path := writeTempConfig(t, `
platform: ios
username: admin
password: sw-password
default_vlan: "10"
jump_host:
  host: 192.0.2.5
  port: 2222
  username: global-user
  key_file: ~/.ssh/global_key
  known_hosts: ~/.ssh/global_kh
switches:
  - target: 192.0.2.10
  - target: 192.0.2.11
    jump_host:
      username: local-user
      port: 2200
  - target: 192.0.2.12
    jump_host: none
  - target: 192.0.2.13
    jump_host:
      disabled: true
      password_command: "exit 1"
  - target: 192.0.2.14
    jump_host: null
`)

	cfg, err := Load(path, "", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Switches[0].JumpHost == nil || cfg.Switches[0].JumpHost.Username != "global-user" {
		t.Fatalf("sw0 should inherit global jump_host: %+v", cfg.Switches[0].JumpHost)
	}
	sw1 := cfg.Switches[1].JumpHost
	if sw1 == nil || sw1.Host != "192.0.2.5" || sw1.Port != 2200 || sw1.Username != "local-user" || sw1.KeyFile != "~/.ssh/global_key" || sw1.KnownHosts != "~/.ssh/global_kh" {
		t.Fatalf("sw1 partial override mismatch: %+v", sw1)
	}
	for _, idx := range []int{2, 3, 4} {
		if cfg.Switches[idx].JumpHost != nil {
			t.Fatalf("switch %d expected opt-out nil JumpHost, got %+v", idx, cfg.Switches[idx].JumpHost)
		}
	}
}

func TestLoadJumpHostValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "missing global host",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
jump_host:
  username: ops
switches:
  - target: 192.0.2.10
`,
			wantErr: "global jump_host.host is required",
		},
		{
			name: "missing switch host without global",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
switches:
  - target: 192.0.2.10
    jump_host:
      username: ops
`,
			wantErr: "jump_host.host is required for switch 192.0.2.10",
		},
		{
			name: "invalid global port",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
jump_host:
  host: 192.0.2.5
  port: 70000
switches:
  - target: 192.0.2.10
`,
			wantErr: "invalid global jump_host port 70000",
		},
		{
			name: "invalid switch port",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
switches:
  - target: 192.0.2.10
    jump_host:
      host: 192.0.2.5
      port: -1
`,
			wantErr: "invalid switch 192.0.2.10 jump_host port -1",
		},
		{
			name: "invalid scalar spec",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
jump_host: "user@"
switches:
  - target: 192.0.2.10
`,
			wantErr: "invalid global jump_host",
		},
		{
			name: "non scalar non mapping jump_host",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
jump_host:
  - one
switches:
  - target: 192.0.2.10
`,
			wantErr: "jump_host must be a string or mapping",
		},
		{
			name: "missing env var in jump_host",
			yaml: `
platform: ios
username: admin
default_vlan: "10"
jump_host:
  host: "${NEGEV_MISSING_JUMP_ENV}"
switches:
  - target: 192.0.2.10
`,
			wantErr: `environment variable "NEGEV_MISSING_JUMP_ENV" not set`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempConfig(t, tc.yaml)
			_, err := Load(path, "", 0)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q; want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}
