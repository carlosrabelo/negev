package dmos

import (
	"reflect"
	"testing"
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
