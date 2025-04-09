package nxos

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	version   string
	err       error
	responses map[string]string
	cmdErr    map[string]error
}

func (m *mockRepo) Connect() error                                    { return nil }
func (m *mockRepo) Disconnect()                                       {}
func (m *mockRepo) IsConnected() bool                                 { return true }
func (m *mockRepo) SetSessionProfile(profile entities.SessionProfile) {}
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if cmd == "show version" {
		if m.err != nil {
			return "", m.err
		}
		return m.version, nil
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
	d := &Driver{}
	if got := d.Name(); got != "nxos" {
		t.Fatalf("Name() = %q; expected %q", got, "nxos")
	}
}

func TestDetectNXOS(t *testing.T) {
	d := &Driver{}
	nxosBanner := `
Cisco Nexus Operating System (NX-OS) Software
Software
  NXOS: version 9.3(8)
Hardware
  cisco Nexus9000 C93180YC-EX chassis
`
	match, err := d.Detect(&mockRepo{version: nxosBanner})
	if err != nil {
		t.Fatalf("Detect(nxos) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(nxos banner) = false; expected true")
	}

	iosBanner := `Cisco IOS Software, C2960 Software (C2960-LANBASEK9-M), Version 15.0(2)SE`
	match, err = d.Detect(&mockRepo{version: iosBanner})
	if err != nil {
		t.Fatalf("Detect(ios) error: %v", err)
	}
	if match {
		t.Fatal("Detect(ios banner) = true; expected false")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	want := []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
	if len(seq) != len(want) {
		t.Fatalf("len(seq) = %d; expected %d", len(seq), len(want))
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("seq[%d] = %+v; expected %+v", i, seq[i], want[i])
		}
	}
	for _, p := range seq {
		if strings.Contains(p.SendCmd, "enable") {
			t.Fatalf("auth sequence must not send enable; got %q", p.SendCmd)
		}
	}
}

func TestNormalizeInterface(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Eth1/1", "Ethernet1/1"},
		{"Ethernet1/10", "Ethernet1/10"},
		{"ethernet 1/1/1", "Ethernet1/1/1"},
		{"Eth101/1/48", "Ethernet101/1/48"},
		{"Po1", "port-channel1"},
		{"port-channel10", "port-channel10"},
		{"mgmt0", "mgmt0"},
		{"Vlan10", "Vlan10"},
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
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Eth1/1, Eth1/2
10   USERS                            active    Eth1/3
20   SERVERS                          active
`
	got := parseVLANs(output)
	want := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseVLANs() = %v; expected %v", got, want)
	}
}

func TestParseTrunkInterfaces(t *testing.T) {
	output := `
Port          Native  Status        Port
              Vlan                  Channel
Eth1/48       1       trunking      --
Po1           1       trunking      --

Port          Vlans Allowed on Trunk
Eth1/48       1-4094
Po1           1-4094
`
	got := parseTrunkInterfaces(output)
	want := []string{"Ethernet1/48", "port-channel1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkInterfaces() = %v; expected %v", got, want)
	}
}

func TestParseActivePorts(t *testing.T) {
	output := `
Port          Name               Status    Vlan      Duplex  Speed   Type
Eth1/1        --                 connected 10        full    1000    10g
Eth1/2        server A           notconnec 1         auto    auto    --
Eth1/3        uplink to core     connected trunk     full    10G     10g
Eth1/4        --                 disabled  1         full    1000    --
Eth1/5        --                 sfpAbsent 1         auto    auto    --
Eth1/10       access             connected 20        full    1000    10g
Po1           --                 connected trunk     full    10G     --
mgmt0         --                 connected routed    full    1000    --
`
	got := parseActivePorts(output)
	want := []entities.Port{
		{Interface: "Ethernet1/1", Vlan: "10"},
		{Interface: "Ethernet1/10", Vlan: "20"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseActivePorts() = %+v; expected %+v", got, want)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
   VLAN     MAC Address      Type      age     Secure NTFY Ports
---------+-----------------+--------+---------+------+----+------------------
* 10       0011.2233.4455   dynamic  10        F      F    Eth1/1
  20       aabb.ccdd.eeff   dynamic  20        F      F    Eth1/10
G 10       0000.0000.0001   static   -         F      F    sup-eth1(R)
  10       0011.2233.4466   dynamic  5         F      F    Eth1/48
C 30       0050.56aa.bbcc   static   -         F      F    vPC Peer-Link
`
	trunkSet := map[string]bool{"Ethernet1/48": true, "port-channel1": true}
	got := parseMacTable(output, trunkSet)
	want := []entities.Device{
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "Ethernet1/1"},
		{Vlan: "20", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "Ethernet1/10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable() = %+v; expected %+v", got, want)
	}
}

func TestConfigureAccessCommands(t *testing.T) {
	got := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "Ethernet1/1"}, "10")
	want := []string{
		"configure terminal",
		"interface Ethernet1/1",
		"switchport",
		"switchport mode access",
		"switchport access vlan 10",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigureAccessCommands() = %v; expected %v", got, want)
	}
}

func TestCreateVLANCommands(t *testing.T) {
	got := (&Driver{}).CreateVLANCommands("20")
	want := []string{
		"configure terminal",
		"vlan 20",
		"exit",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateVLANCommands() = %v; expected %v", got, want)
	}
	for _, cmd := range got {
		if strings.Contains(cmd, "interface vlan") {
			t.Fatalf("CreateVLANCommands must not create an SVI; got %q", cmd)
		}
	}
}

func TestDeleteVLANCommands(t *testing.T) {
	got := (&Driver{}).DeleteVLANCommands("20")
	want := []string{
		"configure terminal",
		"no vlan 20",
		"end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteVLANCommands() = %v; expected %v", got, want)
	}
}

func TestSaveCommands(t *testing.T) {
	got := (&Driver{}).SaveCommands()
	want := []string{"copy running-config startup-config"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCommands() = %v; expected %v", got, want)
	}
}

func TestIsNXOSCommandError(t *testing.T) {
	d := &Driver{}
	if !d.IsCommandError("% Invalid command at '^' marker.") {
		t.Fatal("expected invalid command to be an error")
	}
	if d.IsCommandError("Packets invalid: 0") {
		t.Fatal("must not treat a counter line containing 'invalid' as a command error")
	}
}

func TestGetSessionProfile(t *testing.T) {
	p := (&Driver{}).GetSessionProfile()
	if p.PromptPattern == nil || !p.PromptPattern.MatchString("Nexus#\n") {
		t.Fatal("expected NX-OS prompt pattern to match Nexus#")
	}
	if p.PagerPattern == nil || !p.PagerPattern.MatchString("--More--") {
		t.Fatal("expected NX-OS pager pattern to match --More--")
	}
	if p.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q", p.PagerResponse)
	}
}

func TestGetL3Interfaces(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show ip interface brief": `
IP Interface Status for VRF "default"(1)
Interface            IP Address      Interface Status
Vlan1                192.168.1.1     protocol-up/link-up/admin-up
Vlan10               10.0.0.1        protocol-up/link-up/admin-up
Ethernet1/1          unassigned      protocol-up/link-up/admin-up
Vlan20               10.0.20.1       protocol-up/link-up/admin-up
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
