package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"golang.org/x/term"
)

func TestExitStatus(t *testing.T) {
	tests := []struct {
		name   string
		write  bool
		result entities.RunResult
		want   int
	}{
		{name: "clean sandbox", write: false, result: entities.RunResult{}, want: 0},
		{name: "sandbox port changes", write: false, result: entities.RunResult{PortsChanged: 1}, want: 2},
		{name: "sandbox vlan create", write: false, result: entities.RunResult{VLANsCreated: 1}, want: 2},
		{name: "sandbox vlan delete", write: false, result: entities.RunResult{VLANsDeleted: 1}, want: 2},
		{name: "write with changes", write: true, result: entities.RunResult{PortsChanged: 3}, want: 0},
		{name: "ports failed only", write: false, result: entities.RunResult{PortsFailed: 2}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitStatus(tt.write, tt.result); got != tt.want {
				t.Fatalf("exitStatus(%v, %+v) = %d; want %d", tt.write, tt.result, got, tt.want)
			}
		})
	}
}

func TestRunMissingTarget(t *testing.T) {
	if code := run(nil); code != 1 {
		t.Fatalf("run(nil) = %d; want 1", code)
	}
	if code := run([]string{"--config", "/nonexistent/negev-config.yaml"}); code != 1 {
		t.Fatalf("run(no selector) = %d; want 1", code)
	}
}

func TestRunVersion(t *testing.T) {
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("run(--version) = %d; want 0", code)
	}
}

func TestRunConfigError(t *testing.T) {
	code := run([]string{"--target", "10.0.0.1", "--config", "/nonexistent/negev-config.yaml"})
	if code != 1 {
		t.Fatalf("run(bad config) = %d; want 1", code)
	}
}

func TestRunAnnouncesConfigPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte(`username: admin
password: secret
switches:
  - target: 192.0.2.10
    platform: ios
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	code := run([]string{"--config", cfgPath})
	_ = w.Close()
	os.Stderr = old
	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if code != 1 {
		t.Fatalf("run = %d; want 1 (missing target)", code)
	}
	want := "Using config: " + cfgPath
	if !strings.Contains(string(out), want) {
		t.Fatalf("stderr missing %q; got:\n%s", want, out)
	}
}

func TestAskPassFlag(t *testing.T) {
	calls := 0
	readPassword = func(fd int) ([]byte, error) {
		calls++
		switch calls {
		case 1:
			return []byte("prompt-secret"), nil
		case 2:
			return []byte("prompt-enable"), nil
		default:
			t.Fatalf("unexpected readPassword call %d", calls)
			return nil, nil
		}
	}
	t.Cleanup(func() { readPassword = term.ReadPassword })

	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:         "10.0.0.1",
		Password:       "yaml-pass",
		EnablePassword: "yaml-enable",
	}}}

	ask := services.AskPassFunc(promptAskPass)
	pw, en, err := ask()
	if err != nil {
		t.Fatalf("askPass: %v", err)
	}
	if pw != "prompt-secret" || en != "prompt-enable" {
		t.Fatalf("askPass = %q/%q", pw, en)
	}

	for i := range cfg.Switches {
		if cfg.Switches[i].Target != "10.0.0.1" {
			continue
		}
		cfg.Switches[i].Password = pw
		if en != "" {
			cfg.Switches[i].EnablePassword = en
		}
	}
	if cfg.Switches[0].Password != "prompt-secret" {
		t.Fatalf("password not overridden: %q", cfg.Switches[0].Password)
	}
	if cfg.Switches[0].EnablePassword != "prompt-enable" {
		t.Fatalf("enable not overridden: %q", cfg.Switches[0].EnablePassword)
	}
	if calls != 2 {
		t.Fatalf("readPassword calls = %d; want 2", calls)
	}
}
