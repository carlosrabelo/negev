package entities

import "testing"

func TestDeviceAndPortFields(t *testing.T) {
	d := Device{
		Vlan:      "10",
		Mac:       "001122334455",
		MacFull:   "00:11:22:33:44:55",
		Interface: "GigabitEthernet1/0/1",
		IsStatic:  true,
	}
	if !d.IsStatic {
		t.Fatal("Device.IsStatic should be settable")
	}

	p := Port{
		Interface: "GigabitEthernet1/0/2",
		Vlan:      "routed",
		IsRouted:  true,
		IsTrunk:   false,
	}
	if !p.IsRouted || p.IsTrunk {
		t.Fatalf("Port flags = routed=%v trunk=%v", p.IsRouted, p.IsTrunk)
	}

	trunk := Port{Interface: "GigabitEthernet1/0/24", Vlan: "trunk", IsTrunk: true}
	if !trunk.IsTrunk || trunk.IsRouted {
		t.Fatalf("trunk Port flags = routed=%v trunk=%v", trunk.IsRouted, trunk.IsTrunk)
	}
}
