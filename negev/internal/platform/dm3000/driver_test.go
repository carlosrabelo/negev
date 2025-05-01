package dm3000

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
	err       error
	responses map[string]string
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
	if got := (&Driver{}).Name(); got != "dm3000" {
		t.Fatalf("Name() = %q; expected %q", got, "dm3000")
	}
}

func TestDetectDm3000(t *testing.T) {
	d := &Driver{}

	banner3324 := `
DmSwitch3324F2 Device, Compiled Mar 15 2010
System software version: 13.0.0
Copyright (c) Datacom
`
	match, err := d.Detect(&mockRepo{version: banner3324})
	if err != nil {
		t.Fatalf("Detect(3324F2) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(3324F2 banner) = false; expected true")
	}

	banner1200 := `
DM1200E 24GT + 4GX
Software Version 1.0.0.x
Datacom
`
	match, err = d.Detect(&mockRepo{version: banner1200})
	if err != nil {
		t.Fatalf("Detect(DM1200E) error: %v", err)
	}
	if match {
		t.Fatal("Detect(DM1200E banner) = true; expected false")
	}

	bareDatacom := `Copyright (c) Datacom`
	match, err = d.Detect(&mockRepo{version: bareDatacom})
	if err != nil {
		t.Fatalf("Detect(bare datacom) error: %v", err)
	}
	if match {
		t.Fatal("Detect(bare datacom) = true; expected false")
	}

	systemOnly := &mockRepo{
		version: "% 30: Invalid input detected at '^' marker.",
		responses: map[string]string{
			"show system": readFixture(t, "show_system.txt"),
		},
	}
	match, err = d.Detect(systemOnly)
	if err != nil {
		t.Fatalf("Detect(show system fallback) error: %v", err)
	}
	if !match {
		t.Fatal("Detect via show system = false; expected true")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) != 4 {
		t.Fatalf("len(seq) = %d; expected 4", len(seq))
	}
	if seq[0].WaitFor != "Username:" || seq[0].SendCmd != "USERNAME_PLACEHOLDER\n" {
		t.Fatalf("first step = %+v", seq[0])
	}
	if seq[1].WaitFor != "Password:" || seq[1].SendCmd != "PASSWORD_PLACEHOLDER\n" {
		t.Fatalf("second step = %+v", seq[1])
	}
	if seq[2].WaitFor != "#" || seq[2].SendCmd != "no terminal paging\n" {
		t.Fatalf("third step = %+v", seq[2])
	}
	if seq[3].WaitFor != "#" || seq[3].SendCmd != "" {
		t.Fatalf("fourth step = %+v", seq[3])
	}
	for _, p := range seq {
		if p.SendCmd == "enable\n" {
			t.Fatal("auth sequence must not send enable")
		}
	}
}

func TestNormalizeInterface(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Eth 1/1", "ethernet1/1"},
		{"Eth 1/ 1", "ethernet1/1"},
		{"ethernet 1/14", "ethernet1/14"},
		{"Ethernet1/3", "ethernet1/3"},
		{"Po1", "port-channel1"},
		{"port-channel10", "port-channel10"},
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

func TestParseVLANsRealAndLegacy(t *testing.T) {
	got := parseVLANs(readFixture(t, "show_vlan.txt"))
	want := []string{"1", "10", "40", "50"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs(real) = %v; expected %v", got, want)
	}

	legacy := `
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Eth 1/1
10   USERS                            active
VLAN 20:
`
	got = parseVLANs(legacy)
	want = []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs(legacy) = %v; expected %v", got, want)
	}
}

func TestParseTrunkInterfacesRealAndLegacy(t *testing.T) {
	got := parseTrunkInterfaces(readFixture(t, "show_interfaces_switchport.txt"))
	want := []string{"ethernet1/25"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkInterfaces(real) = %v; expected %v", got, want)
	}

	legacy := `
interface ethernet 1/1
  Allowed VLANs: 1-4094 (t)
interface ethernet 1/3
  Allowed VLANs: 10
interface ethernet 1/24
  Allowed VLANs: 1-4094 (t)
`
	got = parseTrunkInterfaces(legacy)
	want = []string{"ethernet1/1", "ethernet1/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkInterfaces(legacy) = %v; expected %v", got, want)
	}
}

func TestParseActivePortsReal(t *testing.T) {
	clearSwitchportCache()
	t.Cleanup(clearSwitchportCache)

	repo := &mockRepo{
		target: "192.0.2.4",
		responses: map[string]string{
			"show interfaces status":     readFixture(t, "show_interfaces_status.txt"),
			"show interfaces switchport": readFixture(t, "show_interfaces_switchport.txt"),
		},
	}
	got, err := (&Driver{}).GetActivePorts(repo)
	if err != nil {
		t.Fatalf("GetActivePorts failed: %v", err)
	}
	want := []entities.Port{
		{Interface: "ethernet1/1", Vlan: "40"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetActivePorts() = %+v; expected %+v", got, want)
	}
}

func TestParseMacTableRealAndLegacy(t *testing.T) {
	trunkSet := map[string]bool{"ethernet1/25": true}
	got := parseMacTable(readFixture(t, "show_mac_address_table.txt"), trunkSet)
	want := []entities.Device{
		{Vlan: "40", Mac: "001122aabb01", MacFull: "00:11:22:aa:bb:01", Interface: "ethernet1/1", IsStatic: false},
		{Vlan: "40", Mac: "001122aabb02", MacFull: "00:11:22:aa:bb:02", Interface: "ethernet1/1", IsStatic: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable(real) = %+v; expected %+v", got, want)
	}

	legacy := `
Unit Interface MAC Address        VLAN Type
---- --------- -----------------  ---- ---------
1    Eth 1/ 1  00-e0-29-94-24-de  1    Static
1    Eth 1/ 3  00-11-22-33-44-55  10   Dynamic
1    Eth 1/24  aa-bb-cc-dd-ee-ff  20   Dynamic
`
	got = parseMacTable(legacy, map[string]bool{"ethernet1/24": true})
	want = []entities.Device{
		{Vlan: "1", Mac: "00e0299424de", MacFull: "00:e0:29:94:24:de", Interface: "ethernet1/1", IsStatic: true},
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "ethernet1/3", IsStatic: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable(legacy) = %+v; expected %+v", got, want)
	}
}

func TestConfigureAccessCommands(t *testing.T) {
	got := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "ethernet1/3", Vlan: "40"}, "10")
	want := []string{
		"configure",
		"interface vlan 40",
		"no set-member ethernet 1/3",
		"exit",
		"interface vlan 10",
		"set-member untagged ethernet 1/3",
		"exit",
		"interface ethernet 1/3",
		"switchport native vlan 10",
		"exit",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigureAccessCommands(move) = %v; expected %v", got, want)
	}

	gotNew := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "ethernet1/3"}, "10")
	wantNew := []string{
		"configure",
		"interface vlan 10",
		"set-member untagged ethernet 1/3",
		"exit",
		"interface ethernet 1/3",
		"switchport native vlan 10",
		"exit",
		"end",
	}
	if !reflect.DeepEqual(gotNew, wantNew) {
		t.Fatalf("ConfigureAccessCommands(no old vlan) = %v; expected %v", gotNew, wantNew)
	}
}

func TestCreateVLANCommands(t *testing.T) {
	got := (&Driver{}).CreateVLANCommands("10")
	want := []string{
		"configure",
		"interface vlan 10",
		"exit",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateVLANCommands() = %v; expected %v", got, want)
	}
}

func TestDeleteVLANCommands(t *testing.T) {
	got := (&Driver{}).DeleteVLANCommands("10")
	want := []string{
		"configure",
		"no interface vlan 10",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteVLANCommands() = %v; expected %v", got, want)
	}
}

func TestSaveCommands(t *testing.T) {
	got := (&Driver{}).SaveCommands()
	want := []string{
		"copy running-config startup-config",
		"write memory",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCommands() = %v; expected %v", got, want)
	}
}

func TestIsDm3000CommandError(t *testing.T) {
	d := &Driver{}
	for _, sample := range []string{
		"% Error: unknown command",
		"% Unrecognized command",
		"% 30: Invalid input detected at '^' marker.",
	} {
		if !d.IsCommandError(sample) {
			t.Fatalf("expected %q to be treated as command error", sample)
		}
	}
	if d.IsCommandError("Packets invalid: 0") {
		t.Fatal("must not treat a counter line containing 'invalid' as a command error")
	}
}

func TestGetSessionProfile(t *testing.T) {
	p := (&Driver{}).GetSessionProfile()
	for _, sample := range []string{
		"DmSwitch#\n",
		"dc-sw-04(config)#\n",
		"dc-sw-04(config-if-eth1/3)#\n",
	} {
		if p.PromptPattern == nil || !p.PromptPattern.MatchString(sample) {
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

func TestGetL3InterfacesRealAndFallback(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show ip interface": readFixture(t, "show_ip_interface.txt"),
	}}
	got, err := (&Driver{}).GetL3Interfaces(repo)
	if err != nil {
		t.Fatalf("GetL3Interfaces(real) error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"50"}) {
		t.Fatalf("GetL3Interfaces(real) = %v; expected [50]", got)
	}

	repoBrief := &mockRepo{responses: map[string]string{
		"show ip interface": "% Unrecognized command",
		"show ip interface brief": `
Interface            IP-Address      Status
Vlan1                192.0.2.1       up
Vlan10               192.0.2.10      up
ethernet1/1          unassigned      up
Vlan-interface20     192.0.2.20      up
`,
	}}
	got, err = (&Driver{}).GetL3Interfaces(repoBrief)
	if err != nil {
		t.Fatalf("GetL3Interfaces(brief) error: %v", err)
	}
	want := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetL3Interfaces(brief) = %v; expected %v", got, want)
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
