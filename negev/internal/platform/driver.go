package platform

import (
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type SwitchDriver interface {
	Name() string
	Detect(repo ports.SwitchRepository) (bool, error)
	GetAuthenticationSequence() []entities.AuthPrompt
	GetVLANList(repo ports.SwitchRepository) ([]string, error)
	GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error)
	GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error)
	GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error)
	ConfigureAccessCommands(port entities.Port, vlan string) []string
	CreateVLANCommands(vlan string) []string
	DeleteVLANCommands(vlan string) []string
	SaveCommands() []string
}
