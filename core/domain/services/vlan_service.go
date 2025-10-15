package services

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
)

// VLANServiceImpl implements the VLAN management service
type VLANServiceImpl struct {
	switchRepo ports.SwitchRepository
	config     entities.SwitchConfig
}

// NewVLANService creates a new instance of the VLAN service
func NewVLANService(switchRepo ports.SwitchRepository, config entities.SwitchConfig) *VLANServiceImpl {
	return &VLANServiceImpl{
		switchRepo: switchRepo,
		config:     config,
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
		if trunks[port.Interface] {
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
		normDevMac := normalizeMAC(dev.MacFull)
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

		if !v.config.SkipVlanCheck && !existingVLANs[targetVlan] {
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
		if _, err := v.switchRepo.ExecuteCommand("write memory"); err != nil {
			log.Printf("Error saving configuration: %v", err)
		} else {
			fmt.Println("Configuration saved")
		}
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
	output, err := v.switchRepo.ExecuteCommand("show vlan brief")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve VLAN list: %v", err)
	}
	re := regexp.MustCompile(`(?m)^(\d+)\s+\S+`)
	matches := re.FindAllStringSubmatch(output, -1)
	vlans := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			vlans[match[1]] = true
		}
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Existing VLANs: %v\n", vlans)
	}
	if len(vlans) == 0 {
		fmt.Println("Warning: No VLANs found on the switch. Maybe create them first.")
	}
	return vlans, nil
}

// GetTrunkInterfaces gets trunk interfaces
func (v *VLANServiceImpl) GetTrunkInterfaces() (map[string]bool, error) {
	output, err := v.switchRepo.ExecuteCommand("show interfaces trunk")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve trunk interfaces: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z]+\d+\/\d+(?:\/\d+)?)`)
	matches := re.FindAllStringSubmatch(output, -1)
	trunks := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			trunks[match[1]] = true
		}
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

// GetActivePorts gets active ports
func (v *VLANServiceImpl) GetActivePorts() ([]entities.Port, error) {
	output, err := v.switchRepo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface status: %v", err)
	}
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+(?:\/\d+)?)\s+(?:[^\s]*\s+)?connected\s+(\d+|trunk)\s+.*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	ports := make([]entities.Port, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			log.Printf("Warning: Ignoring malformed interface line: %s", match[0])
			continue
		}
		if !strings.Contains(match[0], "connected") {
			log.Printf("Warning: Line %s does not show connected status, ignoring", match[0])
			continue
		}
		ports = append(ports, entities.Port{Interface: match[1], Vlan: match[2]})
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d active ports\n", len(ports))
	}
	return ports, nil
}

// GetMacTable gets MAC address table
func (v *VLANServiceImpl) GetMacTable() ([]entities.Device, error) {
	output, err := v.switchRepo.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MAC table: %v", err)
	}
	if v.config.IsRawOutputEnabled() {
		fmt.Printf("Raw output of 'show mac address-table dynamic':\n%s\n", output)
	}
	trunks, err := v.GetTrunkInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve trunk interfaces: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)\s*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	devices := make([]entities.Device, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring malformed MAC table line: %s\n", match[0])
			}
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		if _, err := strconv.Atoi(vlan); err != nil {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid VLAN '%s': %s\n", vlan, match[0])
			}
			continue
		}
		if !regexp.MustCompile(`^[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}$`).MatchString(mac) {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid MAC '%s': %s\n", mac, match[0])
			}
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z]+\d+\/\d+(?:\/\d+)?$`).MatchString(iface) {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid interface '%s': %s\n", iface, match[0])
			}
			continue
		}
		if trunks[iface] {
			if v.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping MAC %s on trunk interface %s\n", mac, iface)
			}
			continue
		}
		macNoDots := strings.ReplaceAll(mac, ".", "")
		var macFull strings.Builder
		for i := 0; i < len(macNoDots); i += 2 {
			if i > 0 {
				macFull.WriteString(":")
			}
			macFull.WriteString(macNoDots[i : i+2])
		}
		if v.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Adding device VLAN=%s MAC=%s Interface=%s\n", vlan, macFull.String(), iface)
		}
		devices = append(devices, entities.Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(),
			Interface: iface,
		})
	}
	if v.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

// ConfigureVlan configura VLAN em uma interface
func (v *VLANServiceImpl) ConfigureVlan(iface, vlan string) {
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
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
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		fmt.Sprintf("name VLAN_%s", vlan),
		"end",
	}
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
		fmt.Printf("DEBUG: Created VLAN %s\n", vlan)
	}
	return nil
}

// DeleteVLAN deletes an existing VLAN
func (v *VLANServiceImpl) DeleteVLAN(vlan string) error {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("no vlan %s", vlan),
		"end",
	}
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
		fmt.Printf("DEBUG: Deleted VLAN %s\n", vlan)
	}
	return nil
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
		if dev.Interface == port {
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

func normalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
