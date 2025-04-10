package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestMultiRunnerConcurrency(t *testing.T) {
	switches := make([]entities.SwitchConfig, 8)
	for i := range switches {
		switches[i] = entities.SwitchConfig{Target: fmt.Sprintf("10.0.0.%d", i+1), Platform: "ios"}
	}

	var inFlight, maxSeen int32
	runner := observeConcurrency(func(sw entities.SwitchConfig, stdout, stderr io.Writer) (entities.RunResult, error) {
		time.Sleep(50 * time.Millisecond)
		return entities.RunResult{}, nil
	}, &inFlight, &maxSeen)

	const limit = 3
	_ = RunMulti(switches, MultiRunOptions{
		Concurrency: limit,
		Stdout:      io.Discard,
		Stderr:      io.Discard,
		Runner:      runner,
	})
	if got := atomic.LoadInt32(&maxSeen); got > int32(limit) {
		t.Fatalf("max in-flight = %d; want <= %d", got, limit)
	}
	if got := atomic.LoadInt32(&maxSeen); got < 2 {
		t.Fatalf("max in-flight = %d; expected concurrent workers", got)
	}
}

func TestMultiRunnerOutputIsolation(t *testing.T) {
	switches := []entities.SwitchConfig{
		{Name: "sw-a", Target: "10.0.0.1", Platform: "ios"},
		{Name: "sw-b", Target: "10.0.0.2", Platform: "ios"},
	}
	var outBuf bytes.Buffer
	runner := func(sw entities.SwitchConfig, stdout, stderr io.Writer) (entities.RunResult, error) {
		fmt.Fprintf(stdout, "line1-from-%s\n", sw.Name)
		time.Sleep(20 * time.Millisecond)
		fmt.Fprintf(stdout, "line2-from-%s\n", sw.Name)
		return entities.RunResult{PortsChanged: 1}, nil
	}

	result := RunMulti(switches, MultiRunOptions{
		Concurrency: 2,
		Write:       false,
		Stdout:      &outBuf,
		Stderr:      io.Discard,
		Runner:      runner,
	})
	if result.ExitCode(false) != 2 {
		t.Fatalf("exit = %d; want 2", result.ExitCode(false))
	}

	text := outBuf.String()
	// Each target's lines must appear as contiguous prefixed blocks (no interleave
	// of sw-a/sw-b within a flush), i.e. both lines for a label stay together.
	for _, label := range []string{"sw-a", "sw-b"} {
		block := fmt.Sprintf("[%s] line1-from-%s\n[%s] line2-from-%s\n", label, label, label, label)
		if !strings.Contains(text, block) {
			t.Fatalf("missing contiguous block for %s in:\n%s", label, text)
		}
	}
	if !strings.Contains(text, "TARGET") || !strings.Contains(text, "SIMULATED") {
		t.Fatalf("missing summary table:\n%s", text)
	}
}
