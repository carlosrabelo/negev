package dmos

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

func TestParseDmOSVLANList(t *testing.T) {
	// Test "show vlan table" format
	output := `VLAN 1 [DefaultVlan]: static, active
VLAN 10: static, active
VLAN 20: static, active
VLAN 100: static, active
`
	vlans := parseDmOSVLANList(output)
	expected := map[string]bool{"1": true, "10": true, "20": true, "100": true}
	if !reflect.DeepEqual(vlans, expected) {
		t.Fatalf("unexpected VLAN list: got %v, expected %v", vlans, expected)
	}

	// Test simple format (fallback)
	output2 := `VLAN Name          Type
1    default       active
10   users         active
20   guests        active
`
	vlans2 := parseDmOSVLANList(output2)
	expected2 := map[string]bool{"1": true, "10": true, "20": true}
	if !reflect.DeepEqual(vlans2, expected2) {
		t.Fatalf("unexpected VLAN list for simple format: got %v, expected %v", vlans2, expected2)
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

func TestParseDmOSTrunksFromSwitchport(t *testing.T) {
	output := `Information of Eth 1/1:
 Allowed VLANs:                 12(s,u)
Information of Eth 1/11:
 Allowed VLANs:                 12(s,u),    30(s,u)
Information of Eth 1/25:
 Allowed VLANs:                  1(s,t),    10(s,t),    11(s,t),    12(s,t),
                                20(s,t),    21(s,t),    22(s,t),    30(s,t)
 Forbidden VLANs:
Information of Eth 1/26:
 Allowed VLANs:                  1(s,t),    10(s,t),    11(s,t)
`
	trunks := parseDmOSTrunksFromSwitchport(output)
	expected := map[string]bool{
		"ethernet 1/25": true,
		"ethernet 1/26": true,
	}
	if !reflect.DeepEqual(trunks, expected) {
		t.Fatalf("unexpected trunk map: got %v, expected %v", trunks, expected)
	}
}

func TestParseDmOSSwitchportVLANs(t *testing.T) {
	output := `Information of Eth 1/1:
 Native VLAN:                   12
Information of Eth 1/4:
 Native VLAN:                   10
Information of Eth 1/25:
 Native VLAN:                   1
`
	vlanMap := parseDmOSSwitchportVLANs(output)
	expected := map[string]string{
		"ethernet 1/1":  "12",
		"ethernet 1/4":  "10",
		"ethernet 1/25": "1",
	}
	if !reflect.DeepEqual(vlanMap, expected) {
		t.Fatalf("unexpected VLAN map: got %v, expected %v", vlanMap, expected)
	}
}

func TestParseDmOSInterfaceStatus(t *testing.T) {
	output := `Information of  Eth 1/1
 Current status:
  Link status:            Down

Information of  Eth 1/4
 Current status:
  Link status:            Up
  Operation speed-duplex: 10M full

Information of  Eth 1/25
 Current status:
  Link status:            Up
  Operation speed-duplex: 1000M full
`
	ports := parseDmOSInterfaceStatus(output)
	expected := []entities.Port{
		{Interface: "ethernet 1/4", Vlan: ""},
		{Interface: "ethernet 1/25", Vlan: ""},
	}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("unexpected active ports: got %v, expected %v", ports, expected)
	}
}

func TestParseDmOSMACTable(t *testing.T) {
	// DmOS format
	output := `Unit Block Interface MAC Address       VLAN VPN Type
   1       Eth  1/4  5E:15:F4:01:9A:57   10   - Learned
   1       Eth  1/25 B0:7D:47:CE:A2:AF  201   - Learned
   1       Eth  1/6  18:0D:2C:0D:B3:0A   32   - Learned
`
	trunks := map[string]bool{"ethernet 1/25": true}
	devices := parseDmOSMACTable(output, trunks)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %+v", len(devices), devices)
	}

	// Check first device
	dev1 := devices[0]
	if dev1.Vlan != "10" || dev1.Interface != "ethernet 1/4" || dev1.Mac != "5e15f4019a57" {
		t.Fatalf("unexpected device[0]: %+v", dev1)
	}

	// Check second device
	dev2 := devices[1]
	if dev2.Vlan != "32" || dev2.Interface != "ethernet 1/6" || dev2.Mac != "180d2c0db30a" {
		t.Fatalf("unexpected device[1]: %+v", dev2)
	}
}
