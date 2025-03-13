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

func TestShouldSkipSSHEnableStep(t *testing.T) {
	if !shouldSkipSSHEnableStep(true, ">", "enable\n", "secret") {
		t.Fatal("expected enable prompt to be skipped when already privileged")
	}
	if !shouldSkipSSHEnableStep(true, "Password:", "secret\n", "secret") {
		t.Fatal("expected enable password to be skipped when already privileged")
	}
	if shouldSkipSSHEnableStep(true, "#", "terminal length 0\n", "secret") {
		t.Fatal("must still send terminal length at privileged prompt")
	}
	if shouldSkipSSHEnableStep(false, ">", "enable\n", "secret") {
		t.Fatal("must not skip enable when still in user exec")
	}
}
