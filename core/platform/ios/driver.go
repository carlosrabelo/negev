package ios

import (
	"fmt"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
)

const driverName = "ios"

// Driver implements the SwitchDriver behaviour for Cisco IOS switches.
type Driver struct{}

// New creates a new IOS driver instance.
func New() *Driver {
	return &Driver{}
}

// Name returns the canonical platform identifier.
func (d *Driver) Name() string {
	return driverName
}

// Detect inspects the device to determine whether it is running IOS.
func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	if !repo.IsConnected() {
		if err := repo.Connect(); err != nil {
			return false, err
		}
	}
	output, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(output), "cisco ios"), nil
}

// GetVLANList retrieves existing VLANs.
func (d *Driver) GetVLANList(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	commands := []string{"show vlan brief", "show vlan"}
	var lastErr error
	for _, cmd := range commands {
		output, err := repo.ExecuteCommand(cmd)
		if err != nil {
			lastErr = err
			continue
		}
		if cfg.IsRawOutputEnabled() {
			fmt.Printf("Raw output of '%s':\n%s\n", cmd, output)
		}
		vlans := parseIOSVLANList(output)
		if len(vlans) > 0 {
			if cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Existing VLANs detected using '%s': %v\n", cmd, vlans)
			}
			return vlans, nil
		}
		if isIOSCommandError(output) {
			lastErr = fmt.Errorf("command '%s' unsupported by switch", cmd)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to retrieve VLAN list: %w", lastErr)
	}
	return map[string]bool{}, nil
}

// GetTrunkInterfaces retrieves trunk interfaces.
func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	output, err := repo.ExecuteCommand("show interfaces trunk")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve trunk interfaces: %w", err)
	}
	trunks := parseIOSTrunks(output)
	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

// GetActivePorts retrieves interfaces that are currently connected.
func (d *Driver) GetActivePorts(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Port, error) {
	output, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface status: %w", err)
	}
	ports := parseIOSInterfaceStatus(output)
	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d active ports\n", len(ports))
	}
	return ports, nil
}

// GetMacTable retrieves the MAC address table entries.
func (d *Driver) GetMacTable(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo, cfg)
	if err != nil {
		return nil, err
	}
	output, err := repo.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MAC table: %w", err)
	}
	if cfg.IsRawOutputEnabled() {
		fmt.Printf("Raw output of 'show mac address-table dynamic':\n%s\n", output)
	}
	devices := parseIOSMACTable(output, trunks)
	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

// ConfigureAccessCommands returns commands to set an interface as access port.
func (d *Driver) ConfigureAccessCommands(iface, vlan string) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("interface %s", iface),
		"switchport mode access",
		fmt.Sprintf("switchport access vlan %s", vlan),
		"end",
	}
}

// CreateVLANCommands returns commands to create a VLAN and ensure its interface exists.
func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		"exit",
		fmt.Sprintf("interface vlan %s", vlan),
		"no shutdown",
		"end",
	}
}

// DeleteVLANCommands returns commands to delete a VLAN.
func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("interface vlan %s", vlan),
		"shutdown",
		"exit",
		fmt.Sprintf("no interface vlan %s", vlan),
		"exit",
		fmt.Sprintf("no vlan %s", vlan),
		"end",
	}
}

// SaveCommands returns commands that persist the running configuration.
func (d *Driver) SaveCommands() []string {
	return []string{"write memory"}
}
