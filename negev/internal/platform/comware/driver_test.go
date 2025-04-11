package comware

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	responses map[string]string
	err       error
}

func (m *mockRepo) Connect() error                                    { return nil }
func (m *mockRepo) Disconnect()                                       {}
func (m *mockRepo) IsConnected() bool                                 { return true }
func (m *mockRepo) SetSessionProfile(profile entities.SessionProfile) {}
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.responses != nil {
		if out, ok := m.responses[cmd]; ok {
			return out, nil
		}
	}
	return "", errors.New("unexpected command: " + cmd)
}

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "comware" {
		t.Fatalf("Name() = %q; expected comware", got)
	}
}

func TestDetectComware(t *testing.T) {
	banner := `
3Com Switch 4210 26-Port
Software Version 3.10 Release 2212P01
Copyright (c) 2004-2010 3Com Corp.
`
	match, err := (&Driver{}).Detect(&mockRepo{responses: map[string]string{
		"display version": banner,
	}})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !match {
		t.Fatal("Detect(3Com banner) = false; expected true")
	}

	match, err = (&Driver{}).Detect(&mockRepo{responses: map[string]string{
		"display version": "Cisco IOS Software",
	}})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if match {
		t.Fatal("Detect(IOS banner) = true; expected false")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) < 3 || seq[0].WaitFor != "Username:" {
		t.Fatalf("seq = %+v", seq)
	}
	for _, p := range seq {
		if p.SendCmd == "enable\n" {
			t.Fatal("Comware auth must not send enable")
		}
	}
}

func TestNormalizeInterface(t *testing.T) {
	d := &Driver{}
	cases := map[string]string{
		"Eth1/0/1":              "Ethernet1/0/1",
		"Ethernet1/0/24":        "Ethernet1/0/24",
		"GE1/0/25":              "GigabitEthernet1/0/25",
		"GigabitEthernet1/0/26": "GigabitEthernet1/0/26",
		"TENGE1/0/1":            "Ten-GigabitEthernet1/0/1",
	}
	for in, want := range cases {
		if got := d.NormalizeInterface(in); got != want {
			t.Errorf("NormalizeInterface(%q) = %q; expected %q", in, got, want)
		}
	}
}

func TestParseVLANs(t *testing.T) {
	output := `
 Total 26 VLAN exist(s).
 The following VLANs exist:
  1(default), 10-12, 20-22, 30-32, 40, 50, 60, 100-106, 201-206,
`
	got := parseVLANs(output)
	if len(got) != 26 {
		t.Fatalf("parseVLANs len = %d; expected 26; got %v", len(got), got)
	}
	wantFirst := []string{"1", "10", "11", "12", "20"}
	for i, w := range wantFirst {
		if got[i] != w {
			t.Fatalf("parseVLANs()[%d] = %q; expected %q (full=%v)", i, got[i], w, got)
		}
	}
}

func TestParseTrunkInterfaces(t *testing.T) {
	output := `
 The following trunk ports exist:
  Ethernet1/0/24           GigabitEthernet1/0/25    GigabitEthernet1/0/26    
`
	got := parseTrunkInterfaces(output)
	want := []string{"Ethernet1/0/24", "GigabitEthernet1/0/25", "GigabitEthernet1/0/26"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTrunkInterfaces() = %v; expected %v", got, want)
	}
}

func TestParseActivePorts(t *testing.T) {
	output := `
Interface   Link     Speed  Duplex Type   PVID Description                 
--------------------------------------------------------------------------------
Aux1/0/0    UP       --     --     --     --   
Eth1/0/1    DOWN     A      A      access 100  
Eth1/0/5    UP       A      A      access 12   
Eth1/0/10   UP       A      A      access 10   
Eth1/0/24   DOWN     A      A      trunk  1    
GE1/0/25    UP       A1000M Afull  trunk  1    
GE1/0/26    DOWN     A      A      trunk  1    
GE1/0/27    ADM DOWN A1000M Afull  trunk  1    
NULL0       UP       --     --     --     --   
Vlan50      UP       --     --     --     --   
`
	got := parseActivePorts(output)
	want := []entities.Port{
		{Interface: "Ethernet1/0/5", Vlan: "12"},
		{Interface: "Ethernet1/0/10", Vlan: "10"},
		{Interface: "GigabitEthernet1/0/25", Vlan: "1", IsTrunk: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseActivePorts() = %+v; expected %+v", got, want)
	}
}

func TestParseMacTable(t *testing.T) {
	output := `
MAC ADDR        VLAN ID   STATE          PORT INDEX               AGING TIME(s)
1c2a-a32e-f757  1         Learned        GigabitEthernet1/0/25    AGING
3464-a900-d471  10        Learned        Ethernet1/0/5            AGING
`
	trunkSet := map[string]bool{"GigabitEthernet1/0/25": true}
	got := parseMacTable(output, trunkSet)
	want := []entities.Device{
		{Vlan: "10", Mac: "3464a900d471", MacFull: "34:64:a9:00:d4:71", Interface: "Ethernet1/0/5"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMacTable() = %+v; expected %+v", got, want)
	}
}

func TestConfigureAccessCommands(t *testing.T) {
	got := (&Driver{}).ConfigureAccessCommands(entities.Port{Interface: "Ethernet1/0/5"}, "10")
	want := []string{
		"system-view",
		"interface Ethernet1/0/5",
		"port link-type access",
		"port access vlan 10",
		"quit",
		"return",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigureAccessCommands() = %v; expected %v", got, want)
	}
}

func TestCreateVLANCommands(t *testing.T) {
	got := (&Driver{}).CreateVLANCommands("20")
	want := []string{
		"system-view",
		"vlan 20",
		"quit",
		"return",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateVLANCommands() = %v; expected %v", got, want)
	}
}

func TestDeleteVLANCommands(t *testing.T) {
	got := (&Driver{}).DeleteVLANCommands("20")
	want := []string{
		"system-view",
		"undo vlan 20",
		"return",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteVLANCommands() = %v; expected %v", got, want)
	}
	if got := (&Driver{}).DeleteVLANCommands("1"); got != nil {
		t.Fatalf("DeleteVLANCommands(1) = %v; expected nil", got)
	}
}

func TestSaveCommands(t *testing.T) {
	got := (&Driver{}).SaveCommands()
	want := []string{"save\ny\n\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCommands() = %v; expected %v", got, want)
	}
}

func TestIsComwareCommandError(t *testing.T) {
	d := &Driver{}
	if !d.IsCommandError("% Unrecognized command found at '^' position.") {
		t.Fatal("expected unrecognized command to be an error")
	}
	if !d.IsCommandError("% Wrong parameter found at '^' position.") {
		t.Fatal("expected wrong parameter to be an error")
	}
	if d.IsCommandError("Packets invalid: 0") {
		t.Fatal("must not treat a counter line as a command error")
	}
}

func TestGetRunningConfig(t *testing.T) {
	repo := &mockRepo{responses: map[string]string{
		"display current-configuration": "# comware cfg\nsysname SW1\n",
	}}
	got, err := (&Driver{}).GetRunningConfig(repo)
	if err != nil {
		t.Fatalf("GetRunningConfig: %v", err)
	}
	if !strings.Contains(got, "sysname SW1") {
		t.Fatalf("unexpected config: %q", got)
	}
}
