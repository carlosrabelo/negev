package dmswitch

import (
	"errors"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockCacheRepo struct {
	target      string
	cmdCount    int
	lastCommand string
}

func (m *mockCacheRepo) Connect() error                                    { return nil }
func (m *mockCacheRepo) Disconnect()                                       {}
func (m *mockCacheRepo) IsConnected() bool                                 { return true }
func (m *mockCacheRepo) SetSessionProfile(profile entities.SessionProfile) {}
func (m *mockCacheRepo) GetTarget() string                                 { return m.target }
func (m *mockCacheRepo) ExecuteCommand(cmd string) (string, error) {
	m.cmdCount++
	m.lastCommand = cmd
	if cmd == "show interfaces switchport" {
		return "switchport output for " + m.target, nil
	}
	return "", errors.New("unknown command")
}

func TestSwitchportCacheKeyedByTarget(t *testing.T) {
	clearSwitchportCache()
	t.Cleanup(clearSwitchportCache)

	repo1 := &mockCacheRepo{target: "192.168.1.10"}
	repo2 := &mockCacheRepo{target: "192.168.1.20"}

	out1, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out1 != "switchport output for 192.168.1.10" {
		t.Fatalf("out1 = %q", out1)
	}
	if repo1.cmdCount != 1 {
		t.Fatalf("repo1.cmdCount = %d; expected 1", repo1.cmdCount)
	}

	out1Cached, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if out1Cached != out1 {
		t.Fatalf("cached out1 = %q; expected %q", out1Cached, out1)
	}
	if repo1.cmdCount != 1 {
		t.Fatalf("repo1.cmdCount after cache hit = %d; expected 1", repo1.cmdCount)
	}

	out2, err := getSwitchportOutput(repo2)
	if err != nil {
		t.Fatalf("unexpected error for repo2: %v", err)
	}
	if out2 != "switchport output for 192.168.1.20" {
		t.Fatalf("out2 = %q", out2)
	}
	if repo2.cmdCount != 1 {
		t.Fatalf("repo2.cmdCount = %d; expected 1", repo2.cmdCount)
	}

	clearSwitchportCache()
	out1AfterClear, err := getSwitchportOutput(repo1)
	if err != nil {
		t.Fatalf("unexpected error after clear: %v", err)
	}
	if out1AfterClear != out1 {
		t.Fatalf("out1 after clear = %q", out1AfterClear)
	}
	if repo1.cmdCount != 2 {
		t.Fatalf("repo1.cmdCount after clear = %d; expected 2", repo1.cmdCount)
	}
}
