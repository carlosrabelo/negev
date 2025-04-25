package services

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type mockRepository struct {
	connected bool
	executed  []string
}

func (m *mockRepository) Connect() error {
	m.connected = true
	return nil
}

func (m *mockRepository) Disconnect() {
	m.connected = false
}

func (m *mockRepository) ExecuteCommand(cmd string) (string, error) {
	m.executed = append(m.executed, cmd)
	return "", nil
}

func (m *mockRepository) IsConnected() bool {
	return m.connected
}

type mockDriver struct{}

func (m *mockDriver) Name() string                                     { return "mock" }
func (m *mockDriver) Detect(repo ports.SwitchRepository) (bool, error) { return true, nil }
func (m *mockDriver) GetAuthenticationSequence() []entities.AuthPrompt { return nil }
func (m *mockDriver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	return []string{"1", "10"}, nil
}
func (m *mockDriver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	return nil, nil
}
func (m *mockDriver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	return []entities.Port{{Interface: "Gi1/0/1", Vlan: "1"}}, nil
}
func (m *mockDriver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	return []entities.Device{{Vlan: "1", Mac: "aabbccddeeff", MacFull: "aa:bb:cc:dd:ee:ff", Interface: "Gi1/0/1"}}, nil
}
func (m *mockDriver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return []string{"switchport access vlan " + vlan}
}
func (m *mockDriver) CreateVLANCommands(vlan string) []string { return nil }
func (m *mockDriver) DeleteVLANCommands(vlan string) []string { return nil }
func (m *mockDriver) SaveCommands() []string {
	return []string{"write memory"}
}
func (m *mockDriver) ClearCache()                       {}
func (m *mockDriver) IsCommandError(output string) bool { return false }

func TestProcessPortsSaveConfiguration(t *testing.T) {
	// 1. Caso com escrita ativa (Sandbox = false), deve salvar
	repo := &mockRepository{}
	drv := &mockDriver{}
	cfg := entities.SwitchConfig{
		Sandbox:     false,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}

	svc := NewVLANService(repo, cfg, drv)
	if err := svc.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	// Verificar se executou o comando "write memory" (save)
	saved := false
	for _, cmd := range repo.executed {
		if cmd == "write memory" {
			saved = true
		}
	}
	if !saved {
		t.Error("expected configuration to be saved via 'write memory', but it was not")
	}

	// 2. Caso com sandbox ativo, não deve salvar
	repoSandbox := &mockRepository{}
	cfgSandbox := entities.SwitchConfig{
		Sandbox:     true,
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}
	svcSandbox := NewVLANService(repoSandbox, cfgSandbox, drv)
	if err := svcSandbox.ProcessPorts(); err != nil {
		t.Fatalf("ProcessPorts failed: %v", err)
	}

	savedSandbox := false
	for _, cmd := range repoSandbox.executed {
		if cmd == "write memory" {
			savedSandbox = true
		}
	}
	if savedSandbox {
		t.Error("expected configuration NOT to be saved in sandbox mode, but it was")
	}
}
