package platform

import (
	"fmt"

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
	ClearCache()
}

var drivers []SwitchDriver

func Register(d SwitchDriver) {
	drivers = append(drivers, d)
}

func Get(name string) SwitchDriver {
	for _, d := range drivers {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

func Available() []string {
	names := make([]string, 0, len(drivers))
	for _, d := range drivers {
		names = append(names, d.Name())
	}
	return names
}

func Detect(repo ports.SwitchRepository) (SwitchDriver, error) {
	for _, d := range drivers {
		match, err := d.Detect(repo)
		if err != nil {
			return nil, fmt.Errorf("detection failed for driver %s: %v", d.Name(), err)
		}
		if match {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no matching driver found")
}
