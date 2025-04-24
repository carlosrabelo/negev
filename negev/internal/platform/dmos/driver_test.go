package dmos

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

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

func TestParseDmOSSwitchportVLANs(t *testing.T) {
	output := `
interface ethernet 1/1
  Native VLAN: 1
interface ethernet 1/2
  Native VLAN: 10
`
	got := parseDmOSSwitchportVLANs(output)
	expected := map[string]string{
		"ethernet1/1": "1",
		"ethernet1/2": "10",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseDmOSSwitchportVLANs() = %v; expected %v", got, expected)
	}
}

func TestCompareInterfaceNames(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"Ethernet1/1", "Ethernet1/2", true},
		{"Ethernet1/2", "Ethernet1/1", false},
		{"Ethernet1/10", "Ethernet1/2", false},
		{"Ethernet2/1", "Ethernet1/2", false},
		{"Ethernet1/2", "Ethernet2/1", true},
	}

	for _, tc := range tests {
		got := compareInterfaceNames(tc.a, tc.b)
		if got != tc.expected {
			t.Errorf("compareInterfaceNames(%q, %q) = %t; expected %t", tc.a, tc.b, got, tc.expected)
		}
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
	switchportVLANs := map[string]string{
		"ethernet1/1": "10",
		"ethernet1/3": "20",
	}
	got := parseActivePorts(statusOutput, switchportVLANs)
	expected := []entities.Port{
		{Interface: "ethernet1/1", Vlan: "10"},
		{Interface: "ethernet1/3", Vlan: "20"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseActivePorts() = %+v; expected %+v", got, expected)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
      1   Dynamic   Eth 1/1            00:11:22:33:44:55       10     Learned
      2   Dynamic   Eth 1/2            AA:BB:CC:DD:EE:FF       20     Learned
      3   Dynamic   Eth 1/3            00:00:00:00:11:11       30     Learned
`
	trunkSet := map[string]bool{"ethernet1/3": true}
	got := parseMacTable(output, trunkSet)
	expected := []entities.Device{
		{Vlan: "10", Mac: "001122334455", MacFull: "00:11:22:33:44:55", Interface: "ethernet1/1"},
		{Vlan: "20", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "ethernet1/2"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseMacTable() = %+v; expected %+v", got, expected)
	}
}
