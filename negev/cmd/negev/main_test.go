package main

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
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
