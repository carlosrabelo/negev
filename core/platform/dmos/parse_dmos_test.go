package dmos

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

func TestParseDmOSVLANList(t *testing.T) {
	output := `VLAN Name          Type
1    default       active
10   users         active
20   guests        active
`
	vlans := parseDmOSVLANList(output)
	expected := map[string]bool{"1": true, "10": true, "20": true}
	if !reflect.DeepEqual(vlans, expected) {
		t.Fatalf("unexpected VLAN list: %v", vlans)
	}
}

func TestParseDmOSTrunks(t *testing.T) {
	output := `VLAN   Tagged Ports             Untagged Ports
-----  -----------------------  ----------------------
10     ethernet 1/24            ethernet 1/1
20     ethernet 1/24
`
	got := parseDmOSTrunks(output)
	expected := map[string]bool{"ethernet 1/24": true}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected trunk map: %v", got)
	}
}

func TestParseDmOSInterfaceStatus(t *testing.T) {
	output := `Interface          Status  Mode    Native VLAN
ethernet 1/1       up      access  10
ethernet 1/2       down    access  1
ethernet 1/24      up      trunk   10
`
	ports := parseDmOSInterfaceStatus(output)
	expected := []entities.Port{
		{Interface: "ethernet 1/1", Vlan: "10"},
		{Interface: "ethernet 1/24", Vlan: "10"},
	}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("unexpected active ports: %v", ports)
	}
}

func TestParseDmOSMACTable(t *testing.T) {
	output := `VLAN  MAC Address       Type     Port
10    0011.2233.4455    dynamic  ethernet 1/1
20    aa11.bb22.cc33    dynamic  ethernet 1/24
`
	trunks := map[string]bool{"ethernet 1/24": true}
	devices := parseDmOSMACTable(output, trunks)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	dev := devices[0]
	if dev.Vlan != "10" || dev.Interface != "ethernet 1/1" || dev.Mac != "001122334455" {
		t.Fatalf("unexpected device parsed: %+v", dev)
	}
}
