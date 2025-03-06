package transport

import (
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestSSHConnectFailure(t *testing.T) {
	sc := NewSSHClient(entities.SwitchConfig{
		Target: "127.0.0.1", Username: "admin", Password: "pass",
	})
	err := sc.Connect()
	if err == nil {
		t.Fatal("expected SSH connect failure against localhost")
	}
	if !strings.Contains(err.Error(), "failed to connect") && !strings.Contains(err.Error(), "failed to establish") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSHClientBasicsAndConnectFailure(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target: "127.0.0.1", Username: "admin", Password: "pass",
		EnablePassword: "enable", VerbosityLevel: 3,
	}
	sc := NewSSHClient(cfg)
	if sc.IsConnected() {
		t.Fatal("expected not connected")
	}
	sc.Disconnect()
	err := sc.Connect()
	if err == nil {
		t.Fatal("expected SSH connect failure against localhost")
	}
}
