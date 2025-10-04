package switchmanager

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/internal/config"
	"github.com/carlosrabelo/negev/internal/transport"
)

// Port represents a live switch port paired with its VLAN
type Port struct {
	Interface string
	Vlan      string
}

// Device stores device information discovered on a port
type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

// Manager coordinates the VLAN workflow for a single switch
type Manager struct {
	cfg       config.SwitchConfig
	globalCfg config.Config
	client    transport.Client
}

// New creates a Manager with a cached transport client
func New(cfg config.SwitchConfig, globalCfg config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		globalCfg: globalCfg,
		client:    transport.Get(cfg),
	}
}

// ProcessPorts adjusts VLANs for active ports according to configuration
func (m *Manager) ProcessPorts() error {
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Switch %s config DefaultVlan=%s ExcludeMacs=%v ExcludePorts=%v\n", m.cfg.Target, m.cfg.DefaultVlan, m.cfg.ExcludeMacs, m.cfg.ExcludePorts)
	}
	if !m.client.IsConnected() {
		if err := m.client.Connect(); err != nil {
			return err
		}
	}

	existingVLANs, err := m.getVlanList()
	if err != nil {
		return err
	}

	if m.cfg.CreateVLANs {
		requiredVLANs := m.getRequiredVLANs()
		for _, vlan := range sortedKeys(requiredVLANs) {
			if !existingVLANs[vlan] {
				fmt.Printf("Creating VLAN %s on switch\n", vlan)
				if err := m.createVLAN(vlan); err != nil {
					return err
				}
				existingVLANs[vlan] = true
			}
		}
	}

	trunks, err := m.getTrunkInterfaces()
	if err != nil {
		return err
	}
	activePorts, err := m.getActivePorts()
	if err != nil {
		return err
	}
	if len(activePorts) == 0 {
		fmt.Println("No active ports found on the switch")
		return nil
	}

	devices, err := m.getMacTable()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No devices found in the MAC address table")
		return nil
	}

	excludedPorts := make(map[string]struct{}, len(m.cfg.ExcludePorts))
	for _, port := range m.cfg.ExcludePorts {
		excludedPorts[strings.ToLower(port)] = struct{}{}
	}

	changed := false
	for _, port := range activePorts {
		if _, skip := excludedPorts[strings.ToLower(port.Interface)]; skip {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping excluded port %s\n", port.Interface)
			}
			continue
		}
		if trunks[port.Interface] {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring trunk interface %s\n", port.Interface)
			}
			continue
		}

		portDevices := filterDevices(devices, port.Interface)
		if len(portDevices) == 0 {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping port %s because no device is active\n", port.Interface)
			}
			continue
		}
		if len(portDevices) > 1 {
			log.Printf("Warning: Multiple MACs detected on port %s: %v. Ignoring port to stay safe.", port.Interface, deviceMacs(portDevices))
			continue
		}

		dev := portDevices[0]
		normDevMac := config.NormalizeMAC(dev.MacFull)
		if m.cfg.IsDebugEnabled() {
			fmt.Printf("DEBUG: Checking MAC %s on port %s against exclude list %v\n", dev.MacFull, port.Interface, m.cfg.ExcludeMacs)
		}
		if isExcluded(normDevMac, m.cfg.ExcludeMacs) {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: MAC %s is excluded, leaving port %s as is\n", dev.MacFull, port.Interface)
			}
			continue
		}

		macPrefix := normDevMac[:6]
		targetVlan := m.cfg.MacToVlan[macPrefix]
		if targetVlan == "" || targetVlan == "0" || targetVlan == "00" {
			targetVlan = m.cfg.DefaultVlan
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: No mapping for %s (prefix %s), going with default VLAN %s\n", dev.MacFull, macPrefix, m.cfg.DefaultVlan)
			}
		} else if m.cfg.IsDebugEnabled() {
			fmt.Printf("DEBUG: MAC %s (prefix %s) maps to VLAN %s\n", dev.MacFull, macPrefix, targetVlan)
		}

		if !m.cfg.SkipVlanCheck && !existingVLANs[targetVlan] {
			log.Printf("Error: VLAN %s does not exist on switch, ignoring port %s", targetVlan, port.Interface)
			continue
		}

		if targetVlan != port.Vlan {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Changing %s from VLAN %s to %s\n", port.Interface, port.Vlan, targetVlan)
			}
			m.configureVlan(port.Interface, targetVlan)
			changed = true
		} else if m.cfg.IsDebugEnabled() {
			fmt.Printf("DEBUG: Port %s already sits in VLAN %s\n", port.Interface, targetVlan)
		}
	}

	fmt.Printf("State before saving: Sandbox=%v Changed=%v\n", m.cfg.Sandbox, changed)
	if !m.cfg.Sandbox && changed {
		if _, err := m.client.ExecuteCommand("write memory"); err != nil {
			log.Printf("Error saving configuration: %v", err)
		} else {
			fmt.Println("Configuration saved")
		}
	} else {
		if !changed {
			fmt.Println("No changes required")
		} else if m.cfg.Sandbox {
			fmt.Println("Changes simulated (sandbox mode enabled, use -w to apply)")
		}
	}
	return nil
}

func (m *Manager) getVlanList() (map[string]bool, error) {
	output, err := m.client.ExecuteCommand("show vlan brief")
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
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Existing VLANs: %v\n", vlans)
	}
	if len(vlans) == 0 {
		fmt.Println("Warning: No VLANs found on the switch. Maybe create them first.")
	}
	return vlans, nil
}

func (m *Manager) getTrunkInterfaces() (map[string]bool, error) {
	output, err := m.client.ExecuteCommand("show interfaces trunk")
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
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

func (m *Manager) getActivePorts() ([]Port, error) {
	output, err := m.client.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface status: %v", err)
	}
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+(?:\/\d+)?)\s+(?:[^\s]*\s+)?connected\s+(\d+|trunk)\s+.*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	ports := make([]Port, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			log.Printf("Warning: Ignoring malformed interface line: %s", match[0])
			continue
		}
		if !strings.Contains(match[0], "connected") {
			log.Printf("Warning: Line %s does not show connected status, ignoring", match[0])
			continue
		}
		ports = append(ports, Port{Interface: match[1], Vlan: match[2]})
	}
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d active ports\n", len(ports))
	}
	return ports, nil
}

func (m *Manager) getMacTable() ([]Device, error) {
	output, err := m.client.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MAC table: %v", err)
	}
	if m.cfg.IsRawOutputEnabled() {
		fmt.Printf("Raw output of 'show mac address-table dynamic':\n%s\n", output)
	}
	trunks, err := m.getTrunkInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve trunk interfaces: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)\s*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	devices := make([]Device, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring malformed MAC table line: %s\n", match[0])
			}
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		if _, err := strconv.Atoi(vlan); err != nil {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid VLAN '%s': %s\n", vlan, match[0])
			}
			continue
		}
		if !regexp.MustCompile(`^[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}$`).MatchString(mac) {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid MAC '%s': %s\n", mac, match[0])
			}
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z]+\d+\/\d+(?:\/\d+)?$`).MatchString(iface) {
			if m.cfg.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid interface '%s': %s\n", iface, match[0])
			}
			continue
		}
		if trunks[iface] {
			if m.cfg.IsDebugEnabled() {
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
		if m.cfg.IsDebugEnabled() {
			fmt.Printf("DEBUG: Adding device VLAN=%s MAC=%s Interface=%s\n", vlan, macFull.String(), iface)
		}
		devices = append(devices, Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(),
			Interface: iface,
		})
	}
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

func (m *Manager) configureVlan(iface, vlan string) {
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
	if m.cfg.Sandbox {
		fmt.Printf("SANDBOX: Simulating configuration for %s to VLAN %s\n", iface, vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return
	}
	for _, cmd := range commands {
		if _, err := m.client.ExecuteCommand(cmd); err != nil {
			log.Printf("Error executing %s: %v", cmd, err)
		}
	}
	fmt.Printf("Configured %s to VLAN %s\n", iface, vlan)
}

func (m *Manager) createVLAN(vlan string) error {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		fmt.Sprintf("name VLAN_%s", vlan),
		"end",
	}
	for _, cmd := range commands {
		if _, err := m.client.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to create VLAN %s: %v", vlan, err)
		}
	}
	if m.cfg.IsDebugEnabled() {
		fmt.Printf("DEBUG: Created VLAN %s\n", vlan)
	}
	return nil
}

func (m *Manager) getRequiredVLANs() map[string]bool {
	requiredVLANs := make(map[string]bool)
	requiredVLANs[m.cfg.DefaultVlan] = true
	for _, vlan := range m.cfg.MacToVlan {
		if vlan == "0" || vlan == "00" {
			continue
		}
		requiredVLANs[vlan] = true
	}
	return requiredVLANs
}

func filterDevices(devices []Device, port string) []Device {
	filtered := make([]Device, 0, len(devices))
	for _, dev := range devices {
		if dev.Interface == port {
			filtered = append(filtered, dev)
		}
	}
	return filtered
}

func deviceMacs(devices []Device) []string {
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
