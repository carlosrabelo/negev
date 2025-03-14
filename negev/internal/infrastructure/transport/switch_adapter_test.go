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

func (m *mockClient) Disconnect() {
	m.connected = false
}

func (m *mockClient) ExecuteCommand(cmd string) (string, error) {
	m.executed = append(m.executed, cmd)
	return m.commandOut, m.commandErr
}

func (m *mockClient) IsConnected() bool {
	return m.connected
}

func (m *mockClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	m.prompts = prompts
}

func TestSwitchAdapterGetTargetAndSetAuthSequence(t *testing.T) {
	cfg := entities.SwitchConfig{Target: "192.168.1.10"}
	adapter := NewSwitchAdapter(cfg)
	if adapter.GetTarget() != "192.168.1.10" {
		t.Errorf("adapter.GetTarget() = %q; expected \"192.168.1.10\"", adapter.GetTarget())
	}

	mockCli := &mockClient{}
	adapter.client = mockCli
	prompts := []entities.AuthPrompt{{WaitFor: "Username:", SendCmd: "admin\n"}}
	adapter.SetAuthSequence(prompts)
	if len(mockCli.prompts) != 1 || mockCli.prompts[0].WaitFor != "Username:" {
		t.Errorf("expected prompts to be propagated to client, got %+v", mockCli.prompts)
	}
}

func TestSwitchAdapterLifecycleWithClient(t *testing.T) {
	mockCli := &mockClient{commandOut: "ok"}
	adapter := NewSwitchAdapterWithClient(entities.SwitchConfig{Target: "10.0.0.1"}, mockCli)

	if adapter.IsConnected() {
		t.Fatal("expected not connected before Connect")
	}
	if err := adapter.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !adapter.IsConnected() {
		t.Fatal("expected connected after Connect")
	}

	out, err := adapter.ExecuteCommand("show version")
	if err != nil || out != "ok" {
		t.Fatalf("ExecuteCommand = %q, %v", out, err)
	}
	if len(mockCli.executed) != 1 {
		t.Fatalf("expected one executed command, got %v", mockCli.executed)
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
		t.Fatal("expected connect error from ExecuteCommand")
	}
}

func TestSwitchAdapterDisconnectNilClient(t *testing.T) {
	adapter := NewSwitchAdapter(entities.SwitchConfig{Target: "10.0.0.1"})
	adapter.Disconnect()
	if adapter.IsConnected() {
		t.Fatal("expected not connected")
	}
}

func TestSwitchAdapterSetAuthSequenceCreatesClient(t *testing.T) {
	t.Cleanup(CloseAll)
	cfg := entities.SwitchConfig{
		Target:    "203.0.113.10",
		Transport: "ssh",
		Username:  "u",
		Password:  "p",
	}
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
