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
