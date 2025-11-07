package services

import (
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
)

// MockVLANService implements the VLANService port for testing
type MockVLANService struct {
	processPortsCalled bool
	processPortsError  error
}

func (m *MockVLANService) ProcessPorts() error {
	m.processPortsCalled = true
	return m.processPortsError
}

func (m *MockVLANService) GetVlanList() (map[string]bool, error) {
	return map[string]bool{"10": true, "20": true}, nil
}

func (m *MockVLANService) GetTrunkInterfaces() (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (m *MockVLANService) GetActivePorts() ([]entities.Port, error) {
	return []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}, nil
}

func (m *MockVLANService) GetMacTable() ([]entities.Device, error) {
	return []entities.Device{{Vlan: "10", Mac: "001122", MacFull: "00:11:22:33:44:55", Interface: "Gi1/0/1"}}, nil
}

func (m *MockVLANService) ConfigureVlan(iface, vlan string) {
	// Mock implementation
}

func (m *MockVLANService) CreateVLAN(vlan string) error {
	return nil
}

func (m *MockVLANService) DeleteVLAN(vlan string) error {
	return nil
}

// MockTransportClient implements the transport.Client interface for testing
type MockTransportClient struct {
	connected     bool
	connectError  error
	executedCmds  []string
	cmdResponses  map[string]string
	cmdErrors     map[string]error
}

func (m *MockTransportClient) Connect() error {
	if m.connectError != nil {
		return m.connectError
	}
	m.connected = true
	return nil
}

func (m *MockTransportClient) Disconnect() {
	m.connected = false
}

func (m *MockTransportClient) ExecuteCommand(cmd string) (string, error) {
	m.executedCmds = append(m.executedCmds, cmd)
	if m.cmdErrors != nil {
		if err, exists := m.cmdErrors[cmd]; exists {
			return "", err
		}
	}
	if m.cmdResponses != nil {
		if resp, exists := m.cmdResponses[cmd]; exists {
			return resp, nil
		}
	}
	return "mock response", nil
}

func (m *MockTransportClient) IsConnected() bool {
	return m.connected
}

// MockSwitchDriver implements the platform.SwitchDriver interface for testing
type MockSwitchDriver struct {
	name string
}

func (m *MockSwitchDriver) Name() string {
	return m.name
}

func (m *MockSwitchDriver) Detect(repo ports.SwitchRepository) (bool, error) {
	return true, nil
}

func (m *MockSwitchDriver) GetVLANList(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	return map[string]bool{"10": true, "20": true}, nil
}

func (m *MockSwitchDriver) GetTrunkInterfaces(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (m *MockSwitchDriver) GetActivePorts(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Port, error) {
	return []entities.Port{{Interface: "Gi1/0/1", Vlan: "10"}}, nil
}

func (m *MockSwitchDriver) GetMacTable(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Device, error) {
	return []entities.Device{{Vlan: "10", Mac: "001122", MacFull: "00:11:22:33:44:55", Interface: "Gi1/0/1"}}, nil
}

func (m *MockSwitchDriver) ConfigureAccessCommands(iface, vlan string) []string {
	return []string{"interface " + iface, "switchport access vlan " + vlan}
}

func (m *MockSwitchDriver) CreateVLANCommands(vlan string) []string {
	return []string{"vlan " + vlan, "name VLAN" + vlan}
}

func (m *MockSwitchDriver) DeleteVLANCommands(vlan string) []string {
	return []string{"no vlan " + vlan}
}

func (m *MockSwitchDriver) SaveCommands() []string {
	return []string{"write memory"}
}

func (m *MockSwitchDriver) GetAuthenticationSequence(username, password, enablePassword string) []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: username},
		{WaitFor: "Password:", SendCmd: password},
		{WaitFor: ">", SendCmd: "enable"},
		{WaitFor: "Password:", SendCmd: enablePassword},
	}
}

func TestNewVLANApplicationService(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Platform:       "ios",
		Transport:      "telnet",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	mockClient := &MockTransportClient{}
	mockDriver := &MockSwitchDriver{name: "ios"}

	service := NewVLANApplicationService(config, mockClient, mockDriver)

	if service == nil {
		t.Fatal("NewVLANApplicationService() returned nil")
	}

	if service.vlanService == nil {
		t.Fatal("VLANApplicationService.vlanService is nil")
	}
}

func TestVLANApplicationService_ProcessPorts_Success(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Platform:       "ios",
		Transport:      "telnet",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	mockClient := &MockTransportClient{}
	mockDriver := &MockSwitchDriver{name: "ios"}

	service := NewVLANApplicationService(config, mockClient, mockDriver)

	// Replace the internal VLAN service with our mock
	mockVLANService := &MockVLANService{}
	service.vlanService = mockVLANService

	err := service.ProcessPorts()
	if err != nil {
		t.Fatalf("ProcessPorts() returned error: %v", err)
	}

	if !mockVLANService.processPortsCalled {
		t.Error("ProcessPorts() did not call the underlying VLAN service")
	}
}

func TestVLANApplicationService_ProcessPorts_Error(t *testing.T) {
	config := entities.SwitchConfig{
		Target:         "192.168.1.1",
		Platform:       "ios",
		Transport:      "telnet",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	mockClient := &MockTransportClient{}
	mockDriver := &MockSwitchDriver{name: "ios"}

	service := NewVLANApplicationService(config, mockClient, mockDriver)

	// Replace the internal VLAN service with our mock that returns an error
	mockVLANService := &MockVLANService{
		processPortsError: &testError{"mock error"},
	}
	service.vlanService = mockVLANService

	err := service.ProcessPorts()
	if err == nil {
		t.Fatal("ProcessPorts() should have returned an error")
	}

	if !mockVLANService.processPortsCalled {
		t.Error("ProcessPorts() did not call the underlying VLAN service")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}