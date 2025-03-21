package dmswitch

import (
	"errors"
	"testing"
)

type mockRepo struct {
	target  string
	version string
	err     error
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
func (m *mockRepo) GetTarget() string { return m.target }
func (m *mockRepo) ExecuteCommand(cmd string) (string, error) {
	if cmd == "show version" {
		if m.err != nil {
			return "", m.err
		}
		return m.version, nil
	}
	return "", errors.New("unexpected command: " + cmd)
}

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dmswitch" {
		t.Fatalf("Name() = %q; expected %q", got, "dmswitch")
	}
}

func TestDetectDmSwitch(t *testing.T) {
	d := &Driver{}

	banner3324 := `
DmSwitch3324F2 Device, Compiled Mar 15 2010
System software version: 13.0.0
Copyright (c) Datacom
`
	match, err := d.Detect(&mockRepo{version: banner3324})
	if err != nil {
		t.Fatalf("Detect(3324F2) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(3324F2 banner) = false; expected true")
	}

	banner1200 := `
DM1200E 24GT + 4GX
Software Version 1.0.0.x
Datacom
`
	match, err = d.Detect(&mockRepo{version: banner1200})
	if err != nil {
		t.Fatalf("Detect(DM1200E) error: %v", err)
	}
	if match {
		t.Fatal("Detect(DM1200E banner) = true; expected false")
	}

	bareDatacom := `Copyright (c) Datacom`
	match, err = d.Detect(&mockRepo{version: bareDatacom})
	if err != nil {
		t.Fatalf("Detect(bare datacom) error: %v", err)
	}
	if match {
		t.Fatal("Detect(bare datacom) = true; expected false")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) != 4 {
		t.Fatalf("len(seq) = %d; expected 4", len(seq))
	}
	if seq[0].WaitFor != "Username:" || seq[0].SendCmd != "USERNAME_PLACEHOLDER\n" {
		t.Fatalf("first step = %+v", seq[0])
	}
	if seq[1].WaitFor != "Password:" || seq[1].SendCmd != "PASSWORD_PLACEHOLDER\n" {
		t.Fatalf("second step = %+v", seq[1])
	}
	if seq[2].WaitFor != "#" || seq[2].SendCmd != "terminal length 0\n" {
		t.Fatalf("third step = %+v", seq[2])
	}
	if seq[3].WaitFor != "#" || seq[3].SendCmd != "" {
		t.Fatalf("fourth step = %+v", seq[3])
	}
	for _, p := range seq {
		if p.SendCmd == "enable\n" {
			t.Fatal("auth sequence must not send enable")
		}
	}
}
