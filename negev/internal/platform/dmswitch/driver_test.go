package dmswitch

import (
	"errors"
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	target  string
	version string
	err     error
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
func (m *mockRepo) GetTarget() string { return m.target }
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if cmd == "show version" {
		if m.err != nil {
			return "", m.err
		}
		return m.version, nil
	}
	return "", errors.New("unexpected command: " + cmd)
}

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dmswitch" {
		t.Fatalf("Name() = %q; expected %q", got, "dmswitch")
	}
}

func TestDetectDmSwitch(t *testing.T) {
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
	if seq[2].WaitFor != "#" || seq[2].SendCmd != "terminal length 0\n" {
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
	for _, tc := range tests {
		if got := normalizeInterface(tc.in); got != tc.want {
			t.Errorf("normalizeInterface(%q) = %q; expected %q", tc.in, got, tc.want)
		}
	}
}

func TestParseVLANs(t *testing.T) {
	output := `
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Eth 1/1
10   USERS                            active
VLAN 20:
`
	got := parseVLANs(output)
	want := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs() = %v; expected %v", got, want)
	}
}

func TestParseTrunkInterfaces(t *testing.T) {
	output := `
interface ethernet 1/1
  Allowed VLANs: 1-4094 (t)
interface ethernet 1/3
  Allowed VLANs: 10
interface ethernet 1/24
  Allowed VLANs: 1-4094 (t)
`
	got := parseTrunkInterfaces(output)
	want := []string{"ethernet1/1", "ethernet1/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkInterfaces() = %v; expected %v", got, want)
	}
}

func TestParseActivePorts(t *testing.T) {
	status := `
Information of Eth 1/1:
  Link status: Up
Information of Eth 1/2:
  Link status: Down
Information of Eth 1/3:
  Link status: Up
`
	vlans := map[string]string{
		"ethernet1/1": "1",
		"ethernet1/3": "10",
	}
	got := parseActivePorts(status, vlans)
	want := []entities.Port{
		{Interface: "ethernet1/1", Vlan: "1"},
		{Interface: "ethernet1/3", Vlan: "10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseActivePorts() = %+v; expected %+v", got, want)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
Unit Interface MAC Address        VLAN Type
---- --------- -----------------  ---- ---------
1    Eth 1/ 1  00-e0-29-94-24-de  1    Static
1    Eth 1/ 3  00-11-22-33-44-55  10   Dynamic
1    Eth 1/24  aa-bb-cc-dd-ee-ff  20   Dynamic
`
	trunkSet := map[string]bool{"ethernet1/24": true}
	got := parseMacTable(output, trunkSet)
	want := []entities.Device{
		{Vlan: "1", Mac: "00e0299424de", MacFull: "00:e0:29:94:24:de", Interface: "ethernet1/1"},
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "ethernet1/3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable() = %+v; expected %+v", got, want)
	}
}
