package dmswitch

import (
	"strings"

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
	return "dmswitch"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "dm1200") || strings.Contains(lower, "1200e") {
		return false, nil
	}
	if strings.Contains(lower, "dmswitch") {
		return true, nil
	}
	if strings.Contains(lower, "datacom") && (strings.Contains(lower, "3324") || strings.Contains(lower, "3224")) {
		return true, nil
	}
	return false, nil
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
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

func (d *Driver) ClearCache() {
	clearSwitchportCache()
}

func (d *Driver) IsCommandError(output string) bool {
	return false
}
