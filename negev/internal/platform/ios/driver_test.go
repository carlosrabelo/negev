package ios

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

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
`
	got := parseActivePorts(output)
	expected := []entities.Port{
		{Interface: "Gi1/0/1", Vlan: "10"},
		{Interface: "Gi1/0/2", Vlan: "20"},
		{Interface: "Gi1/0/3", Vlan: "30"},
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
  20    aabb.ccdd.eeff    DYNAMIC     Gi1/0/2
  20    0000.0000.1111    DYNAMIC     Gi1/0/24
`
	trunkSet := map[string]bool{"Gi1/0/24": true}
	got := parseMacTable(output, trunkSet)
	expected := []entities.Device{
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "Gi1/0/1"},
		{Vlan: "20", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "Gi1/0/2"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseMacTable() = %+v; expected %+v", got, expected)
	}
}
