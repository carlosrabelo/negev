package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type mockRepository struct {
	connected     bool
	executed      []string
	connectErr    error
	commandErr    error
	commandOut    string
	failOnCmd     string
	commandErrBy  map[string]error
	sessionSet    bool
	sessionProfile entities.SessionProfile
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

func (m *mockRepository) SetSessionProfile(profile entities.SessionProfile) {
	m.sessionSet = true
	m.sessionProfile = profile
}

type stubDriver struct {
	vlans           []string
	trunks          []string
	ports           []entities.Port
	devices         []entities.Device
	l3Vlans         []string
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
func (d *stubDriver) GetSessionProfile() entities.SessionProfile { return entities.SessionProfile{} }
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
func (d *stubDriver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	return d.l3Vlans, nil
}
func (d *stubDriver) NormalizeInterface(name string) string { return name }
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

func TestProcessPortsSaveConfiguration(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}

	svc := NewVLANService(repo, cfg, drv)
	if _, err := svc.ProcessPorts(); err != nil {
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
	if _, err := svcSandbox.ProcessPorts(); err != nil {
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
	if _, err := svc.ProcessPorts(); err == nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	drv2 := baseDriver()
	drv2.vlans = []string{"1"}
	cfg2 := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "99",
		MacToVlan:   map[string]string{"aabbcc": "99"},
	}
	if _, err := NewVLANService(repo, cfg2, drv2).ProcessPorts(); err != nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
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
			_, err := NewVLANService(&mockRepository{}, entities.SwitchConfig{}, tc.drv).ProcessPorts()
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
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
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
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
	if err != nil || !trunks["gi1/0/24"] {
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
	if !svc.isProtected("1") || !svc.isProtected("1000") || !svc.isProtected("30") || svc.isProtected("40") {
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
	repo := &mockRepository{failOnCmd: "write memory"}
	drv := baseDriver()
	drv.saveCmds = []string{"write memory"}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected save configuration failure")
	}
}

func TestSaveConfigurationCommandError(t *testing.T) {
	drv := baseDriver()
	drv.saveCmds = []string{"write memory"}
	repo := &mockRepository{failOnCmd: "write memory"}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected save to fail when the CLI reports an error")
	}
}

func TestSaveConfigurationFallsBackToNextCommand(t *testing.T) {
	drv := baseDriver()
	drv.saveCmds = []string{"copy running-config startup-config", "save"}
	repo := &mockRepository{failOnCmd: "copy running-config startup-config"}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("expected fallback save to succeed: %v", err)
	}
	saved := false
	for _, cmd := range repo.executed {
		if cmd == "save" {
			saved = true
		}
	}
	if !saved {
		t.Fatalf("expected fallback save command, got %v", repo.executed)
	}
}

func TestProcessPortsAssignsNoDataVlan(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.devices = nil
	drv.vlans = []string{"1", "10", "999"}
	cfg := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		NoDataVlan:  "999",
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	if len(repo.executed) != 0 {
		t.Fatalf("sandbox should not execute commands, got %v", repo.executed)
	}

	repoWrite := &mockRepository{}
	cfg.Sandbox = false
	if _, err := NewVLANService(repoWrite, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts write failed: %v", err)
	}
	configured := false
	for _, cmd := range repoWrite.executed {
		if cmd == "switchport access vlan 999" {
			configured = true
		}
	}
	if !configured {
		t.Fatalf("expected no_data_vlan assignment, got %v", repoWrite.executed)
	}
}

func TestProcessPortsSandboxVLANSyncSummary(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:      true,
		CreateVLANs:  true,
		AllowedVlans: []string{"1", "10"},
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	if len(repo.executed) != 0 {
		t.Fatalf("sandbox must not execute commands, got %v", repo.executed)
	}
}

func TestProcessPortsKeepsDefaultAndNoDataVLANs(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "20", "999"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		DefaultVlan:  "1",
		NoDataVlan:   "999",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repo.executed {
		if cmd == "no vlan 1" || cmd == "no vlan 999" {
			t.Fatalf("must not delete default/no_data VLANs, got %v", repo.executed)
		}
	}
}

func TestProcessPortsCreateVLANsRequiresAllowed(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "20"}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		CreateVLANs: true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err == nil {
		t.Fatal("expected error when --create-vlans has empty allowed_vlans")
	}
	for _, cmd := range repo.executed {
		if cmd == "no vlan 20" || cmd == "no vlan 1" {
			t.Fatalf("must not delete VLANs when allowed_vlans is empty, got %v", repo.executed)
		}
	}
}

func TestProcessPortsSkipsEmptyPortWithoutNoDataVlan(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.devices = nil
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	drv.vlans = []string{"1", "10", "999"}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "1",
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repo.executed {
		if len(cmd) >= 22 && cmd[:22] == "switchport access vlan" {
			t.Fatalf("empty port must stay unchanged without no_data_vlan, got %v", repo.executed)
		}
	}
}

func TestProcessPortsSyncDoesNotInventNoDataVlan(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "20", "999"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		DefaultVlan:  "1",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	created999 := false
	deleted999 := false
	for _, cmd := range repo.executed {
		if cmd == "vlan 999" {
			created999 = true
		}
		if cmd == "no vlan 999" {
			deleted999 = true
		}
	}
	if created999 {
		t.Fatalf("must not create no_data_vlan when unset, got %v", repo.executed)
	}
	if !deleted999 {
		t.Fatalf("must not protect 999 when no_data_vlan is unset, got %v", repo.executed)
	}
}

func TestProcessPortsSetsSessionProfile(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	cfg := entities.SwitchConfig{Sandbox: true, DefaultVlan: "10"}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	if !repo.sessionSet {
		t.Fatal("expected SetSessionProfile before Connect")
	}
}

func TestProcessPortsSkipsRoutedPort(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.ports = []entities.Port{
		{Interface: "Gi1/0/1", Vlan: "routed", IsRouted: true},
		{Interface: "Gi1/0/2", Vlan: "1"},
	}
	drv.devices = []entities.Device{
		{Interface: "Gi1/0/1", Mac: "aabbccddeeff", Vlan: "1"},
		{Interface: "Gi1/0/2", Mac: "aabbccddeeff", Vlan: "1"},
	}
	cfg := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	result, err := NewVLANService(repo, cfg, drv).ProcessPorts()
	if err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	if result.PortsChanged != 1 {
		t.Fatalf("PortsChanged = %d; expected 1 (routed port skipped)", result.PortsChanged)
	}
}

func TestProcessPortsNeverDeletesVLAN1(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "20"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repo.executed {
		if cmd == "no vlan 1" {
			t.Fatalf("VLAN 1 must never be deleted, got %v", repo.executed)
		}
	}
}

func TestProcessPortsProtectsL3SVI(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "50", "20"}
	drv.l3Vlans = []string{"50"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	for _, cmd := range repo.executed {
		if cmd == "no vlan 50" {
			t.Fatalf("L3 SVI VLAN must not be deleted, got %v", repo.executed)
		}
	}
}

func TestProcessPortsDeletesAfterPortMigrate(t *testing.T) {
	repo := &mockRepository{}
	drv := baseDriver()
	drv.vlans = []string{"1", "10", "20"}
	drv.ports = []entities.Port{{Interface: "Gi1/0/1", Vlan: "20"}}
	drv.devices = []entities.Device{
		{Interface: "Gi1/0/1", Mac: "aabbccddeeff", Vlan: "20"},
	}
	cfg := entities.SwitchConfig{
		Sandbox:      false,
		CreateVLANs:  true,
		AllowedVlans: []string{"10"},
		DefaultVlan:  "10",
		MacToVlan:    map[string]string{"aabbcc": "10"},
	}
	if _, err := NewVLANService(repo, cfg, drv).ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}
	configureIdx, deleteIdx := -1, -1
	for i, cmd := range repo.executed {
		if cmd == "switchport access vlan 10" {
			configureIdx = i
		}
		if cmd == "no vlan 20" {
			deleteIdx = i
		}
	}
	if configureIdx < 0 || deleteIdx < 0 {
		t.Fatalf("expected configure and delete, got %v", repo.executed)
	}
	if configureIdx > deleteIdx {
		t.Fatalf("port migrate must happen before VLAN delete, got %v", repo.executed)
	}
}

func TestProcessPortsPartialPortFailureStillSaves(t *testing.T) {
	repo := &mockRepository{failOnCmd: "switchport access vlan 20"}
	drv := baseDriver()
	drv.vlans = []string{"1", "10", "20"}
	drv.ports = []entities.Port{
		{Interface: "Gi1/0/1", Vlan: "1"},
		{Interface: "Gi1/0/2", Vlan: "1"},
	}
	drv.devices = []entities.Device{
		{Interface: "Gi1/0/1", Mac: "aabbccddeeff", Vlan: "1"},
		{Interface: "Gi1/0/2", Mac: "112233445566", Vlan: "1"},
	}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan: map[string]string{
			"aabbcc": "10",
			"112233": "20",
		},
	}
	result, err := NewVLANService(repo, cfg, drv).ProcessPorts()
	if err == nil {
		t.Fatal("expected aggregated port failure")
	}
	if result.PortsChanged != 1 || result.PortsFailed != 1 {
		t.Fatalf("PortsChanged=%d PortsFailed=%d; expected 1 and 1", result.PortsChanged, result.PortsFailed)
	}
	saved := false
	for _, cmd := range repo.executed {
		if cmd == "write memory" {
			saved = true
		}
	}
	if !saved {
		t.Fatalf("successful changes must be saved despite partial failure, got %v", repo.executed)
	}
}

func TestLongestPrefixMatchExactOverOUI(t *testing.T) {
	mapping := map[string]string{
		"0004f2":       "50",
		"0004f21a2b3c": "55",
	}
	if got := resolveMacToVlan("0004f21a2b3c", mapping, "1"); got != "55" {
		t.Fatalf("exact MAC = %s; want 55", got)
	}
	if got := resolveMacToVlan("0004f2aabbcc", mapping, "1"); got != "50" {
		t.Fatalf("OUI fallback = %s; want 50", got)
	}

	repo := &mockRepository{}
	drv := &stubDriver{
		vlans:   []string{"1", "50", "55"},
		ports:   []entities.Port{{Interface: "Gi1/0/1", Vlan: "1"}},
		devices: []entities.Device{{Interface: "Gi1/0/1", Mac: "0004f21a2b3c", Vlan: "1"}},
	}
	cfg := entities.SwitchConfig{Sandbox: true, DefaultVlan: "1", MacToVlan: mapping}
	result, err := NewVLANService(repo, cfg, drv).ProcessPorts()
	if err != nil {
		t.Fatalf("ProcessPorts: %v", err)
	}
	if result.PortsChanged != 1 {
		t.Fatalf("PortsChanged=%d; want 1 (exact MAC -> VLAN 55)", result.PortsChanged)
	}
}

func TestLongestPrefixMatchVariableLengths(t *testing.T) {
	mapping := map[string]string{
		"3c2af4":   "20",
		"3c2af499": "25",
	}
	if got := resolveMacToVlan("3c2af4998877", mapping, "1"); got != "25" {
		t.Fatalf("8-char prefix = %s; want 25", got)
	}
	if got := resolveMacToVlan("3c2af4aabbcc", mapping, "1"); got != "20" {
		t.Fatalf("6-char prefix = %s; want 20", got)
	}
	if got := resolveMacToVlan("aabbccddeeff", mapping, "10"); got != "10" {
		t.Fatalf("default fallback = %s; want 10", got)
	}
}
