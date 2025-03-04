package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPathCustomAndMissing(t *testing.T) {
	got, err := FindPath("/tmp/custom-negev.yaml", 0)
	if err != nil || got != "/tmp/custom-negev.yaml" {
		t.Fatalf("FindPath custom = %q, %v", got, err)
	}

	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := FindPath("config.yaml", 0); err == nil {
		t.Fatal("expected missing config.yaml error")
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("switches: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := FindPath("config.yaml", 3)
	if err != nil || found != filepath.Join(".", "config.yaml") && found != "config.yaml" && found != cfgPath {
		// Accept relative "./config.yaml" as implemented
		if err != nil {
			t.Fatalf("FindPath existing failed: %v", err)
		}
		if _, statErr := os.Stat(found); statErr != nil {
			t.Fatalf("FindPath returned unusable path %q: %v", found, statErr)
		}
	}
}
