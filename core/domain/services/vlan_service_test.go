package services

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
)

type mockSwitchRepo struct {
	connected     bool
	connectErr    error
	connectCalled bool
	executed      []string
	responses     map[string]string
	execErrors    map[string]error
}

func (m *mockSwitchRepo) Connect() error {
	m.connectCalled = true
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockSwitchRepo) Disconnect() {
	m.connected = false
}

func (m *mockSwitchRepo) ExecuteCommand(cmd string) (string, error) {
	m.executed = append(m.executed, cmd)
	if m.execErrors != nil {
		if err, ok := m.execErrors[cmd]; ok {
			return "", err
		}
	}
	if m.responses != nil {
		if resp, ok := m.responses[cmd]; ok {
			return resp, nil
		}
	}
	return "", nil
}

func (m *mockSwitchRepo) IsConnected() bool {
	return m.connected
}

type mockDriver struct {
	name              string
	detectResult      bool
	detectErr         error
	vlanList          map[string]bool
	trunks            map[string]bool
	activePorts       []entities.Port
	macTable          []entities.Device
	configureCommands map[string][]string
	createCommands    map[string][]string
	deleteCommands    map[string][]string
	saveCommands      []string
}

func (m *mockDriver) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockDriver) Detect(repo ports.SwitchRepository) (bool, error) {
	if m.detectErr != nil {
		return false, m.detectErr
	}
	return m.detectResult, nil
}

func (m *mockDriver) GetVLANList(ports.SwitchRepository, entities.SwitchConfig) (map[string]bool, error) {
	return cloneBoolMap(m.vlanList), nil
}

func (m *mockDriver) GetTrunkInterfaces(ports.SwitchRepository, entities.SwitchConfig) (map[string]bool, error) {
	return cloneBoolMap(m.trunks), nil
}

func (m *mockDriver) GetActivePorts(ports.SwitchRepository, entities.SwitchConfig) ([]entities.Port, error) {
	return clonePorts(m.activePorts), nil
}

func (m *mockDriver) GetMacTable(ports.SwitchRepository, entities.SwitchConfig) ([]entities.Device, error) {
	return cloneDevices(m.macTable), nil
}

func (m *mockDriver) ConfigureAccessCommands(iface, vlan string) []string {
	if m.configureCommands == nil {
		return nil
	}
	key := fmt.Sprintf("%s|%s", iface, vlan)
	return cloneStrings(m.configureCommands[key])
}

func (m *mockDriver) CreateVLANCommands(vlan string) []string {
	if m.createCommands == nil {
		return nil
	}
	return cloneStrings(m.createCommands[vlan])
}

func (m *mockDriver) DeleteVLANCommands(vlan string) []string {
	if m.deleteCommands == nil {
		return nil
	}
	return cloneStrings(m.deleteCommands[vlan])
}

func (m *mockDriver) SaveCommands() []string {
	return []string{"write memory"}
}

func (m *mockDriver) GetAuthenticationSequence(username, password, enablePassword string) []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: username},
		{WaitFor: "Password:", SendCmd: password},
		{WaitFor: ">", SendCmd: "enable"},
		{WaitFor: "Password:", SendCmd: enablePassword},
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return map[string]bool{}
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePorts(in []entities.Port) []entities.Port {
	if in == nil {
		return nil
	}
	out := make([]entities.Port, len(in))
	copy(out, in)
	return out
}

func cloneDevices(in []entities.Device) []entities.Device {
	if in == nil {
		return nil
	}
	out := make([]entities.Device, len(in))
	copy(out, in)
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func TestProcessPortsAppliesVLANChangeAndSaves(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList:    map[string]bool{"10": true, "20": true},
		trunks:      map[string]bool{},
		activePorts: []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}},
		macTable: []entities.Device{
			{Vlan: "10", Mac: "001122", MacFull: "00:11:22:33:44:55", Interface: "Gi1/0/1"},
		},
		configureCommands: map[string][]string{
			"Gi1/0/1|20": {"conf t", "interface Gi1/0/1", "switchport access vlan 20"},
		},
		saveCommands: []string{"write memory"},
	}
	config := entities.SwitchConfig{
		Target:      "switch-01",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"001122": "20"},
		Sandbox:     false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	expected := []string{
		"conf t",
		"interface Gi1/0/1",
		"switchport access vlan 20",
		"write memory",
	}
	if !reflect.DeepEqual(repo.executed, expected) {
		t.Fatalf("unexpected commands executed.\nwant: %v\n got: %v", expected, repo.executed)
	}
	if !repo.connectCalled {
		t.Fatalf("expected Connect() to be called")
	}
}

func TestProcessPortsSandboxSkipsExecution(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList:    map[string]bool{"10": true, "20": true},
		trunks:      map[string]bool{},
		activePorts: []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}},
		macTable: []entities.Device{
			{Vlan: "10", Mac: "001122", MacFull: "00:11:22:33:44:55", Interface: "Gi1/0/1"},
		},
		configureCommands: map[string][]string{
			"Gi1/0/1|20": {"conf t", "interface Gi1/0/1", "switchport access vlan 20"},
		},
		saveCommands: []string{"write memory"},
	}
	config := entities.SwitchConfig{
		Target:      "switch-sandbox",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"001122": "20"},
		Sandbox:     true,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	if len(repo.executed) != 0 {
		t.Fatalf("expected no commands to execute in sandbox mode, got %v", repo.executed)
	}
}

func TestProcessPortsSkipsExcludedAndTrunkPorts(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList: map[string]bool{"10": true, "20": true},
		trunks:   map[string]bool{"gi1/0/2": true},
		activePorts: []entities.Port{
			{Interface: "Gi1/0/1", Vlan: "10"},
			{Interface: "Gi1/0/2", Vlan: "10"},
		},
		macTable: []entities.Device{
			{Vlan: "10", Mac: "00aa11", MacFull: "00:aa:11:22:33:44", Interface: "Gi1/0/1"},
			{Vlan: "10", Mac: "00bb22", MacFull: "00:bb:22:33:44:55", Interface: "Gi1/0/2"},
		},
		configureCommands: map[string][]string{
			"Gi1/0/1|20": {"conf t", "interface Gi1/0/1", "switchport access vlan 20"},
			"Gi1/0/2|20": {"conf t", "interface Gi1/0/2", "switchport access vlan 20"},
		},
	}
	config := entities.SwitchConfig{
		Target:       "switch-exclude-trunk",
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"00aa11": "20", "00bb22": "20"},
		ExcludePorts: []string{"Gi1/0/1"},
		Sandbox:      false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	if len(repo.executed) != 0 {
		t.Fatalf("expected no commands because both ports are skipped, got %v", repo.executed)
	}
}

func TestProcessPortsIgnoresPortsWithMultipleMacs(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList: map[string]bool{"10": true, "20": true},
		activePorts: []entities.Port{
			{Interface: "Gi1/0/1", Vlan: "10"},
		},
		macTable: []entities.Device{
			{Vlan: "10", Mac: "00aa11", MacFull: "00:aa:11:22:33:44", Interface: "Gi1/0/1"},
			{Vlan: "10", Mac: "00bb22", MacFull: "00:bb:22:33:44:55", Interface: "Gi1/0/1"},
		},
		configureCommands: map[string][]string{
			"Gi1/0/1|20": {"conf t", "interface Gi1/0/1", "switchport access vlan 20"},
		},
	}
	config := entities.SwitchConfig{
		Target:      "switch-multiple-macs",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"00aa11": "20"},
		Sandbox:     false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	if len(repo.executed) != 0 {
		t.Fatalf("expected no commands when multiple MACs appear on a port, got %v", repo.executed)
	}
}

func TestProcessPortsSkipsWhenTargetVLANNeverCreated(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList: map[string]bool{"10": true},
		activePorts: []entities.Port{
			{Interface: "Gi1/0/1", Vlan: "10"},
		},
		macTable: []entities.Device{
			{Vlan: "10", Mac: "00aa11", MacFull: "00:aa:11:22:33:44", Interface: "Gi1/0/1"},
		},
		configureCommands: map[string][]string{
			"Gi1/0/1|20": {"conf t", "interface Gi1/0/1", "switchport access vlan 20"},
		},
	}
	config := entities.SwitchConfig{
		Target:      "switch-missing-vlan",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"00aa11": "20"},
		Sandbox:     false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	if len(repo.executed) != 0 {
		t.Fatalf("expected no commands because target VLAN does not exist, got %v", repo.executed)
	}
}

func TestProcessPortsDeletesDisallowedVLANs(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList: map[string]bool{"10": true, "30": true, "1001": true},
		deleteCommands: map[string][]string{
			"30": {"no vlan 30"},
		},
	}
	config := entities.SwitchConfig{
		Target:       "switch-delete",
		DefaultVlan:  "10",
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		ProtectedVlans: []string{
			"1001",
		},
		Sandbox: false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	expected := []string{"no vlan 30"}
	if !reflect.DeepEqual(repo.executed, expected) {
		t.Fatalf("unexpected commands executed.\nwant: %v\n got: %v", expected, repo.executed)
	}
}

func TestProcessPortsCreatesMissingVLANs(t *testing.T) {
	repo := &mockSwitchRepo{}
	driver := &mockDriver{
		vlanList: map[string]bool{"10": true},
		createCommands: map[string][]string{
			"20": {"vlan 20", "name USERS"},
		},
	}
	config := entities.SwitchConfig{
		Target:       "switch-create",
		DefaultVlan:  "10",
		CreateVLANs:  true,
		AllowedVlans: []string{"10", "20"},
		Sandbox:      false,
	}

	service := NewVLANService(repo, config, driver)
	if err := service.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	expected := []string{"vlan 20", "name USERS"}
	if !reflect.DeepEqual(repo.executed, expected) {
		t.Fatalf("unexpected commands executed.\nwant: %v\n got: %v", expected, repo.executed)
	}
	if !repo.connectCalled {
		t.Fatalf("expected Connect() to be called")
	}
}
