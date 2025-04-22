package dmos

import (
	"fmt"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

type Driver struct{}

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dmos"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return nil
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return nil
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return nil
}

func (d *Driver) SaveCommands() []string {
	return nil
}
