package entities

import (
	"testing"
)

func TestDevice_Creation(t *testing.T) {
	device := Device{
		Vlan:      "10",
		Mac:       "aabbcc",
		MacFull:   "aa:bb:cc:dd:ee:ff",
		Interface: "Gi1/0/1",
	}

	if device.Vlan != "10" {
		t.Errorf("Expected VLAN '10', got '%s'", device.Vlan)
	}

	if device.Mac != "aabbcc" {
		t.Errorf("Expected MAC 'aabbcc', got '%s'", device.Mac)
	}

	if device.MacFull != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Expected full MAC 'aa:bb:cc:dd:ee:ff', got '%s'", device.MacFull)
	}

	if device.Interface != "Gi1/0/1" {
		t.Errorf("Expected interface 'Gi1/0/1', got '%s'", device.Interface)
	}
}

func TestDevice_ZeroValues(t *testing.T) {
	var device Device

	if device.Vlan != "" {
		t.Errorf("Expected empty VLAN, got '%s'", device.Vlan)
	}

	if device.Mac != "" {
		t.Errorf("Expected empty MAC, got '%s'", device.Mac)
	}

	if device.MacFull != "" {
		t.Errorf("Expected empty full MAC, got '%s'", device.MacFull)
	}

	if device.Interface != "" {
		t.Errorf("Expected empty interface, got '%s'", device.Interface)
	}
}

func TestPort_Creation(t *testing.T) {
	port := Port{
		Interface: "Gi1/0/1",
		Vlan:      "20",
	}

	if port.Interface != "Gi1/0/1" {
		t.Errorf("Expected interface 'Gi1/0/1', got '%s'", port.Interface)
	}

	if port.Vlan != "20" {
		t.Errorf("Expected VLAN '20', got '%s'", port.Vlan)
	}
}

func TestPort_ZeroValues(t *testing.T) {
	var port Port

	if port.Interface != "" {
		t.Errorf("Expected empty interface, got '%s'", port.Interface)
	}

	if port.Vlan != "" {
		t.Errorf("Expected empty VLAN, got '%s'", port.Vlan)
	}
}

func TestAuthPrompt_Creation(t *testing.T) {
	prompt := AuthPrompt{
		WaitFor: "Username:",
		SendCmd: "admin",
	}

	if prompt.WaitFor != "Username:" {
		t.Errorf("Expected wait for 'Username:', got '%s'", prompt.WaitFor)
	}

	if prompt.SendCmd != "admin" {
		t.Errorf("Expected send command 'admin', got '%s'", prompt.SendCmd)
	}
}

func TestAuthPrompt_EmptySendCmd(t *testing.T) {
	prompt := AuthPrompt{
		WaitFor: "Password:",
		SendCmd: "",
	}

	if prompt.WaitFor != "Password:" {
		t.Errorf("Expected wait for 'Password:', got '%s'", prompt.WaitFor)
	}

	if prompt.SendCmd != "" {
		t.Errorf("Expected empty send command, got '%s'", prompt.SendCmd)
	}
}

func TestAuthPrompt_ZeroValues(t *testing.T) {
	var prompt AuthPrompt

	if prompt.WaitFor != "" {
		t.Errorf("Expected empty wait for, got '%s'", prompt.WaitFor)
	}

	if prompt.SendCmd != "" {
		t.Errorf("Expected empty send command, got '%s'", prompt.SendCmd)
	}
}