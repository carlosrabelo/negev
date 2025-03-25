package ports

import "github.com/carlosrabelo/negev/negev/internal/domain/entities"

type SwitchRepository interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
	SetSessionProfile(profile entities.SessionProfile)
}
