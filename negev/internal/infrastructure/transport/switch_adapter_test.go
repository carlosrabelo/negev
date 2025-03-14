package transport

import (
	"errors"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockClient struct {
	connected  bool
	prompts    []entities.AuthPrompt
	connectErr error
	executed   []string
	commandOut string
	commandErr error
}

func (m *mockClient) Connect() error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}
func (m *mockClient) Disconnect()                       { m.connected = false }
func (m *mockClient) ExecuteCommand(cmd string) (string, error) {
	m.executed = append(m.executed, cmd)
	return m.commandOut, m.commandErr
}
func (m *mockClient) IsConnected() bool { return m.connected }
func (m *mockClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	m.prompts = prompts
}

func TestSwitchAdapterGetTargetAndSetAuthSequence(t *testing.T) {
	cfg := entities.SwitchConfig{Target: "192.168.1.10"}
	adapter := NewSwitchAdapter(cfg)
	if adapter.GetTarget() != "192.168.1.10" {
		t.Errorf("adapter.GetTarget() = %q", adapter.GetTarget())
	}
	mockCli := &mockClient{}
	adapter.client = mockCli
	adapter.SetAuthSequence([]entities.AuthPrompt{{WaitFor: "Username:", SendCmd: "admin\n"}})
	if len(mockCli.prompts) != 1 || mockCli.prompts[0].WaitFor != "Username:" {
		t.Errorf("expected prompts on client, got %+v", mockCli.prompts)
	}
}

func TestSwitchAdapterLifecycleWithClient(t *testing.T) {
	mockCli := &mockClient{commandOut: "ok"}
	adapter := NewSwitchAdapterWithClient(entities.SwitchConfig{Target: "10.0.0.1"}, mockCli)
	if err := adapter.Connect(); err != nil {
		t.Fatal(err)
	}
	out, err := adapter.ExecuteCommand("show version")
	if err != nil || out != "ok" {
		t.Fatalf("ExecuteCommand = %q, %v", out, err)
	}
	adapter.Disconnect()
	if adapter.IsConnected() {
		t.Fatal("expected disconnected")
	}
}

func TestSwitchAdapterExecuteCommandConnectError(t *testing.T) {
	mockCli := &mockClient{connectErr: errors.New("down")}
	adapter := NewSwitchAdapterWithClient(entities.SwitchConfig{Target: "10.0.0.1"}, mockCli)
	if _, err := adapter.ExecuteCommand("show version"); err == nil {
		t.Fatal("expected connect error")
	}
}

func TestSwitchAdapterDisconnectNilClient(t *testing.T) {
	adapter := NewSwitchAdapter(entities.SwitchConfig{Target: "10.0.0.1"})
	adapter.Disconnect()
}

func TestSwitchAdapterSetAuthSequenceCreatesClient(t *testing.T) {
	t.Cleanup(CloseAll)
	cfg := entities.SwitchConfig{Target: "203.0.113.10", Transport: "ssh", Username: "u", Password: "p"}
	adapter := NewSwitchAdapter(cfg)
	adapter.SetAuthSequence([]entities.AuthPrompt{{WaitFor: "#", SendCmd: ""}})
	if adapter.client == nil {
		t.Fatal("expected client to be created")
	}
	sshCli, ok := adapter.client.(*SSHClient)
	if !ok {
		t.Fatalf("expected *SSHClient, got %T", adapter.client)
	}
	if len(sshCli.authSequence) != 1 {
		t.Fatalf("expected auth sequence on SSH client, got %+v", sshCli.authSequence)
	}
}
