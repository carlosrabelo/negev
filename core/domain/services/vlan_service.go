package services

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
	"github.com/carlosrabelo/negev/core/platform"
)

// VLANServiceImpl implements the VLAN management service
type VLANServiceImpl struct {
	switchRepo ports.SwitchRepository
	config     entities.SwitchConfig
	driver     platform.SwitchDriver
}

// NewVLANService creates a new instance of the VLAN service
func NewVLANService(switchRepo ports.SwitchRepository, config entities.SwitchConfig, driver platform.SwitchDriver) *VLANServiceImpl {
	return &VLANServiceImpl{
		switchRepo: switchRepo,
		config:     config,
		driver:     driver,
	}
}

// ProcessPorts adjusts VLANs for active ports according to configuration
func (v *VLANServiceImpl) ProcessPorts() error {
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Switch %s config DefaultVlan=%s ExcludeMacs=%v ExcludePorts=%v\n", v.config.Target, v.config.DefaultVlan, v.config.ExcludeMacs, v.config.ExcludePorts)
	}

	if !v.switchRepo.IsConnected() {
		if err := v.switchRepo.Connect(); err != nil {
			return err
		}
	}

	existingVLANs, err := v.GetVlanList()
	if err != nil {
		return err
	}

	if v.config.CreateVLANs {
		allowedVLANs := v.getAllowedVLANs()
		if len(allowedVLANs) == 0 {
			fmt.Println("Skipping VLAN sync: no allowed_vlans configured")
		} else {
			for _, vlan := range sortedKeys(allowedVLANs) {
				if !existingVLANs[vlan] {
					fmt.Printf("Creating VLAN %s on switch\n", vlan)
					if err := v.CreateVLAN(vlan); err != nil {
						return err
					}
					existingVLANs[vlan] = true
				}
			}
			// Delete VLANs that exist but are not allowed and not protected
			for vlan := range existingVLANs {
				if !allowedVLANs[vlan] && !v.isProtectedVLAN(vlan) {
					fmt.Printf("Deleting VLAN %s from switch\n", vlan)
					if err := v.DeleteVLAN(vlan); err != nil {
						return err
					}
					delete(existingVLANs, vlan)
				} else if !allowedVLANs[vlan] && v.isProtectedVLAN(vlan) {
					fmt.Printf("Skipping deletion of protected VLAN %s\n", vlan)
				}
			}
		}
	}

	trunks, err := v.GetTrunkInterfaces()
	if err != nil {
		return err
	}

	activePorts, err := v.GetActivePorts()
	if err != nil {
		return err
	}

	if len(activePorts) == 0 {
		fmt.Println("No active ports found on the switch")
		return nil
	}

	devices, err := v.GetMacTable()
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		fmt.Println("No devices found in the MAC address table")
		return nil
	}

	excludedPorts := make(map[string]struct{}, len(v.config.ExcludePorts))
	for _, port := range v.config.ExcludePorts {
		excludedPorts[strings.ToLower(port)] = struct{}{}
	}

	changed := false
	for _, port := range activePorts {
		if _, skip := excludedPorts[strings.ToLower(port.Interface)]; skip {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping excluded port %s\n", port.Interface)
			}
			continue
		}
		if trunks[strings.ToLower(port.Interface)] {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring trunk interface %s\n", port.Interface)
			}
			continue
		}

		portDevices := filterDevices(devices, port.Interface)
		if len(portDevices) == 0 {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping port %s because no device is active\n", port.Interface)
			}
			continue
		}
		if len(portDevices) > 1 {
			log.Printf("Warning: Multiple MACs detected on port %s: %v. Ignoring port to stay safe.", port.Interface, deviceMacs(portDevices))
			continue
		}

		dev := portDevices[0]
		normDevMac := dev.Mac
		if len(normDevMac) < 6 {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring malformed MAC %s on port %s\n", dev.MacFull, port.Interface)
			}
			continue
		}
		if v.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Checking MAC %s on port %s against exclude list %v\n", dev.MacFull, port.Interface, v.config.ExcludeMacs)
		}
		if isExcluded(normDevMac, v.config.ExcludeMacs) {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: MAC %s is excluded, leaving port %s as is\n", dev.MacFull, port.Interface)
			}
			continue
		}

		macPrefix := normDevMac[:6]
		targetVlan := v.config.MacToVlan[macPrefix]
		if targetVlan == "" || targetVlan == "0" || targetVlan == "00" {
			targetVlan = v.config.DefaultVlan
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: No mapping for %s (prefix %s), going with default VLAN %s\n", dev.MacFull, macPrefix, v.config.DefaultVlan)
			}
		} else if v.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: MAC %s (prefix %s) maps to VLAN %s\n", dev.MacFull, macPrefix, targetVlan)
		}

		if !existingVLANs[targetVlan] {
			log.Printf("Error: VLAN %s does not exist on switch, ignoring port %s", targetVlan, port.Interface)
			continue
		}

		if targetVlan != port.Vlan {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Changing %s from VLAN %s to %s\n", port.Interface, port.Vlan, targetVlan)
			}
			v.ConfigureVlan(port.Interface, targetVlan)
			changed = true
		} else if v.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Port %s already sits in VLAN %s\n", port.Interface, targetVlan)
		}
	}

	fmt.Printf("State before saving: Sandbox=%v Changed=%v\n", v.config.Sandbox, changed)
	if !v.config.Sandbox && changed {
		v.saveConfiguration()
	} else {
		if !changed {
			fmt.Println("No changes required")
		} else if v.config.Sandbox {
			fmt.Println("Changes simulated (sandbox mode enabled, use -w to apply)")
		}
	}
	return nil
}

// GetVlanList gets the list of existing VLANs
func (v *VLANServiceImpl) GetVlanList() (map[string]bool, error) {
	vlans, err := v.driver.GetVLANList(v.switchRepo, v.config)
	if err != nil {
		return nil, err
	}
	if len(vlans) == 0 {
		fmt.Println("Warning: No VLANs found on the switch. Maybe create them first.")
	}
	return vlans, nil
}

// GetTrunkInterfaces gets trunk interfaces
func (v *VLANServiceImpl) GetTrunkInterfaces() (map[string]bool, error) {
	return v.driver.GetTrunkInterfaces(v.switchRepo, v.config)
}

// GetActivePorts gets active ports
func (v *VLANServiceImpl) GetActivePorts() ([]entities.Port, error) {
	return v.driver.GetActivePorts(v.switchRepo, v.config)
}

// GetMacTable gets MAC address table
func (v *VLANServiceImpl) GetMacTable() ([]entities.Device, error) {
	return v.driver.GetMacTable(v.switchRepo, v.config)
}

// ConfigureVlan configures VLAN on an interface
func (v *VLANServiceImpl) ConfigureVlan(iface, vlan string) {
	commands := v.driver.ConfigureAccessCommands(iface, vlan)
	if v.config.Sandbox {
		fmt.Printf("SANDBOX: Simulating configuration for %s to VLAN %s\n", iface, vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return
	}
	for _, cmd := range commands {
		if _, err := v.switchRepo.ExecuteCommand(cmd); err != nil {
			log.Printf("Error executing %s: %v", cmd, err)
		}
	}
	fmt.Printf("Configured %s to VLAN %s\n", iface, vlan)
}

// CreateVLAN creates a new VLAN
func (v *VLANServiceImpl) CreateVLAN(vlan string) error {
	commands := v.driver.CreateVLANCommands(vlan)
	if v.config.Sandbox {
		fmt.Printf("SANDBOX: Simulating creation of VLAN %s\n", vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return nil
	}
	for _, cmd := range commands {
		if _, err := v.switchRepo.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to create VLAN %s: %v", vlan, err)
		}
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Created VLAN %s with interface\n", vlan)
	}
	return nil
}

// DeleteVLAN deletes an existing VLAN
func (v *VLANServiceImpl) DeleteVLAN(vlan string) error {
	commands := v.driver.DeleteVLANCommands(vlan)
	if v.config.Sandbox {
		fmt.Printf("SANDBOX: Simulating deletion of VLAN %s\n", vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return nil
	}
	for _, cmd := range commands {
		if _, err := v.switchRepo.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to delete VLAN %s: %v", vlan, err)
		}
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Deleted VLAN %s with interface\n", vlan)
	}
	return nil
}

func (v *VLANServiceImpl) saveConfiguration() {
	commands := v.driver.SaveCommands()
	for idx, cmd := range commands {
		if v.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Saving configuration using '%s'\n", cmd)
		}
		if _, err := v.switchRepo.ExecuteCommand(cmd); err != nil {
			log.Printf("Error saving configuration with '%s': %v", cmd, err)
			if idx == len(commands)-1 {
				fmt.Println("Warning: Unable to persist configuration automatically; please save manually.")
			}
			continue
		}
		fmt.Printf("Configuration saved using '%s'\n", cmd)
		return
	}
}

// Helper functions
func (v *VLANServiceImpl) getAllowedVLANs() map[string]bool {
	allowedVLANs := make(map[string]bool)
	for _, vlan := range v.config.AllowedVlans {
		allowedVLANs[vlan] = true
	}
	return allowedVLANs
}

func (v *VLANServiceImpl) getProtectedVLANs() map[string]bool {
	protectedVLANs := make(map[string]bool)

	// Add user-defined protected VLANs
	for _, vlan := range v.config.ProtectedVlans {
		protectedVLANs[vlan] = true
	}

	return protectedVLANs
}

func (v *VLANServiceImpl) isProtectedVLAN(vlan string) bool {
	// Check user-defined protected VLANs
	for _, protected := range v.config.ProtectedVlans {
		if vlan == protected {
			return true
		}
	}

	// Check if VLAN is in extended range (1000-4094)
	if vlanNum, err := strconv.Atoi(vlan); err == nil {
		if vlanNum >= 1000 && vlanNum <= 4094 {
			return true
		}
	}

	return false
}

func filterDevices(devices []entities.Device, port string) []entities.Device {
	filtered := make([]entities.Device, 0, len(devices))
	for _, dev := range devices {
		if strings.EqualFold(dev.Interface, port) {
			filtered = append(filtered, dev)
		}
	}
	return filtered
}

func deviceMacs(devices []entities.Device) []string {
	macs := make([]string, 0, len(devices))
	for _, dev := range devices {
		macs = append(macs, dev.MacFull)
	}
	return macs
}

func isExcluded(mac string, excluded []string) bool {
	for _, candidate := range excluded {
		if mac == candidate {
			return true
		}
	}
	return false
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
