package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

func TestAuditLogAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	log, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	log.now = func() time.Time {
		return time.Date(2026, 10, 29, 13, 18, 0, 0, time.UTC)
	}
	log.user = func() string { return "carlos" }

	err = log.AppendPortVLAN(ports.PortVLANAudit{
		Target:     "10.13.150.82",
		SwitchName: "bh-a2-sw-01",
		Port:       "GigabitEthernet1/0/1",
		MAC:        "1c2aa32ef757",
		OldVLAN:    1,
		NewVLAN:    10,
		Sandbox:    false,
		Status:     "SUCCESS",
	})
	if err != nil {
		t.Fatalf("AppendPortVLAN: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %o; want 0644", info.Mode().Perm())
	}

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("expected one JSONL line")
	}
	var got map[string]any
	if err := json.Unmarshal(sc.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	checks := map[string]any{
		"timestamp":   "2026-10-29T13:18:00Z",
		"user":        "carlos",
		"target":      "10.13.150.82",
		"switch_name": "bh-a2-sw-01",
		"port":        "GigabitEthernet1/0/1",
		"mac":         "1c2aa32ef757",
		"old_vlan":    float64(1),
		"new_vlan":    float64(10),
		"action":      "SET_PORT_VLAN",
		"sandbox":     false,
		"status":      "SUCCESS",
	}
	for k, want := range checks {
		if got[k] != want {
			t.Fatalf("%s = %#v; want %#v (full=%v)", k, got[k], want, got)
		}
	}
	if sc.Scan() {
		t.Fatal("expected exactly one line")
	}
}

func TestAuditLogDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "audit.log")
	log, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("parent dir missing: %v", err)
	}
	if err := log.AppendPortVLAN(ports.PortVLANAudit{
		Target:  "10.0.0.1",
		Port:    "Gi1/0/1",
		OldVLAN: 1,
		NewVLAN: 20,
		Status:  "SUCCESS",
		Sandbox: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("audit file missing: %v", err)
	}
}
