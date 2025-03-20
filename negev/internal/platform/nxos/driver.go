package nxos

import (
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "nxos"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	return false, nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return nil
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	return nil, nil
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	return nil, nil
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

func (d *Driver) ClearCache() {}

func (d *Driver) IsCommandError(output string) bool {
	return false
}
