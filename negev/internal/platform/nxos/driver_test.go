package nxos

import (
	"errors"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type mockRepo struct {
	version string
	err     error
}

func (m *mockRepo) Connect() error    { return nil }
func (m *mockRepo) Disconnect()       {}
func (m *mockRepo) IsConnected() bool { return true }
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
	d := &Driver{}
	if got := d.Name(); got != "nxos" {
		t.Fatalf("Name() = %q; expected %q", got, "nxos")
	}
}

func TestDetectNXOS(t *testing.T) {
	d := &Driver{}
	nxosBanner := `
Cisco Nexus Operating System (NX-OS) Software
Software
  NXOS: version 9.3(8)
Hardware
  cisco Nexus9000 C93180YC-EX chassis
`
	match, err := d.Detect(&mockRepo{version: nxosBanner})
	if err != nil {
		t.Fatalf("Detect(nxos) error: %v", err)
	}
	if !match {
		t.Fatal("Detect(nxos banner) = false; expected true")
	}

	iosBanner := `Cisco IOS Software, C2960 Software (C2960-LANBASEK9-M), Version 15.0(2)SE`
	match, err = d.Detect(&mockRepo{version: iosBanner})
	if err != nil {
		t.Fatalf("Detect(ios) error: %v", err)
	}
	if match {
		t.Fatal("Detect(ios banner) = true; expected false")
	}
}

func TestAuthenticationSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	want := []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
	if len(seq) != len(want) {
		t.Fatalf("len(seq) = %d; expected %d", len(seq), len(want))
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("seq[%d] = %+v; expected %+v", i, seq[i], want[i])
		}
	}
	for _, p := range seq {
		if strings.Contains(p.SendCmd, "enable") {
			t.Fatalf("auth sequence must not send enable; got %q", p.SendCmd)
		}
	}
}
