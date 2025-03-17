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

	a := &fakeDriver{name: "alpha", match: false}
	b := &fakeDriver{name: "beta", match: true}
	Register(a)
	Register(b)

	if Get("missing") != nil {
		t.Fatal("expected nil for missing driver")
	}
	if Get("beta") != b {
		t.Fatal("expected Get(beta) to return registered driver")
	}
	names := Available()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("Available() = %v", names)
	}

	got, err := Detect(nil)
	if err != nil || got != b {
		t.Fatalf("Detect() = %v, %v", got, err)
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
