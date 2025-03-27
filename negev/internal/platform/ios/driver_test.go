package ios

import (
	"errors"
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	version   string
	responses map[string]string
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
func (m *mockRepo) SetSessionProfile(profile entities.SessionProfile) {}
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if cmd == "show version" {
		return m.version, nil
	}
	if m.responses != nil {
		if out, ok := m.responses[cmd]; ok {
			return out, nil
		}
	}
	return "", errors.New("unexpected command: " + cmd)
}

func TestDetectRejectsNXOS(t *testing.T) {
	banner := `
Cisco Nexus Operating System (NX-OS) Software
Software
  NXOS: version 9.3(8)
Hardware
  cisco Nexus9000 C93180YC-EX chassis
`
	match, err := (&Driver{}).Detect(&mockRepo{version: banner})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if match {
		t.Fatal("Detect(NX-OS banner) = true; expected false")
	}
}

func TestParseVLANs(t *testing.T) {
	output := `
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi1/0/1, Gi1/0/2
10   VLAN_10                          active    Gi1/0/3
20   VLAN_20                          active
`
	got := parseVLANs(output)
	expected := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseVLANs() = %v; expected %v", got, expected)
	}
}

func TestParseTrunkInterfaces(t *testing.T) {
	output := `
Port        Mode             Encapsulation  Status        Native vlan
Gi1/0/24    on               802.1q         trunking      1

Port        Vlans allowed on trunk
Gi1/0/24    1-4094
`
	got := parseTrunkInterfaces(output)
	expected := []string{"Gi1/0/24"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseTrunkInterfaces() = %v; expected %v", got, expected)
	}
}

func TestParseActivePorts(t *testing.T) {
	output := `
Port      Name               Status       Vlan       Duplex  Speed Type
Gi1/0/1   connected          connected    10         a-full  a-1000 10/100/1000BaseTX
Gi1/0/2   Link connected Core connected    20         a-full  a-1000 10/100/1000BaseTX
Gi1/0/3   Link up back       connected    30         a-full  a-1000 10/100/1000BaseTX
Gi1/0/4                      notconnect   1            auto   auto 10/100/1000BaseTX
Gi1/0/5   SPAN dest          monitor      10         a-full  a-1000 10/100/1000BaseTX
Gi1/0/6                      connected    routed     a-full  a-1000 10/100/1000BaseTX
`
	got := parseActivePorts(output)
	expected := []entities.Port{
		{Interface: "Gi1/0/1", Vlan: "10"},
		{Interface: "Gi1/0/2", Vlan: "20"},
		{Interface: "Gi1/0/3", Vlan: "30"},
		{Interface: "Gi1/0/6", Vlan: "routed", IsRouted: true},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseActivePorts() = %+v; expected %+v", got, expected)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
          Mac Address Table
-------------------------------------------

Vlan    Mac Address       Type        Ports
----    -----------       ----        -----
  10    0011.2233.4455    DYNAMIC     Gi1/0/1
  20    aabb.ccdd.eeff    Dynamic     Gi1/0/2
  30    0000.1111.2222    STATIC      Gi1/0/3
  20    0000.0000.1111    DYNAMIC     Gi1/0/24
`
	trunkSet := map[string]bool{"gigabitethernet1/0/24": true}
	got := parseMacTable(output, trunkSet, &Driver{})
	expected := []entities.Device{
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "GigabitEthernet1/0/1"},
		{Vlan: "20", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "GigabitEthernet1/0/2"},
		{Vlan: "30", Mac: "000011112222", MacFull: "00:00:11:11:22:22", Interface: "GigabitEthernet1/0/3", IsStatic: true},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseMacTable() = %+v; expected %+v", got, expected)
	}
}

func TestGetSessionProfile(t *testing.T) {
	p := (&Driver{}).GetSessionProfile()
	if p.PromptPattern == nil || !p.PromptPattern.MatchString("Switch#\n") {
		t.Fatal("expected IOS prompt pattern to match Switch#")
	}
	if p.PagerPattern == nil || !p.PagerPattern.MatchString("--More--") {
		t.Fatal("expected IOS pager pattern to match --More--")
	}
	if p.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q", p.PagerResponse)
	}
}

func TestNormalizeInterface(t *testing.T) {
	d := &Driver{}
	cases := map[string]string{
		"Gi1/0/1":              "GigabitEthernet1/0/1",
		"Fa0/1":                "FastEthernet0/1",
		"Te1/1/1":              "TenGigabitEthernet1/1/1",
		"Po10":                 "Port-channel10",
		"GigabitEthernet1/0/2": "GigabitEthernet1/0/2",
	}
	for in, want := range cases {
		if got := d.NormalizeInterface(in); got != want {
			t.Errorf("NormalizeInterface(%q) = %q; expected %q", in, got, want)
		}
	}
}

func TestGetL3Interfaces(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"show ip interface brief": `
Interface              IP-Address      OK? Method Status                Protocol
Vlan1                  192.168.1.1     YES NVRAM  up                    up
Vlan10                 unassigned      YES NVRAM  up                    up
GigabitEthernet1/0/1   unassigned      YES unset  up                    up
Vlan20                 10.0.0.1        YES NVRAM  up                    up
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
