package platform

import (
	"errors"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type fakeDriver struct {
	name    string
	match   bool
	detectE error
}

func (f *fakeDriver) Name() string { return f.name }
func (f *fakeDriver) Detect(repo ports.SwitchRepository) (bool, error) {
	return f.match, f.detectE
}
func (f *fakeDriver) GetSessionProfile() entities.SessionProfile       { return entities.SessionProfile{} }
func (f *fakeDriver) GetAuthenticationSequence() []entities.AuthPrompt { return nil }
func (f *fakeDriver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}
func (f *fakeDriver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}
func (f *fakeDriver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	return nil, nil
}
func (f *fakeDriver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	return nil, nil
}
func (f *fakeDriver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}
func (f *fakeDriver) GetRunningConfig(repo ports.SwitchRepository) (string, error) {
	return "", nil
}
func (f *fakeDriver) NormalizeInterface(name string) string { return name }
func (f *fakeDriver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return nil
}
func (f *fakeDriver) CreateVLANCommands(vlan string) []string { return nil }
func (f *fakeDriver) DeleteVLANCommands(vlan string) []string { return nil }
func (f *fakeDriver) SaveCommands() []string                  { return nil }
func (f *fakeDriver) ClearCache()                             {}
func (f *fakeDriver) IsCommandError(output string) bool       { return false }

func TestRegisterGetAvailableDetect(t *testing.T) {
	prev := drivers
	t.Cleanup(func() { drivers = prev })
	drivers = nil

	b := &fakeDriver{name: "beta", match: true}
	a := &fakeDriver{name: "alpha", match: false}
	Register(b)
	Register(a)

	if Get("missing") != nil {
		t.Fatal("expected nil for missing driver")
	}
	if Get("beta") != b {
		t.Fatal("expected Get(beta) to return registered driver")
	}
	names := Available()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("Available() = %v; expected alphabetical order", names)
	}

	got, err := Detect(nil)
	if err != nil || got != b {
		t.Fatalf("Detect() = %v, %v; expected beta (first alphabetical match)", got, err)
	}

	drivers = nil
	zeta := &fakeDriver{name: "zeta", match: true}
	alpha := &fakeDriver{name: "alpha", match: true}
	Register(zeta)
	Register(alpha)
	got, err = Detect(nil)
	if err != nil || got != alpha {
		t.Fatalf("Detect alphabetical = %v, %v; expected alpha before zeta", got, err)
	}

	drivers = nil
	Register(&fakeDriver{name: "bad", detectE: errors.New("boom")})
	if _, err := Detect(nil); err == nil {
		t.Fatal("expected detect error")
	}

	drivers = nil
	Register(&fakeDriver{name: "none", match: false})
	if _, err := Detect(nil); err == nil {
		t.Fatal("expected no matching driver")
	}
}
