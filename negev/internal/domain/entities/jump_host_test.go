package entities

import (
	"strings"
	"testing"
)

func TestJumpHostHelpers(t *testing.T) {
	var nilJump *JumpHostConfig
	if nilJump.IsEnabled() {
		t.Fatal("nil JumpHostConfig must not be enabled")
	}
	if nilJump.EffectivePort() != 22 {
		t.Fatalf("nil EffectivePort = %d; want 22", nilJump.EffectivePort())
	}
	if nilJump.Addr() != "" {
		t.Fatalf("nil Addr = %q; want empty", nilJump.Addr())
	}
	if nilJump.Clone() != nil {
		t.Fatal("nil Clone must return nil")
	}

	j := &JumpHostConfig{Host: "192.0.2.5"}
	if !j.IsEnabled() {
		t.Fatal("expected JumpHostConfig with host to be enabled")
	}
	if j.EffectivePort() != 22 {
		t.Fatalf("default EffectivePort = %d; want 22", j.EffectivePort())
	}
	if j.Addr() != "192.0.2.5:22" {
		t.Fatalf("Addr = %q; want 192.0.2.5:22", j.Addr())
	}

	j.Port = 2222
	j.Username = "ops"
	if j.EffectivePort() != 2222 || j.Addr() != "192.0.2.5:2222" {
		t.Fatalf("custom port Addr = %q, port=%d", j.Addr(), j.EffectivePort())
	}

	cp := j.Clone()
	if cp == j || cp.Host != j.Host || cp.Port != j.Port || cp.Username != j.Username {
		t.Fatalf("Clone mismatch: %+v vs %+v", cp, j)
	}
	cp.Port = 22
	if j.Port != 2222 {
		t.Fatal("modifying clone mutated original")
	}

	j.Disabled = true
	if j.IsEnabled() {
		t.Fatal("disabled JumpHostConfig must not be enabled")
	}

	blank := &JumpHostConfig{Host: "   "}
	if blank.IsEnabled() {
		t.Fatal("whitespace host must not be enabled")
	}
}

func TestParseJumpSpecValid(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want JumpHostConfig
	}{
		{
			name: "host only",
			spec: "bastion.example.com",
			want: JumpHostConfig{Host: "bastion.example.com"},
		},
		{
			name: "user and host",
			spec: "carlos@192.0.2.5",
			want: JumpHostConfig{Host: "192.0.2.5", Username: "carlos"},
		},
		{
			name: "user host port",
			spec: "carlos@192.0.2.5:2222",
			want: JumpHostConfig{Host: "192.0.2.5", Port: 2222, Username: "carlos"},
		},
		{
			name: "ssh scheme",
			spec: "SSH://admin@bastion.internal:2200",
			want: JumpHostConfig{Host: "bastion.internal", Port: 2200, Username: "admin"},
		},
		{
			name: "user with at sign",
			spec: "carlos@corp.example@192.0.2.5:22",
			want: JumpHostConfig{Host: "192.0.2.5", Port: 22, Username: "carlos@corp.example"},
		},
		{
			name: "ipv6 bracketed with port",
			spec: "ops@[2001:db8::1]:2222",
			want: JumpHostConfig{Host: "2001:db8::1", Port: 2222, Username: "ops"},
		},
		{
			name: "ipv6 bracketed without port",
			spec: "[2001:db8::1]",
			want: JumpHostConfig{Host: "2001:db8::1"},
		},
		{
			name: "ipv6 bare",
			spec: "ops@2001:db8::1",
			want: JumpHostConfig{Host: "2001:db8::1", Username: "ops"},
		},
		{
			name: "disable none",
			spec: "none",
			want: JumpHostConfig{Disabled: true},
		},
		{
			name: "disable off uppercase",
			spec: " OFF ",
			want: JumpHostConfig{Disabled: true},
		},
		{
			name: "disable direct",
			spec: "direct",
			want: JumpHostConfig{Disabled: true},
		},
		{
			name: "disable false",
			spec: "false",
			want: JumpHostConfig{Disabled: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseJumpSpec(tc.spec)
			if err != nil {
				t.Fatalf("ParseJumpSpec(%q) unexpected error: %v", tc.spec, err)
			}
			if *got != tc.want {
				t.Fatalf("ParseJumpSpec(%q) = %+v; want %+v", tc.spec, *got, tc.want)
			}
		})
	}
}

func TestParseJumpSpecInvalid(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr string
	}{
		{"", "empty jump host specification"},
		{"   ", "empty jump host specification"},
		{"ssh://", "invalid jump host specification"},
		{"@bastion", "invalid jump host specification"},
		{"user@", "invalid jump host specification"},
		{"user@host/path", "invalid jump host specification"},
		{"user@host name", "invalid jump host specification"},
		{":22", "missing host"},
		{"[]:22", "missing host"},
		{"[not-an-ip]", "invalid jump host IPv6"},
		{"[2001:db8::1", "invalid jump host specification"},
		{"not:an:ip:addr", "invalid jump host IPv6"},
		{"bastion:0", "invalid jump host port"},
		{"bastion:65536", "invalid jump host port"},
		{"bastion:abc", "invalid jump host port"},
	}

	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := ParseJumpSpec(tc.spec)
			if err == nil {
				t.Fatalf("ParseJumpSpec(%q) expected error containing %q, got nil", tc.spec, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ParseJumpSpec(%q) error = %q; want substring %q", tc.spec, err.Error(), tc.wantErr)
			}
		})
	}
}
