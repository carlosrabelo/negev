package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestApplyJumpOptsCLIOverrideAndIsolation(t *testing.T) {
	switches := []entities.SwitchConfig{
		{Target: "192.0.2.10"},
		{
			Target: "192.0.2.11",
			JumpHost: &entities.JumpHostConfig{
				Host:     "198.51.100.1",
				Username: "old",
			},
		},
	}

	asks := 0
	out, err := ApplyJumpOpts(switches, JumpOpts{
		Spec:       "ops@192.0.2.5:2222",
		KeyFile:    "~/.ssh/bastion",
		KnownHosts: "~/.ssh/kh_bastion",
		AskPass: func() (string, error) {
			asks++
			return "prompted-jump-pass", nil
		},
	})
	if err != nil {
		t.Fatalf("ApplyJumpOpts failed: %v", err)
	}
	if asks != 1 {
		t.Fatalf("AskPass called %d times; want 1", asks)
	}
	for i, sw := range out {
		if sw.JumpHost == nil {
			t.Fatalf("switch %d JumpHost nil", i)
		}
		if sw.JumpHost.Host != "192.0.2.5" || sw.JumpHost.Port != 2222 || sw.JumpHost.Username != "ops" {
			t.Fatalf("switch %d JumpHost = %+v", i, sw.JumpHost)
		}
		if sw.JumpHost.KeyFile != "~/.ssh/bastion" || sw.JumpHost.KnownHosts != "~/.ssh/kh_bastion" || sw.JumpHost.Password != "prompted-jump-pass" {
			t.Fatalf("switch %d overrides = %+v", i, sw.JumpHost)
		}
	}
	if out[0].JumpHost == out[1].JumpHost {
		t.Fatal("each switch should get its own cloned JumpHostConfig")
	}
	if switches[0].JumpHost != nil {
		t.Fatal("input slice should not be mutated")
	}
}

func TestApplyJumpOptsDisableAndYAMLInheritance(t *testing.T) {
	switches := []entities.SwitchConfig{
		{
			Target: "192.0.2.10",
			JumpHost: &entities.JumpHostConfig{
				Host:     "192.0.2.5",
				Username: "yaml-user",
			},
		},
		{Target: "192.0.2.11"},
	}

	out, err := ApplyJumpOpts(switches, JumpOpts{
		KeyFile:    "/tmp/key",
		KnownHosts: "/tmp/kh",
		AskPass:    func() (string, error) { return "pw", nil },
	})
	if err != nil {
		t.Fatalf("ApplyJumpOpts YAML enrich failed: %v", err)
	}
	if out[0].JumpHost == nil || out[0].JumpHost.KeyFile != "/tmp/key" || out[0].JumpHost.KnownHosts != "/tmp/kh" || out[0].JumpHost.Password != "pw" {
		t.Fatalf("sw0 not enriched: %+v", out[0].JumpHost)
	}
	if out[1].JumpHost != nil {
		t.Fatalf("sw1 opted out of YAML jump should remain nil, got %+v", out[1].JumpHost)
	}

	disabled, err := ApplyJumpOpts(switches, JumpOpts{Spec: "none"})
	if err != nil {
		t.Fatalf("ApplyJumpOpts --jump none failed: %v", err)
	}
	for i, sw := range disabled {
		if sw.JumpHost != nil {
			t.Fatalf("switch %d expected nil after --jump none, got %+v", i, sw.JumpHost)
		}
	}
}

func TestApplyJumpOptsErrors(t *testing.T) {
	direct := []entities.SwitchConfig{{Target: "192.0.2.10"}}
	withJump := []entities.SwitchConfig{{
		Target:   "192.0.2.10",
		JumpHost: &entities.JumpHostConfig{Host: "192.0.2.5"},
	}}

	cases := []struct {
		name     string
		switches []entities.SwitchConfig
		opts     JumpOpts
		want     string
	}{
		{
			name:     "invalid spec",
			switches: direct,
			opts:     JumpOpts{Spec: "user@"},
			want:     "invalid jump host",
		},
		{
			name:     "key without jump",
			switches: direct,
			opts:     JumpOpts{KeyFile: "~/.ssh/id_ed25519"},
			want:     "--jump-key requires a jump host",
		},
		{
			name:     "known_hosts without jump",
			switches: direct,
			opts:     JumpOpts{KnownHosts: "~/.ssh/kh"},
			want:     "--jump-known-hosts requires a jump host",
		},
		{
			name:     "ask-pass without jump",
			switches: direct,
			opts:     JumpOpts{AskPass: func() (string, error) { return "x", nil }},
			want:     "--jump-ask-pass requires a jump host",
		},
		{
			name:     "none with key",
			switches: withJump,
			opts:     JumpOpts{Spec: "none", KeyFile: "/tmp/k"},
			want:     "--jump-key cannot be combined with --jump none",
		},
		{
			name:     "none with known_hosts",
			switches: withJump,
			opts:     JumpOpts{Spec: "off", KnownHosts: "/tmp/kh"},
			want:     "--jump-known-hosts cannot be combined with --jump off",
		},
		{
			name:     "none with ask-pass",
			switches: withJump,
			opts:     JumpOpts{Spec: "direct", AskPass: func() (string, error) { return "x", nil }},
			want:     "--jump-ask-pass cannot be combined with --jump direct",
		},
		{
			name:     "ask-pass error",
			switches: withJump,
			opts:     JumpOpts{AskPass: func() (string, error) { return "", errors.New("tty error") }},
			want:     "tty error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ApplyJumpOpts(tc.switches, tc.opts)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q; want %q", err.Error(), tc.want)
			}
		})
	}
}
