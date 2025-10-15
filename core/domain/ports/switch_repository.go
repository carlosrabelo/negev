package ports

// SwitchRepository defines the port for network switch interaction
type SwitchRepository interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}
