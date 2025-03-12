package dmos

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestDriverName(t *testing.T) {
	if (&Driver{}).Name() != "dmos" {
		t.Fatal("name")
	}
}

func TestParseVLANs(t *testing.T) {
	output := `
VLAN 1 [DefaultVlan]:
VLAN 10 [VLAN_10]:
VLAN 20:
`
	got := parseVLANs(output)
	expected := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseVLANs() = %v; expected %v", got, expected)
	}
}

func TestParseDmOSTrunksFromSwitchport(t *testing.T) {
	output := `
interface ethernet 1/1
  Description: Link to Core
  Allowed VLANs: 1-4094 (t)
interface ethernet 1/2
  Description: Access Port
  Allowed VLANs: 10 (t)
interface ethernet 1/3
  Allowed VLANs: 20
`
	got := parseDmOSTrunksFromSwitchport(output)
	expected := []string{"ethernet1/1", "ethernet1/2"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseDmOSTrunksFromSwitchport() = %v; expected %v", got, expected)
	}
}

func TestParseDmOSTrunks(t *testing.T) {
	output := `
Ethernet1/1   trunk
Ethernet1/2   access
Gi1/0/1       trunk
`
	got := parseDmOSTrunks(output)
	expected := []string{"Ethernet1/1", "Ethernet1/2"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseDmOSTrunks() = %v; expected %v", got, expected)
	}
}

func TestParseDmOSSwitchportVLANs(t *testing.T) {
	output := `
interface ethernet 1/1
  Native VLAN: 1
interface ethernet 1/2
  Native VLAN: 10
`
	got := parseDmOSSwitchportVLANs(output)
	expected := map[string]string{"ethernet1/1": "1", "ethernet1/2": "10"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseDmOSSwitchportVLANs() = %v; expected %v", got, expected)
	}
}

func TestParseActivePorts(t *testing.T) {
	statusOutput := `
Information of Eth 1/1:
  Link status: Up
Information of Eth 1/2:
  Link status: Down
Information of Eth 1/3:
  Link status: Up
`
	switchportVLANs := map[string]string{"ethernet1/1": "10", "ethernet1/3": "20"}
	got := parseActivePorts(statusOutput, switchportVLANs)
	expected := []entities.Port{
		{Interface: "ethernet1/1", Vlan: "10"},
		{Interface: "ethernet1/3", Vlan: "20"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseActivePorts() = %+v; expected %+v", got, expected)
	}
}
