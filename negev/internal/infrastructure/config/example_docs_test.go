package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDocsExampleConfigLoads(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// negev/internal/infrastructure/config → repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	path := filepath.Join(root, "docs", "examples", "config.yaml")
	cfg, err := Load(path, "192.0.2.10", 0)
	if err != nil {
		t.Fatalf("Load docs/examples/config.yaml: %v", err)
	}
	if len(cfg.Switches) != 2 {
		t.Fatalf("switches = %d; want 2", len(cfg.Switches))
	}
	if cfg.Switches[0].Name != "access-a1" || cfg.Switches[0].Target != "192.0.2.10" {
		t.Fatalf("switch0 = %+v", cfg.Switches[0])
	}
}
