package client

// SwitchClient defines the interface for connecting to and executing commands on network switches
type SwitchClient interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
}
