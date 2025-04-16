package transport

import (
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestAuthWaitPatternsUsername(t *testing.T) {
	got := authWaitPatterns("Username:")
	want := []string{"Username:", "login:", "User:"}
	if len(got) != len(want) {
		t.Fatalf("authWaitPatterns(Username:) = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("authWaitPatterns(Username:) = %v; want %v", got, want)
		}
	}
	if !outputContainsAny("bg-sw-02 login: ", got) {
		t.Fatal("expected login: banner to satisfy Username: wait")
	}
	if pats := authWaitPatterns("Password:"); len(pats) != 1 || pats[0] != "Password:" {
		t.Fatalf("Password: patterns = %v", pats)
	}
}

func TestShouldSkipEnableStep(t *testing.T) {
	if !shouldSkipEnableStep(true, ">", "enable\n", "secret") {
		t.Fatal("expected enable prompt to be skipped when already privileged")
	}
	if !shouldSkipEnableStep(true, "Password:", "secret\n", "secret") {
		t.Fatal("expected enable password to be skipped when already privileged")
	}
	if shouldSkipEnableStep(true, "#", "terminal length 0\n", "secret") {
		t.Fatal("must still send terminal length at privileged prompt")
	}
	if shouldSkipEnableStep(false, ">", "enable\n", "secret") {
		t.Fatal("must not skip enable when still in user exec")
	}
}

func TestSSHAuthMethodsIncludeKeyboardInteractive(t *testing.T) {
	methods := sshAuthMethods("secret")
	if len(methods) != 2 {
		t.Fatalf("expected password + keyboard-interactive, got %d methods", len(methods))
	}
}

func TestSSHClientBasicsAndConnectFailure(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target:         "127.0.0.1",
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable",
		VerbosityLevel: 3,
	}
	sc := NewSSHClient(cfg)
	sc.SetAuthSequence([]entities.AuthPrompt{{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"}})
	if sc.IsConnected() {
		t.Fatal("expected not connected")
	}
	sc.Disconnect()

	err := sc.Connect()
	if err == nil {
		t.Fatal("expected SSH connect failure against localhost")
	}
	if !strings.Contains(err.Error(), "failed to connect") && !strings.Contains(err.Error(), "failed to establish") {
		t.Fatalf("unexpected error: %v", err)
	}
}
