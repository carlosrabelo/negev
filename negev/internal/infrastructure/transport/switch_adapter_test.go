package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type stubClient struct{ connected bool }

func (s *stubClient) Connect() error                  { s.connected = true; return nil }
func (s *stubClient) Disconnect()                     { s.connected = false }
func (s *stubClient) ExecuteCommand(string) (string, error) { return "ok", nil }
func (s *stubClient) IsConnected() bool               { return s.connected }

func TestSwitchAdapterDelegates(t *testing.T) {
	stub := &stubClient{}
	sa := NewSwitchAdapterWithClient(entities.SwitchConfig{Target: "1.1.1.1"}, stub)
	if err := sa.Connect(); err != nil || !sa.IsConnected() {
		t.Fatalf("connect: %v connected=%v", err, sa.IsConnected())
	}
	out, err := sa.ExecuteCommand("show version")
	if err != nil || out != "ok" {
		t.Fatalf("exec: %q %v", out, err)
	}
	if sa.GetTarget() != "1.1.1.1" {
		t.Fatal("GetTarget")
	}
	sa.Disconnect()
	if sa.IsConnected() {
		t.Fatal("expected disconnected")
	}
}
