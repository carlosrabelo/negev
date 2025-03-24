package dm1200

import (
	"errors"
	"testing"
)

type mockRepo struct {
	version string
	sysinfo string
	err     error
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	switch cmd {
	case "show version":
		if m.err != nil {
			return "", m.err
		}
		return m.version, nil
	case "show sysinfo":
		return m.sysinfo, nil
	default:
		return "", errors.New("unexpected command: " + cmd)
	}
}

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dm1200" {
		t.Fatalf("Name() = %q; expected %q", got, "dm1200")
	}
}

func TestDetectDM1200(t *testing.T) {
	d := &Driver{}

	banner1200 := `
DM1200E 24GT + 4GX
Software Version 1.0.0.x
Datacom
`
	match, err := d.Detect(&mockRepo{version: banner1200})
	if err != nil {
		t.Fatalf("Detect(DM1200E) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(DM1200E banner) = false; expected true")
	}

	banner3324 := `
DmSwitch3324F2 Device, Compiled Mar 15 2010
System software version: 13.0.0
Copyright (c) Datacom
`
	match, err = d.Detect(&mockRepo{version: banner3324})
	if err != nil {
		t.Fatalf("Detect(3324F2) error: %v", err)
	}
	if match {
		t.Fatal("Detect(3324F2 banner) = true; expected false")
	}
}

func TestDetectFallsBackToSysinfo(t *testing.T) {
	d := &Driver{}
	repo := &mockRepo{
		err:     errors.New("unrecognized command"),
		sysinfo: "DM1200E 24GT + 4GX\nDatacom\n",
	}
	match, err := d.Detect(repo)
	if err != nil {
		t.Fatalf("Detect(sysinfo fallback) error: %v", err)
	}
	if !match {
		t.Fatal("Detect via show sysinfo = false; expected true")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	wantFirst := "User:"
	if len(seq) != 4 || seq[0].WaitFor != wantFirst {
		t.Fatalf("seq = %+v; expected first WaitFor %q", seq, wantFirst)
	}
	if seq[0].SendCmd != "USERNAME_PLACEHOLDER\n" {
		t.Fatalf("username cmd = %q", seq[0].SendCmd)
	}
	if seq[2].SendCmd != "terminal length 0\n" {
		t.Fatalf("terminal length cmd = %q", seq[2].SendCmd)
	}
	for _, p := range seq {
		if p.SendCmd == "enable\n" {
			t.Fatal("auth sequence must not send enable")
		}
	}
}