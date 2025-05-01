package dm1200e

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	target    string
	version   string
	sysinfo   string
	err       error
	responses map[string]string
	cmdErr    map[string]error
}

func (m *mockRepo) Connect() error                                    { return nil }
func (m *mockRepo) Disconnect()                                       {}
func (m *mockRepo) IsConnected() bool                                 { return true }
func (m *mockRepo) SetSessionProfile(profile entities.SessionProfile) {}
func (m *mockRepo) GetTarget() string                                 { return m.target }
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if cmd == "show version" {
		if m.err != nil {
			return "", m.err
		}
		return m.version, nil
	}
	if cmd == "show sysinfo" {
		return m.sysinfo, nil
	}
	if m.cmdErr != nil {
		if err, ok := m.cmdErr[cmd]; ok {
			return "", err
		}
	}
	if m.responses != nil {
		if out, ok := m.responses[cmd]; ok {
			return out, nil
		}
	}
	return "", errors.New("unexpected command: " + cmd)
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dm1200e" {
		t.Fatalf("Name() = %q; expected %q", got, "dm1200e")
	}
}

func TestDetectDM1200E(t *testing.T) {
	d := &Driver{}

	match, err := d.Detect(&mockRepo{version: readFixture(t, "show_version.txt")})
	if err != nil {
		t.Fatalf("Detect(DM1200E) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(DM1200E banner) = false; expected true")
	}

	banner3324 := `
DmSwitch3324F2 Device, Compiled Mar 15 2010
System software version: 13.0.0
Copyright (c) Datacom
`
	match, err = d.Detect(&mockRepo{version: banner3324})
	if err != nil {
		t.Fatalf("Detect(3324F2) error: %v", err)
	}
	if match {
		t.Fatal("Detect(3324F2 banner) = true; expected false")
	}
}

func TestDetectFallsBackToSysinfo(t *testing.T) {
	d := &Driver{}

	repoErr := &mockRepo{
		err:     errors.New("unrecognized command"),
		sysinfo: "DM1200E 24GT + 4GX\nDatacom\n",
	}
	match, err := d.Detect(repoErr)
	if err != nil {
		t.Fatalf("Detect(sysinfo fallback on err) error: %v", err)
	}
	if !match {
		t.Fatal("Detect via show sysinfo on error = false; expected true")
	}

	repoTextErr := &mockRepo{
		version: "Command not found / Incomplete command.",
		sysinfo: "DM1200E 24GP+4GX\nDatacom\n",
	}
	match, err = d.Detect(repoTextErr)
	if err != nil {
		t.Fatalf("Detect(sysinfo fallback on CLI error) error: %v", err)
	}
	if !match {
		t.Fatal("Detect via show sysinfo on CLI error = false; expected true")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) != 6 || seq[0].WaitFor != "User:" {
		t.Fatalf("seq = %+v", seq)
	}
	if seq[2].WaitFor != ">" || seq[2].SendCmd != "enable\n" {
		t.Fatalf("expected enable step, got %+v", seq[2])
	}
	if seq[3].SendCmd != "ENABLE_PASSWORD_PLACEHOLDER\n" {
		t.Fatalf("expected enable password placeholder, got %+v", seq[3])
	}
}

func TestNormalizeInterface(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1/0/1", "1/0/1"},
		{"1/0/25", "1/0/25"},
		{"ethernet 1/0/3", "1/0/3"},
		{"Eth1/0/3", "1/0/3"},
	}
	d := &Driver{}
	for _, tc := range tests {
		if got := normalizeInterface(tc.in); got != tc.want {
			t.Errorf("normalizeInterface(%q) = %q; expected %q", tc.in, got, tc.want)
		}
		if got := d.NormalizeInterface(tc.in); got != tc.want {
			t.Errorf("Driver.NormalizeInterface(%q) = %q; expected %q", tc.in, got, tc.want)
		}
	}
}

func TestParseVLANsRealAndNamed(t *testing.T) {
	got := parseVLANs(readFixture(t, "show_vlan.txt"))
	want := []string{"1", "10", "20", "50", "100", "101", "102"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs(real) = %v; expected %v", got, want)
	}
}

func TestGetTrunkInterfacesRealAndFallback(t *testing.T) {
	clearSwitchportCache()
	t.Cleanup(clearSwitchportCache)

	repo := &mockRepo{
		target: "192.0.2.21",
		responses: map[string]string{
			"show interfaces switchport": readFixture(t, "show_interfaces_switchport.txt"),
		},
	}
	got, err := (&Driver{}).GetTrunkInterfaces(repo)
	if err != nil {
		t.Fatalf("GetTrunkInterfaces(real) error: %v", err)
	}
	want := []string{"1/0/2", "1/0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetTrunkInterfaces(real) = %v; expected %v", got, want)
	}

	vlanPortAll := `
Port     Mode
1/0/1    Untagged
1/0/25   Tagged
1/0/2    access
`
	if got := parseTrunkFromVlanPortAll(vlanPortAll); !reflect.DeepEqual(got, []string{"1/0/25"}) {
		t.Fatalf("parseTrunkFromVlanPortAll() = %v; expected [1/0/25]", got)
	}

	status := `
Port   Name   Duplex  Speed  Neg  Link  Flow Control  VLAN
1/0/1         Full    1000   Auto Up    On            10
1/0/25        Full    1000   Auto Up    On            trunk
`
	if got := parseTrunkFromStatus(status); !reflect.DeepEqual(got, []string{"1/0/25"}) {
		t.Fatalf("parseTrunkFromStatus() = %v; expected [1/0/25]", got)
	}
}

func TestGetActivePortsRealAndFallback(t *testing.T) {
	clearSwitchportCache()
	t.Cleanup(clearSwitchportCache)

	repo := &mockRepo{
		target: "192.0.2.21",
		responses: map[string]string{
			"show port all":              readFixture(t, "show_port_all.txt"),
			"show interfaces switchport": readFixture(t, "show_interfaces_switchport.txt"),
		},
	}
	got, err := (&Driver{}).GetActivePorts(repo)
	if err != nil {
		t.Fatalf("GetActivePorts(real) failed: %v", err)
	}
	want := []entities.Port{
		{Interface: "1/0/1", Vlan: "10"},
		{Interface: "1/0/3", Vlan: "1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetActivePorts(real) = %+v; expected %+v", got, want)
	}

	fallbackRepo := &mockRepo{
		responses: map[string]string{
			"show port all": "Command not found / Incomplete command.",
			"show interfaces status": `
Port   Name   Duplex  Speed  Neg  Link  Flow Control  VLAN
1/0/1         Full    1000   Auto Up    On            10
1/0/2         N/A     Unknown Auto Down Off           1
1/0/25        Full    1000   Auto Up    On            trunk
`,
		},
	}
	got, err = (&Driver{}).GetActivePorts(fallbackRepo)
	if err != nil {
		t.Fatalf("GetActivePorts(fallback) failed: %v", err)
	}
	wantFallback := []entities.Port{
		{Interface: "1/0/1", Vlan: "10"},
	}
	if !reflect.DeepEqual(got, wantFallback) {
		t.Fatalf("GetActivePorts(fallback) = %+v; expected %+v", got, wantFallback)
	}
}

func TestParseMacTableReal(t *testing.T) {
	trunkSet := map[string]bool{"1/0/2": true, "1/0/24": true}
	got := parseMacTable(readFixture(t, "show_mac_addr_table.txt"), trunkSet)
	want := []entities.Device{
		{Vlan: "10", Mac: "001122aabb01", MacFull: "00:11:22:aa:bb:01", Interface: "1/0/1", IsStatic: false},
		{Vlan: "10", Mac: "001122aabb02", MacFull: "00:11:22:aa:bb:02", Interface: "1/0/1", IsStatic: true},
		{Vlan: "1", Mac: "001122aabb03", MacFull: "00:11:22:aa:bb:03", Interface: "1/0/3", IsStatic: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable(real) = %+v; expected %+v", got, want)
	}
}

func TestConfigureAccessCommands(t *testing.T) {
	got := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "1/0/1"}, "10")
	want := []string{
		"configure",
		"interface 1/0/1",
		"switchport mode access",
		"switchport access vlan 10",
		"exit",
		"exit",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigureAccessCommands() = %v; expected %v", got, want)
	}
}

func TestCreateVLANCommands(t *testing.T) {
	got := (&Driver{}).CreateVLANCommands("10")
	want := []string{
		"vlan database",
		"vlan 10",
		"exit",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateVLANCommands() = %v; expected %v", got, want)
	}
}

func TestDeleteVLANCommands(t *testing.T) {
	got := (&Driver{}).DeleteVLANCommands("10")
	want := []string{
		"vlan database",
		"no vlan 10",
		"exit",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteVLANCommands() = %v; expected %v", got, want)
	}
	if got := (&Driver{}).DeleteVLANCommands("1"); got != nil {
		t.Fatalf("DeleteVLANCommands(1) = %v; expected nil", got)
	}
}

func TestSaveCommands(t *testing.T) {
	got := (&Driver{}).SaveCommands()
	want := []string{
		"copy system:running-config nvram:startup-config\ny",
		"write memory\ny",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCommands() = %v; expected %v", got, want)
	}
}

func TestIsDM1200ECommandError(t *testing.T) {
	d := &Driver{}
	for _, sample := range []string{
		"Invalid input detected at '^' marker.",
		"Unrecognized command",
		"% Error: incomplete command",
		"Command not found / Incomplete command. Use ? to list commands.",
		"An invalid interface has been used for this function.",
	} {
		if !d.IsCommandError(sample) {
			t.Fatalf("expected %q to be an error", sample)
		}
	}
	if d.IsCommandError("Packets invalid: 0") {
		t.Fatal("must not treat a counter line containing 'invalid' as a command error")
	}
}

func TestGetSessionProfile(t *testing.T) {
	p := (&Driver{}).GetSessionProfile()
	if p.PromptPattern == nil {
		t.Fatal("expected prompt pattern")
	}
	for _, sample := range []string{
		"DM1200E#\n",
		"DM1200E>\n",
		"(DM1200E)#\n",
		"(DM1200E) >\n",
		"(DM1200E) (Vlan)#\n",
		"(DM1200E) (Config)#\n",
		"(DM1200E) (Interface 1/0/1)#\n",
	} {
		if !p.PromptPattern.MatchString(sample) {
			t.Fatalf("expected prompt pattern to match %q", sample)
		}
	}
	if p.PagerPattern == nil || !p.PagerPattern.MatchString("--More--") {
		t.Fatal("expected pager pattern to match --More--")
	}
	if p.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q", p.PagerResponse)
	}
}

func TestGetL3InterfacesReal(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show ip interface brief": readFixture(t, "show_ip_interface_brief.txt"),
	}}
	got, err := (&Driver{}).GetL3Interfaces(repo)
	if err != nil {
		t.Fatalf("GetL3Interfaces error: %v", err)
	}
	want := []string{"50"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetL3Interfaces() = %v; expected %v", got, want)
	}
}

func TestGetRunningConfig(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show running-config": "hostname test\n",
	}}
	got, err := (&Driver{}).GetRunningConfig(repo)
	if err != nil {
		t.Fatalf("GetRunningConfig: %v", err)
	}
	if !strings.Contains(got, "hostname test") {
		t.Fatalf("unexpected: %q", got)
	}
}
