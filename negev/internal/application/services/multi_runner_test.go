package services

import (
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
)

func sampleConfig() *config.Config {
	return &config.Config{Switches: []entities.SwitchConfig{
		{Name: "bh-a2-sw-01", Target: "10.13.150.82", Tags: []string{"distribution", "building-a"}, Platform: "ios"},
		{Name: "bh-a2-sw-02", Target: "10.13.150.83", Tags: []string{"access", "building-a"}, Platform: "ios"},
		{Name: "", Target: "10.13.150.84", Tags: []string{"distribution"}, Platform: "nxos"},
	}}
}

func TestMultiRunnerTargetResolution(t *testing.T) {
	cfg := sampleConfig()

	got, err := ResolveTargets(cfg, TargetSelect{Target: "10.13.150.82"})
	if err != nil || len(got) != 1 || got[0].Target != "10.13.150.82" {
		t.Fatalf("IP resolve = %#v, %v", got, err)
	}

	got, err = ResolveTargets(cfg, TargetSelect{Target: "bh-a2-sw-01"})
	if err != nil || len(got) != 1 || got[0].Target != "10.13.150.82" {
		t.Fatalf("name resolve = %#v, %v", got, err)
	}

	got, err = ResolveTargets(cfg, TargetSelect{Target: "10.13.150.82,bh-a2-sw-02,10.13.150.82"})
	if err != nil {
		t.Fatalf("comma resolve: %v", err)
	}
	if len(got) != 2 || got[0].Target != "10.13.150.82" || got[1].Target != "10.13.150.83" {
		t.Fatalf("comma resolve order/dedupe = %#v", targetsOf(got))
	}

	got, err = ResolveTargets(cfg, TargetSelect{All: true})
	if err != nil || len(got) != 3 {
		t.Fatalf("--all = %#v, %v", got, err)
	}

	if _, err := ResolveTargets(cfg, TargetSelect{Target: "missing-sw"}); err == nil || !strings.Contains(err.Error(), `target "missing-sw" not found`) {
		t.Fatalf("missing target = %v", err)
	}

	if _, err := ResolveTargets(cfg, TargetSelect{}); err == nil {
		t.Fatal("expected error when no selector set")
	}
	if _, err := ResolveTargets(cfg, TargetSelect{All: true, Group: "distribution"}); err == nil {
		t.Fatal("expected mutual exclusion error")
	}
}

func TestMultiRunnerTagFiltering(t *testing.T) {
	cfg := sampleConfig()

	got, err := ResolveTargets(cfg, TargetSelect{Group: "distribution"})
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	if len(got) != 2 || got[0].Target != "10.13.150.82" || got[1].Target != "10.13.150.84" {
		t.Fatalf("group distribution = %#v", targetsOf(got))
	}

	got, err = ResolveTargets(cfg, TargetSelect{Group: "building-a"})
	if err != nil || len(got) != 2 {
		t.Fatalf("building-a = %#v, %v", got, err)
	}

	if _, err := ResolveTargets(cfg, TargetSelect{Group: "core"}); err == nil || !strings.Contains(err.Error(), `no switches with tag "core"`) {
		t.Fatalf("empty group = %v", err)
	}
}

func targetsOf(switches []entities.SwitchConfig) []string {
	out := make([]string, len(switches))
	for i, sw := range switches {
		out[i] = sw.Target
	}
	return out
}
