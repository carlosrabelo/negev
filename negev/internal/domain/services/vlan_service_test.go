package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type mockRepository struct {
	connected    bool
	executed     []string
	connectErr   error
	commandErr   error
	commandOut   string
	failOnCmd    string
	commandErrBy map[string]error
}

func (m *mockRepository) Connect() error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockRepository) Disconnect() {
	m.connected = false
}

func (m *mockRepository) ExecuteCommand(cmd string) (string, error) {
	m.executed = append(m.executed, cmd)
	if m.failOnCmd != "" && cmd == m.failOnCmd {
		return m.commandOut, errors.New("command failed")
	}
	if m.commandErrBy != nil {
		if err, ok := m.commandErrBy[cmd]; ok {
			return m.commandOut, err
		}
	}
	if m.commandErr != nil {
		return m.commandOut, m.commandErr
	}
	return m.commandOut, nil
}

func (m *mockRepository) IsConnected() bool {
	return m.connected
}

type stubDriver struct {
	vlans           []string
	trunks          []string
	ports           []entities.Port
	devices         []entities.Device
	vlanListErr     error
	trunkErr        error
	portsErr        error
	macErr          error
	commandErrorOut bool
	createCmds      []string
	deleteCmds      []string
	configurePrefix string
	cleared         bool
	saveCmds        []string
}

func (d *stubDriver) Name() string { return "stub" }
func (d *stubDriver) Detect(repo ports.SwitchRepository) (bool, error) {
	return true, nil
}
func (d *stubDriver) GetAuthenticationSequence() []entities.AuthPrompt { return nil }
func (d *stubDriver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	if d.vlanListErr != nil {
		return nil, d.vlanListErr
	}
	return d.vlans, nil
}
func (d *stubDriver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	if d.trunkErr != nil {
		return nil, d.trunkErr
	}
	return d.trunks, nil
}
func (d *stubDriver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	if d.portsErr != nil {
		return nil, d.portsErr
	}
	return d.ports, nil
}
func (d *stubDriver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	if d.macErr != nil {
		return nil, d.macErr
	}
	return d.devices, nil
}
func (d *stubDriver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	prefix := d.configurePrefix
	if prefix == "" {
		prefix = "switchport access vlan "
	}
	return []string{prefix + vlan}
}
func (d *stubDriver) CreateVLANCommands(vlan string) []string {
	if d.createCmds != nil {
		return append([]string{}, d.createCmds...)
	}
	return []string{"vlan " + vlan}
}
func (d *stubDriver) DeleteVLANCommands(vlan string) []string {
	if d.deleteCmds != nil {
		return append([]string{}, d.deleteCmds...)
	}
	return []string{"no vlan " + vlan}
}
func (d *stubDriver) SaveCommands() []string {
	if d.saveCmds != nil {
		return append([]string{}, d.saveCmds...)
	}
	return []string{"write memory"}
}
func (d *stubDriver) ClearCache() { d.cleared = true }
func (d *stubDriver) IsCommandError(output string) bool {
	return d.commandErrorOut && output != ""
}

func baseDriver() *stubDriver {
	return &stubDriver{
		vlans: []string{"1", "10"},
		ports: []entities.Port{{Interface: "Gi1/0/1", Vlan: "1"}},
		devices: []entities.Device{
			{Vlan: "1", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "Gi1/0/1"},
		},
	}
}


func TestProcessPortsConnectFailure(t *testing.T) {
	repo := &mockRepository{connectErr: errors.New("dial refused")}
	svc := NewVLANService(repo, entities.SwitchConfig{}, baseDriver())
	if err := svc.ProcessPorts(); err == nil {
		t.Fatal("expected connect failure")
	}
}
