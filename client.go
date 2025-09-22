package main

type SwitchClient interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
}
