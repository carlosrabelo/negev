package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"
)

func TestWriteRunningConfigBackupPermissions(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteRunningConfigBackup(dir, "10.13.150.82", "hostname sw\n", time.Date(2026, 10, 29, 13, 18, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	wantName := "10.13.150.82_20261029T131800Z.cfg"
	if filepath.Base(path) != wantName {
		t.Fatalf("basename = %q; want %q", filepath.Base(path), wantName)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o; want 0600", info.Mode().Perm())
	}
}

func TestBackupCreationBeforeWrite(t *testing.T) {
	cli := iosScriptedClient()
	cli.responses["show running-config"] = "hostname before-write\n"
	dir := t.TempDir()

	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:      "10.0.0.1",
		Name:        "edge-1",
		Platform:    "ios",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}

	// sandbox=false → write mode → backup required before mutations
	if _, err := svc.Run(false, 0, false, BackupOpts{Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup files = %v, %v", entries, err)
	}
	if !strings.HasPrefix(entries[0].Name(), "edge-1_") || !strings.HasSuffix(entries[0].Name(), ".cfg") {
		t.Fatalf("unexpected backup name %q", entries[0].Name())
	}
	body, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil || !strings.Contains(string(body), "hostname before-write") {
		t.Fatalf("backup body = %q, %v", body, err)
	}

	runIdx, confIdx := -1, -1
	for i, cmd := range cli.executed {
		if cmd == "show running-config" && runIdx < 0 {
			runIdx = i
		}
		if strings.Contains(cmd, "switchport access vlan") && confIdx < 0 {
			confIdx = i
		}
	}
	if runIdx < 0 || confIdx < 0 {
		t.Fatalf("expected backup and configure commands, got %v", cli.executed)
	}
	if runIdx >= confIdx {
		t.Fatalf("backup must precede write commands: runIdx=%d confIdx=%d cmds=%v", runIdx, confIdx, cli.executed)
	}
}

func TestBackupFailureAbortsWrite(t *testing.T) {
	cli := iosScriptedClient()
	cli.failCmd = "show running-config"
	dir := t.TempDir()

	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:      "10.0.0.1",
		Platform:    "ios",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}

	_, err := svc.Run(false, 0, false, BackupOpts{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "backup failed") {
		t.Fatalf("expected backup abort, got %v", err)
	}
	for _, cmd := range cli.executed {
		if strings.Contains(cmd, "switchport access vlan") || strings.HasPrefix(cmd, "configure terminal") {
			t.Fatalf("write command executed after backup failure: %v", cli.executed)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no backup file should exist, got %v", entries)
	}
}

func TestBackupSkippedInSandboxWithoutForce(t *testing.T) {
	cli := iosScriptedClient()
	cli.responses["show running-config"] = "should-not-run\n"
	dir := t.TempDir()

	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:      "10.0.0.1",
		Platform:    "ios",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}
	if _, err := svc.Run(true, 0, false, BackupOpts{Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, cmd := range cli.executed {
		if cmd == "show running-config" {
			t.Fatal("sandbox without --backup must not fetch running-config")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("unexpected backups: %v", entries)
	}
}
