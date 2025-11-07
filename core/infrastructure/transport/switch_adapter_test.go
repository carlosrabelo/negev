package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

// MockClient implements the Client interface for testing
type MockClient struct {
	connected     bool
	connectError  error
	executedCmds  []string
	cmdResponses  map[string]string
	cmdErrors     map[string]error
}

func (m *MockClient) Connect() error {
	if m.connectError != nil {
		return m.connectError
	}
	m.connected = true
	return nil
}

func (m *MockClient) Disconnect() {
	m.connected = false
}

func (m *MockClient) ExecuteCommand(cmd string) (string, error) {
	m.executedCmds = append(m.executedCmds, cmd)
	if m.cmdErrors != nil {
		if err, exists := m.cmdErrors[cmd]; exists {
			return "", err
		}
	}
	if m.cmdResponses != nil {
		if resp, exists := m.cmdResponses[cmd]; exists {
			return resp, nil
		}
	}
	return "mock response", nil
}

func (m *MockClient) IsConnected() bool {
	return m.connected
}

func TestNewSwitchAdapter(t *testing.T) {
	mockClient := &MockClient{}
	adapter := NewSwitchAdapter(mockClient)

	if adapter == nil {
		t.Fatal("NewSwitchAdapter() returned nil")
	}

	if adapter.client != mockClient {
		t.Error("NewSwitchAdapter() did not set client correctly")
	}
}

func TestSwitchAdapter_Connect(t *testing.T) {
	tests := []struct {
		name        string
		connectErr  error
		expectErr   bool
		expectConn  bool
	}{
		{
			name:       "successful connection",
			connectErr: nil,
			expectErr:  false,
			expectConn: true,
		},
		{
			name:        "connection error",
			connectErr:  &testError{"connection failed"},
			expectErr:   true,
			expectConn:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{connectError: tt.connectErr}
			adapter := NewSwitchAdapter(mockClient)

			err := adapter.Connect()

			if (err != nil) != tt.expectErr {
				t.Errorf("Connect() error = %v, expectErr %v", err, tt.expectErr)
			}

			if mockClient.connected != tt.expectConn {
				t.Errorf("Connect() connected = %v, expectConn %v", mockClient.connected, tt.expectConn)
			}
		})
	}
}

func TestSwitchAdapter_Disconnect(t *testing.T) {
	mockClient := &MockClient{connected: true}
	adapter := NewSwitchAdapter(mockClient)

	adapter.Disconnect()

	if mockClient.connected {
		t.Error("Disconnect() did not disconnect the client")
	}
}

func TestSwitchAdapter_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		cmdResponse  string
		cmdError     error
		expectErr    bool
		expectResp   string
		expectExec   bool
	}{
		{
			name:       "successful command",
			cmd:        "show version",
			cmdResponse: "IOS Version 15.1",
			expectErr:  false,
			expectResp: "IOS Version 15.1",
			expectExec: true,
		},
		{
			name:       "command error",
			cmd:        "invalid command",
			cmdError:   &testError{"command failed"},
			expectErr:  true,
			expectResp: "",
			expectExec: true,
		},
		{
			name:       "no response configured",
			cmd:        "show interfaces",
			expectErr:  false,
			expectResp: "mock response",
			expectExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdResponses := make(map[string]string)
			cmdErrors := make(map[string]error)
			
			if tt.cmdResponse != "" {
				cmdResponses[tt.cmd] = tt.cmdResponse
			}
			if tt.cmdError != nil {
				cmdErrors[tt.cmd] = tt.cmdError
			}

			mockClient := &MockClient{
				cmdResponses: cmdResponses,
				cmdErrors:    cmdErrors,
			}
			adapter := NewSwitchAdapter(mockClient)

			resp, err := adapter.ExecuteCommand(tt.cmd)

			if (err != nil) != tt.expectErr {
				t.Errorf("ExecuteCommand() error = %v, expectErr %v", err, tt.expectErr)
			}

			if resp != tt.expectResp {
				t.Errorf("ExecuteCommand() response = %v, expectResp %v", resp, tt.expectResp)
			}

			if tt.expectExec {
				found := false
				for _, executed := range mockClient.executedCmds {
					if executed == tt.cmd {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExecuteCommand() did not execute command %s", tt.cmd)
				}
			}
		})
	}
}

func TestSwitchAdapter_IsConnected(t *testing.T) {
	tests := []struct {
		name     string
		connected bool
	}{
		{
			name:      "connected",
			connected: true,
		},
		{
			name:      "not connected",
			connected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{connected: tt.connected}
			adapter := NewSwitchAdapter(mockClient)

			result := adapter.IsConnected()

			if result != tt.connected {
				t.Errorf("IsConnected() = %v, want %v", result, tt.connected)
			}
		})
	}
}

func TestAuthConfigurable(t *testing.T) {
	// Test that AuthConfigurable interface is properly defined
	var _ AuthConfigurable = &MockAuthConfigurable{}
}

// MockAuthConfigurable implements AuthConfigurable for testing
type MockAuthConfigurable struct {
	prompts []entities.AuthPrompt
}

func (m *MockAuthConfigurable) SetAuthSequence(prompts []entities.AuthPrompt) {
	m.prompts = prompts
}

func (m *MockAuthConfigurable) GetAuthSequence() []entities.AuthPrompt {
	return m.prompts
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}