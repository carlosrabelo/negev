package ports

type SwitchRepository interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}
