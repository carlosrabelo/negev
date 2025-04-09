package platform

import (
	"fmt"
	"sort"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type SwitchDriver interface {
	Name() string
	Detect(repo ports.SwitchRepository) (bool, error)
	GetSessionProfile() entities.SessionProfile
	GetAuthenticationSequence() []entities.AuthPrompt
	GetVLANList(repo ports.SwitchRepository) ([]string, error)
	GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error)
	GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error)
	GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error)
	GetL3Interfaces(repo ports.SwitchRepository) ([]string, error)
	GetRunningConfig(repo ports.SwitchRepository) (string, error)
	NormalizeInterface(name string) string
	ConfigureAccessCommands(port entities.Port, vlan string) []string
	CreateVLANCommands(vlan string) []string
	DeleteVLANCommands(vlan string) []string
	SaveCommands() []string
	ClearCache()
	IsCommandError(output string) bool
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
	sort.Strings(names)
	return names
}

func sortedDrivers() []SwitchDriver {
	out := make([]SwitchDriver, len(drivers))
	copy(out, drivers)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

func Detect(repo ports.SwitchRepository) (SwitchDriver, error) {
	for _, d := range sortedDrivers() {
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
