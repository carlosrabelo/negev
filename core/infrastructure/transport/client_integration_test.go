package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

func TestNewSSHClient(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewSSHClient(config)
	if client == nil {
		t.Fatal("NewSSHClient() returned nil")
	}

	// Verify it implements the Client interface
	var _ Client = client
}

func TestNewTelnetClient(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewTelnetClient(config)
	if client == nil {
		t.Fatal("NewTelnetClient() returned nil")
	}

	// Verify it implements the Client interface
	var _ Client = client

	// Verify it implements AuthConfigurable
	var _ AuthConfigurable = client
}

func TestTelnetClient_SetAuthSequence(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewTelnetClient(config)
	
	// Type assertion to TelnetClient to access SetAuthSequence method
	telnetClient := client
	if telnetClient == nil {
		t.Fatal("NewTelnetClient() returned nil")
	}

	prompts := []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "admin"},
		{WaitFor: "Password:", SendCmd: "password"},
		{WaitFor: ">", SendCmd: "enable"},
		{WaitFor: "Password:", SendCmd: "enable"},
	}

	// Call SetAuthSequence method directly
	telnetClient.SetAuthSequence(prompts)

	// We can't directly access the authSequence field as it's private
	// but we can verify the method doesn't panic
}

func TestSSHClient_Disconnect(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewSSHClient(config)
	
	// Test that Disconnect doesn't panic
	client.Disconnect()
}

func TestTelnetClient_Disconnect(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewTelnetClient(config)
	
	// Test that Disconnect doesn't panic
	client.Disconnect()
}

func TestSSHClient_IsConnected(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewSSHClient(config)
	
	// Initially should not be connected
	if client.IsConnected() {
		t.Error("New SSH client should not be connected initially")
	}
}

func TestTelnetClient_IsConnected(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := NewTelnetClient(config)
	
	// Initially should not be connected
	if client.IsConnected() {
		t.Error("New Telnet client should not be connected initially")
	}
}