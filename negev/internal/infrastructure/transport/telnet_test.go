package transport

import (
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestTelnetClientBasicsAndConnectFailure(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target:         "127.0.0.1",
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable",
		VerbosityLevel: 3,
	}
	tc := NewTelnetClient(cfg)
	tc.SetAuthSequence([]entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
	})
	if tc.IsConnected() {
		t.Fatal("expected not connected")
	}
	tc.Disconnect()

	err := tc.Connect()
	if err == nil {
		t.Fatal("expected Telnet connect failure against localhost")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("unexpected error: %v", err)
	}
}
