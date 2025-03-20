package dmos

import (
	"errors"
	"testing"
)

type mockCacheRepo struct {
	target      string
	cmdCount    int
	lastCommand string
}

func (m *mockCacheRepo) Connect() error    { return nil }
func (m *mockCacheRepo) Disconnect()       {}
func (m *mockCacheRepo) IsConnected() bool { return true }
func (m *mockCacheRepo) ExecuteCommand(cmd string) (string, error) {
	m.cmdCount++
	m.lastCommand = cmd
	if cmd == "show interfaces switchport" {
		return "switchport output for " + m.target, nil
	}
	return "", errors.New("unknown command")
}
func (m *mockCacheRepo) GetTarget() string {
	return m.target
}

func TestSwitchportCacheKeyedByTarget(t *testing.T) {
	clearSwitchportCache()

	repo1 := &mockCacheRepo{target: "192.168.1.10"}
	repo2 := &mockCacheRepo{target: "192.168.1.20"}

	// 1. Primeira chamada para repo1: deve executar no repo
	out1, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out1 != "switchport output for 192.168.1.10" {
		t.Errorf("unexpected output: %q", out1)
	}
	if repo1.cmdCount != 1 {
		t.Errorf("expected 1 command execution, got %d", repo1.cmdCount)
	}

	// 2. Segunda chamada para repo1 (deve vir da cache)
	out1Cached, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out1Cached != "switchport output for 192.168.1.10" {
		t.Errorf("unexpected output: %q", out1Cached)
	}
	if repo1.cmdCount != 1 {
		t.Errorf("expected still 1 command execution (cached), got %d", repo1.cmdCount)
	}

	// 3. Chamada para repo2 (outro target): deve executar no repo (sem colisão com repo1)
	out2, err := getSwitchportOutput(repo2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out2 != "switchport output for 192.168.1.20" {
		t.Errorf("unexpected output: %q", out2)
	}
	if repo2.cmdCount != 1 {
		t.Errorf("expected 1 command execution, got %d", repo2.cmdCount)
	}

	// 4. Limpar cache: deve resetar a cache e forçar nova execução
	clearSwitchportCache()
	out1AfterClear, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out1AfterClear != "switchport output for 192.168.1.10" {
		t.Errorf("unexpected output: %q", out1AfterClear)
	}
	if repo1.cmdCount != 2 {
		t.Errorf("expected 2 command executions after cache clear, got %d", repo1.cmdCount)
	}
}
