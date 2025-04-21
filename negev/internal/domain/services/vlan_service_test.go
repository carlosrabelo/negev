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

func TestProcessPortsSaveConfiguration(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}

	svc := NewVLANService(repo, cfg, drv)
	if err := svc.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	saved := false
	for _, cmd := range repo.executed {
		if cmd == "write memory" {
			saved = true
		}
	}
	if !saved {
		t.Error("expected configuration to be saved via 'write memory', but it was not")
	}

	repoSandbox := &mockRepository{}
	cfgSandbox := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	svcSandbox := NewVLANService(repoSandbox, cfgSandbox, drv)
	if err := svcSandbox.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repoSandbox.executed {
		if cmd == "write memory" {
			t.Error("expected configuration NOT to be saved in sandbox mode, but it was")
		}
	}
}

func TestProcessPortsConnectFailure(t *testing.T) {
	repo := &mockRepository{connectErr: errors.New("dial refused")}
	svc := NewVLANService(repo, entities.SwitchConfig{}, baseDriver())
	if err := svc.ProcessPorts(); err == nil {
		t.Fatal("expected connect failure")
	}
}

func TestProcessPortsNoChanges(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	if len(repo.executed) != 0 {
		t.Errorf("expected no commands in sandbox with matching VLAN, got %v", repo.executed)
	}
}

func TestProcessPortsSkipsTrunkExcludedAndMultiMAC(t *testing.T) {
	repo := &mockRepository{}
	drv := &stubDriver{
		vlans:  []string{"1", "10"},
		trunks: []string{"Gi1/0/24"},
		ports: []entities.Port{
			{Interface: "Gi1/0/24", Vlan: "1"},
			{Interface: "Gi1/0/2", Vlan: "1"},
			{Interface: "Gi1/0/3", Vlan: "1"},
			{Interface: "Gi1/0/4", Vlan: "1"},
			{Interface: "Gi1/0/5", Vlan: "1"},
		},
		devices: []entities.Device{
			{Mac: "aabbccddeeff", Interface: "Gi1/0/3"},
			{Mac: "aabbccddeeff", Interface: "Gi1/0/4"},
			{Mac: "112233445566", Interface: "Gi1/0/4"},
			{Mac: "abcd", Interface: "Gi1/0/5"},
			{Mac: "deadbeef0001", Interface: "Gi1/0/5"},
		},
	}
	cfg := entities.SwitchConfig{
		Sandbox:      true,
		DefaultVlan:  "10",
		ExcludePorts: []string{"Gi1/0/2"},
		ExcludeMacs:  []string{"aabbccddeeff"},
		MacToVlan:    map[string]string{"deadbe": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
}

func TestProcessPortsDefaultVlanAndMissingVlan(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.devices = []entities.Device{
		{Mac: "ffffffffffff", Interface: "Gi1/0/1"},
	}
	cfg := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"ffffff": "0"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	drv2 := baseDriver()
	drv2.vlans = []string{"1"}
	cfg2 := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "99",
		MacToVlan:   map[string]string{"aabbcc": "99"},
	}
	if err := NewVLANService(repo, cfg2, drv2).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
}

func TestProcessPortsCreateAndDeleteVLANs(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "20", "1005"}
	cfg := entities.SwitchConfig{
		Sandbox:        false,
		CreateVLANs:    true,
		AllowedVlans:   []string{"1", "10"},
		ProtectedVlans: []string{"20"},
		DefaultVlan:    "10",
		MacToVlan:      map[string]string{"aabbcc": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	created, deletedProtected, deletedExtra := false, false, false
	for _, cmd := range repo.executed {
		if cmd == "vlan 10" {
			created = true
		}
		if cmd == "no vlan 20" {
			deletedProtected = true
		}
		if cmd == "no vlan 1005" {
			deletedExtra = true
		}
	}
	if !created {
		t.Error("expected VLAN 10 to be created")
	}
	if deletedProtected {
		t.Error("protected VLAN 20 must not be deleted")
	}
	if deletedExtra {
		t.Error("range-protected VLAN 1005 must not be deleted")
	}
}

func TestProcessPortsCreateVLANSandbox(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1"}
	cfg := entities.SwitchConfig{
		Sandbox:      true,
		CreateVLANs:  true,
		AllowedVlans: []string{"1", "10"},
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repo.executed {
		if cmd == "vlan 10" || cmd == "write memory" {
			t.Errorf("sandbox should not execute write commands, got %q", cmd)
		}
	}
}

func TestConfigureVlanWriteAndCommandError(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	svc := NewVLANService(repo, entities.SwitchConfig{Sandbox: false}, drv)
	if err := svc.ConfigureVlan("Gi1/0/1", "10"); err != nil {
		t.Fatalf("ConfigureVlan failed: %v", err)
	}
	if len(repo.executed) != 1 || repo.executed[0] != "switchport access vlan 10" {
		t.Fatalf("unexpected commands: %v", repo.executed)
	}

	repoErr := &mockRepository{commandErr: errors.New("boom")}
	if err := NewVLANService(repoErr, entities.SwitchConfig{Sandbox: false}, drv).ConfigureVlan("Gi1/0/1", "10"); err == nil {
		t.Fatal("expected command error")
	}

	drvErr := baseDriver()
	drvErr.commandErrorOut = true
	repoOut := &mockRepository{commandOut: "Invalid input"}
	if err := NewVLANService(repoOut, entities.SwitchConfig{Sandbox: false}, drvErr).ConfigureVlan("Gi1/0/1", "10"); err == nil {
		t.Fatal("expected IsCommandError path")
	}
}

func TestCreateAndDeleteVLANWritePaths(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	svc := NewVLANService(repo, entities.SwitchConfig{Sandbox: false}, drv)
	if err := svc.CreateVLAN("30"); err != nil {
		t.Fatalf("CreateVLAN failed: %v", err)
	}
	if err := svc.DeleteVLAN("30"); err != nil {
		t.Fatalf("DeleteVLAN failed: %v", err)
	}

	repoFail := &mockRepository{commandErr: errors.New("fail")}
	if err := NewVLANService(repoFail, entities.SwitchConfig{Sandbox: false}, drv).CreateVLAN("30"); err == nil {
		t.Fatal("expected CreateVLAN error")
	}
	if err := NewVLANService(repoFail, entities.SwitchConfig{Sandbox: false}, drv).DeleteVLAN("30"); err == nil {
		t.Fatal("expected DeleteVLAN error")
	}

	drvErr := baseDriver()
	drvErr.commandErrorOut = true
	repoOut := &mockRepository{commandOut: "% Error"}
	if err := NewVLANService(repoOut, entities.SwitchConfig{Sandbox: false}, drvErr).CreateVLAN("30"); err == nil {
		t.Fatal("expected CreateVLAN IsCommandError")
	}
	if err := NewVLANService(repoOut, entities.SwitchConfig{Sandbox: false}, drvErr).DeleteVLAN("30"); err == nil {
		t.Fatal("expected DeleteVLAN IsCommandError")
	}
}

func TestProcessPortsPropagatesDriverErrors(t *testing.T) {
	cases := []struct {
		name string
		drv  *stubDriver
	}{
		{"vlan list", &stubDriver{vlanListErr: fmt.Errorf("vlan err")}},
		{"trunks", &stubDriver{vlans: []string{"1"}, trunkErr: fmt.Errorf("trunk err")}},
		{"ports", &stubDriver{vlans: []string{"1"}, portsErr: fmt.Errorf("ports err")}},
		{"macs", &stubDriver{vlans: []string{"1"}, ports: []entities.Port{{Interface: "Gi1/0/1"}}, macErr: fmt.Errorf("mac err")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewVLANService(&mockRepository{}, entities.SwitchConfig{}, tc.drv).ProcessPorts()
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestProcessPortsCreateVLANFailure(t *testing.T) {
	repo := &mockRepository{commandErr: errors.New("cannot create")}
	drv := baseDriver()
	drv.vlans = []string{"1"}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"1", "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected create VLAN failure")
	}
}

func TestProcessPortsDeleteVLANFailure(t *testing.T) {
	repo := &mockRepository{failOnCmd: "no vlan 99"}
	drv := baseDriver()
	drv.vlans = []string{"1", "99"}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"1"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected delete VLAN failure")
	}
}

func TestProcessPortsConfigureFailure(t *testing.T) {
	repo := &mockRepository{failOnCmd: "switchport access vlan 10"}
	drv := baseDriver()
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected configure failure")
	}
}

func TestGettersAndHelpers(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.trunks = []string{"Gi1/0/24"}
	svc := NewVLANService(repo, entities.SwitchConfig{
		AllowedVlans:   []string{"10", "20"},
		ProtectedVlans: []string{"30"},
		ExcludeMacs:    []string{"AABBCCDDEEFF"},
		ExcludePorts:   []string{"Gi1/0/9"},
	}, drv)

	vlans, err := svc.GetVlanList()
	if err != nil || !vlans["10"] {
		t.Fatalf("GetVlanList = %v, %v", vlans, err)
	}
	trunks, err := svc.GetTrunkInterfaces()
	if err != nil || !trunks["Gi1/0/24"] {
		t.Fatalf("GetTrunkInterfaces = %v, %v", trunks, err)
	}
	ports, err := svc.GetActivePorts()
	if err != nil || len(ports) != 1 {
		t.Fatalf("GetActivePorts = %v, %v", ports, err)
	}
	macs, err := svc.GetMacTable()
	if err != nil || len(macs) != 1 {
		t.Fatalf("GetMacTable = %v, %v", macs, err)
	}

	if !svc.isExcluded("aabbccddeeff") {
		t.Error("expected MAC exclusion (case-insensitive)")
	}
	if !svc.isExcludedPort("gi1/0/9") {
		t.Error("expected port exclusion (case-insensitive)")
	}
	if !svc.isProtected("1000") || !svc.isProtected("30") || svc.isProtected("40") {
		t.Error("unexpected isProtected results")
	}
	filtered := svc.filterDevices(macs, "Gi1/0/1")
	if len(filtered) != 1 {
		t.Fatalf("filterDevices = %v", filtered)
	}
	keys := sortedKeys(map[string]bool{"b": true, "a": true})
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("sortedKeys = %v", keys)
	}
}

func TestSaveConfigurationError(t *testing.T) {
	repo := &mockRepository{commandErr: errors.New("save failed")}
	drv := baseDriver()
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected save configuration failure")
	}
}
