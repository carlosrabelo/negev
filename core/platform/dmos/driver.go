package dmos

import (
	"fmt"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
)

const driverName = "dmos"

// Driver implements SwitchDriver semantics for Datacom DmOS switches.
type Driver struct{}

// New creates a new DmOS driver.
func New() *Driver {
	return &Driver{}
}

// Name returns the canonical platform identifier.
func (d *Driver) Name() string {
	return driverName
}

// GetAuthenticationSequence returns the DmOS login sequence
// DmOS goes directly to privileged mode after password, no enable command needed
func (d *Driver) GetAuthenticationSequence(username, password, enablePassword string) []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "login:", SendCmd: username + "\n"},
		{WaitFor: "Password:", SendCmd: password + "\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

// Detect determines if the connected device is running DmOS.
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
	lower := strings.ToLower(output)
	return strings.Contains(lower, "dmos") || strings.Contains(lower, "datacom"), nil
}

// GetVLANList retrieves existing VLAN identifiers.
func (d *Driver) GetVLANList(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	// DmOS uses "show vlan table" instead of "show vlan" or "show vlan brief"
	commands := []string{"show vlan table", "show vlan"}
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
		vlans := parseDmOSVLANList(output)
		if len(vlans) > 0 {
			if cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Existing VLANs detected using '%s': %v\n", cmd, vlans)
			}
			return vlans, nil
		}
		if isDmOSCommandError(output) {
			lastErr = fmt.Errorf("command '%s' unsupported by switch", cmd)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to retrieve VLAN list: %w", lastErr)
	}
	return map[string]bool{}, nil
}

// switchportCache stores the cached output to avoid executing slow command twice
var switchportCache struct {
	output string
	target string
}

// GetTrunkInterfaces returns trunk-capable links.
func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error) {
	// Get switchport info (will be cached for GetActivePorts)
	if switchportCache.target != cfg.Target || switchportCache.output == "" {
		output, err := repo.ExecuteCommand("show interfaces switchport")
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve switchport info: %w", err)
		}
		switchportCache.output = output
		switchportCache.target = cfg.Target
	}

	trunks := parseDmOSTrunksFromSwitchport(switchportCache.output)
	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

// GetActivePorts lists access interfaces with link-up state.
func (d *Driver) GetActivePorts(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Port, error) {
	// Get link status
	statusOutput, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface status: %w", err)
	}

	// Reuse cached switchport info if available
	if switchportCache.target != cfg.Target || switchportCache.output == "" {
		output, err := repo.ExecuteCommand("show interfaces switchport")
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve switchport info: %w", err)
		}
		switchportCache.output = output
		switchportCache.target = cfg.Target
	}

	// Parse both outputs
	activePorts := parseDmOSInterfaceStatus(statusOutput)
	vlanMap := parseDmOSSwitchportVLANs(switchportCache.output)

	// Merge VLAN info with active ports
	for i := range activePorts {
		if vlan, ok := vlanMap[activePorts[i].Interface]; ok {
			activePorts[i].Vlan = vlan
		}
	}

	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d active ports\n", len(activePorts))
	}
	return activePorts, nil
}

// GetMacTable fetches the MAC address table.
func (d *Driver) GetMacTable(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo, cfg)
	if err != nil {
		return nil, err
	}
	// DmOS uses "show mac-address-table" without "dynamic"
	output, err := repo.ExecuteCommand("show mac-address-table")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MAC table: %w", err)
	}
	if cfg.IsRawOutputEnabled() {
		fmt.Printf("Raw output of 'show mac-address-table':\n%s\n", output)
	}
	devices := parseDmOSMACTable(output, trunks)
	if cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

// ConfigureAccessCommands returns commands to assign an untagged VLAN to an interface.
func (d *Driver) ConfigureAccessCommands(iface, vlan string) []string {
	port := normalizePort(iface)
	return []string{
		"configure",
		fmt.Sprintf("interface vlan %s", vlan),
		fmt.Sprintf("set-member untagged %s", port),
		"exit",
		fmt.Sprintf("interface %s", port),
		fmt.Sprintf("switchport native vlan %s", vlan),
		"switchport acceptable-frame-type all",
		"exit",
		"end",
	}
}

// CreateVLANCommands ensures the VLAN interface exists (DmOS creates on demand).
func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"configure",
		fmt.Sprintf("interface vlan %s", vlan),
		"exit",
		"end",
	}
}

// DeleteVLANCommands removes a VLAN definition.
func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure",
		fmt.Sprintf("no interface vlan %s", vlan),
		"end",
	}
}

// SaveCommands persists the running configuration.
func (d *Driver) SaveCommands() []string {
	return []string{"copy running-config startup-config", "save"}
}
