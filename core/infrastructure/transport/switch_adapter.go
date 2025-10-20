package transport

import (
	"github.com/carlosrabelo/negev/core/domain/entities"
)

// SwitchAdapter implements the SwitchRepository port using existing infrastructure
type SwitchAdapter struct {
	client Client
}

// NewSwitchAdapter creates a new switch adapter
func NewSwitchAdapter(client Client) *SwitchAdapter {
	return &SwitchAdapter{
		client: client,
	}
}

// Connect connects to the switch
func (s *SwitchAdapter) Connect() error {
	return s.client.Connect()
}

// Disconnect disconnects from the switch
func (s *SwitchAdapter) Disconnect() {
	s.client.Disconnect()
}

// ExecuteCommand executes a command on the switch
func (s *SwitchAdapter) ExecuteCommand(cmd string) (string, error) {
	return s.client.ExecuteCommand(cmd)
}

// IsConnected checks if connected
func (s *SwitchAdapter) IsConnected() bool {
	return s.client.IsConnected()
}

// Client interface that already exists in the transport package
type Client interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}

// AuthConfigurable allows setting authentication prompts after client creation
type AuthConfigurable interface {
	SetAuthSequence(prompts []entities.AuthPrompt)
}
