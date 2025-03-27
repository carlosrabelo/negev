package dm1200

import (
	"errors"
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	version   string
	sysinfo   string
	err       error
	responses map[string]string
	cmdErr    map[string]error
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
func (m *mockRepo) SetSessionProfile(profile entities.SessionProfile) {}
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

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dm1200" {
		t.Fatalf("Name() = %q; expected %q", got, "dm1200")
	}
}

func TestDetectDM1200(t *testing.T) {
	d := &Driver{}

	banner1200 := `
DM1200E 24GT + 4GX
Software Version 1.0.0.x
Datacom
`
	match, err := d.Detect(&mockRepo{version: banner1200})
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
	repo := &mockRepo{
		err:     errors.New("unrecognized command"),
		sysinfo: "DM1200E 24GT + 4GX\nDatacom\n",
	}
	match, err := d.Detect(repo)
	if err != nil {
		t.Fatalf("Detect(sysinfo fallback) error: %v", err)
	}
	if !match {
		t.Fatal("Detect via show sysinfo = false; expected true")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) != 4 || seq[0].WaitFor != "User:" {
		t.Fatalf("seq = %+v", seq)
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

func TestParseVLANs(t *testing.T) {
	output := `
VLAN  Name
----- ----------
1     default
2     Gerencia
10-12 Data
`
	got := parseVLANs(output)
	want := []string{"1", "2", "10", "11", "12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs() = %v; expected %v", got, want)
	}
}

func TestParseTrunkInterfaces(t *testing.T) {
	vlanPortAll := `
Port     Mode
1/0/1    Untagged
1/0/25   Tagged
1/0/2    access
`
	got := parseTrunkFromVlanPortAll(vlanPortAll)
	want := []string{"1/0/25"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkFromVlanPortAll() = %v; expected %v", got, want)
	}

	status := `
Port   Name   Duplex  Speed  Neg  Link  Flow Control  VLAN
1/0/1         Full    1000   Auto Up    On            10
1/0/25        Full    1000   Auto Up    On            trunk
`
	got = parseTrunkFromStatus(status)
	want = []string{"1/0/25"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkFromStatus() = %v; expected %v", got, want)
	}
}

func TestParseActivePorts(t *testing.T) {
	output := `
Port   Name   Duplex  Speed  Neg  Link  Flow Control  VLAN
1/0/1         Full    1000   Auto Up    On            10
1/0/2         N/A     Unknown Auto Down Off           1
1/0/25        Full    1000   Auto Up    On            trunk
`
	got := parseActivePorts(output)
	want := []entities.Port{
		{Interface: "1/0/1", Vlan: "10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseActivePorts() = %+v; expected %+v", got, want)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
VLAN  MAC Address        Port
10    00:11:22:33:44:55  1/0/1
20    00-aa-bb-cc-dd-ee  1/0/10
30    aabb.ccdd.eeff     1/0/25
`
	trunkSet := map[string]bool{"1/0/25": true}
	got := parseMacTable(output, trunkSet)
	want := []entities.Device{
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "1/0/1"},
		{Vlan: "20", Mac: "00aabbccddee", MacFull: "00:aa:bb:cc:dd:ee", Interface: "1/0/10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable() = %+v; expected %+v", got, want)
	}
}

func TestConfigureAccessCommands(t *testing.T) {
	got := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "1/0/1"}, "10")
	want := []string{
		"configure",
		"interface 1/0/1",
		"switchport mode access",
		"switchport access vlan 10",
		"vlan participation exclude 1",
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
		"copy system:running-config nvram:startup-config",
		"write memory",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCommands() = %v; expected %v", got, want)
	}
}

func TestIsDM1200CommandError(t *testing.T) {
	d := &Driver{}
	if !d.IsCommandError("Invalid input detected at '^' marker.") {
		t.Fatal("expected invalid input to be an error")
	}
	if !d.IsCommandError("Unrecognized command") {
		t.Fatal("expected unrecognized command to be an error")
	}
	if !d.IsCommandError("% Error: incomplete command") {
		t.Fatal("expected error: to be an error")
	}
	if d.IsCommandError("Packets invalid: 0") {
		t.Fatal("must not treat a counter line containing 'invalid' as a command error")
	}
}

func TestGetSessionProfile(t *testing.T) {
	p := (&Driver{}).GetSessionProfile()
	if p.PromptPattern == nil || !p.PromptPattern.MatchString("DM1200E#\n") {
		t.Fatal("expected DM1200E prompt pattern to match DM1200E#")
	}
	if p.PagerPattern == nil || !p.PagerPattern.MatchString("--More--") {
		t.Fatal("expected pager pattern to match --More--")
	}
	if p.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q", p.PagerResponse)
	}
}

func TestGetL3Interfaces(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show ip interface brief": `
Interface            IP-Address      Status
Vlan1                192.168.1.1     up
Vlan10               10.0.0.1        up
1/0/1                unassigned      up
Vlan-interface20     10.0.20.1       up
`,
	}}
	got, err := (&Driver{}).GetL3Interfaces(repo)
	if err != nil {
		t.Fatalf("GetL3Interfaces error: %v", err)
	}
	want := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetL3Interfaces() = %v; expected %v", got, want)
	}
}
