package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockClient struct {
	connected bool
	prompts   []entities.AuthPrompt
}

func (m *mockClient) Connect() error {
	m.connected = true
	return nil
}

func (m *mockClient) Disconnect() {
	m.connected = false
}

func (m *mockClient) ExecuteCommand(cmd string) (string, error) {
	return "", nil
}

func (m *mockClient) IsConnected() bool {
	return m.connected
}

func (m *mockClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	m.prompts = prompts
}

func TestSwitchAdapterGetTargetAndSetAuthSequence(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target: "192.168.1.10",
	}

	adapter := NewSwitchAdapter(cfg)
	if adapter.GetTarget() != "192.168.1.10" {
		t.Errorf("adapter.GetTarget() = %q; expected \"192.168.1.10\"", adapter.GetTarget())
	}

	mockCli := &mockClient{}
	adapter.client = mockCli

	prompts := []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "admin\n"},
	}

	adapter.SetAuthSequence(prompts)
	if len(mockCli.prompts) != 1 || mockCli.prompts[0].WaitFor != "Username:" {
		t.Errorf("expected prompts to be propagated to client, got %+v", mockCli.prompts)
	}
}
