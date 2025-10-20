package ios

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

func TestParseIOSVLANList(t *testing.T) {
	output := `VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi1/0/1, Gi1/0/2
10   USERS                            active    Gi1/0/3
20   SERVERS                          active    Gi1/0/4
`
	vlans := parseIOSVLANList(output)
	expected := map[string]bool{"1": true, "10": true, "20": true}
	if !reflect.DeepEqual(vlans, expected) {
		t.Fatalf("unexpected VLAN list: %v", vlans)
	}
}

func TestParseIOSTrunks(t *testing.T) {
	output := `Port        Mode         Encapsulation  Status        Native vlan
Gi1/0/24    on           802.1q         trunking      10
Po1         on           802.1q         trunking      1
`
	got := parseIOSTrunks(output)
	expected := map[string]bool{"gi1/0/24": true, "po1": true}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected trunk map: %v", got)
	}
}

func TestParseIOSInterfaceStatus(t *testing.T) {
	output := `Port      Name               Status       Vlan       Duplex Speed Type
Gi1/0/1                      connected    10         a-full a-100 10/100/1000
Gi1/0/2                      notconnect   1          auto   auto  10/100/1000
Gi1/0/3                      connected    trunk      a-full a-1000 10/100/1000
`
	ports := parseIOSInterfaceStatus(output)
	expected := []entities.Port{
		{Interface: "Gi1/0/1", Vlan: "10"},
		{Interface: "Gi1/0/3", Vlan: "trunk"},
	}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("unexpected active ports: %v", ports)
	}
}

func TestParseIOSMACTable(t *testing.T) {
	output := `   VLAN    MAC Address       Type        Ports
   ----    -----------       --------    -----
   10      0011.2233.4455    DYNAMIC     Gi1/0/1
   20      aa11.bb22.cc33    DYNAMIC     Gi1/0/24
`
	trunks := map[string]bool{"gi1/0/24": true}
	devices := parseIOSMACTable(output, trunks)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	dev := devices[0]
	if dev.Vlan != "10" || dev.Interface != "Gi1/0/1" || dev.Mac != "001122334455" {
		t.Fatalf("unexpected device parsed: %+v", dev)
	}
}
