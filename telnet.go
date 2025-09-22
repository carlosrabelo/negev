package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ziutek/telnet"
)

const (
	DefaultTimeout    = 30 * time.Second
	BufferSize        = 4096
	PromptUsername    = "Username:"
	PromptPassword    = "Password:"
	PromptEnable      = ">"
	PromptPrivileged  = "#"
	TerminalLengthCmd = "terminal length 0\n"
)

// TelnetClient manages a Telnet connection to a switch
type TelnetClient struct {
	conn   *telnet.Conn
	config SwitchConfig
}

// NewTelnetClient creates a new Telnet client with the given configuration
func NewTelnetClient(config SwitchConfig) *TelnetClient {
	return &TelnetClient{config: config}
}

// Connect establishes a Telnet connection to the switch
func (tc *TelnetClient) Connect() error {
	if tc.conn != nil {
		return nil
	}
	conn, err := telnet.Dial("tcp", tc.config.Target+":23")
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", tc.config.Target, err)
	}
	tc.conn = conn
	tc.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Connected to %s\n", tc.config.Target)
	}
	prompts := []struct {
		prompt string
		input  string
	}{
		{PromptUsername, tc.config.Username + "\n"},
		{PromptPassword, tc.config.Password + "\n"},
		{PromptEnable, "enable\n"},
		{PromptPassword, tc.config.EnablePassword + "\n"},
		{PromptPrivileged, TerminalLengthCmd},
		{PromptPrivileged, ""},
	}
	for _, p := range prompts {
		output, err := tc.readUntil(p.prompt, DefaultTimeout)
		if err != nil {
			return fmt.Errorf("failed to wait for %s: %v, output: %s", p.prompt, err, output)
		}
		if p.input != "" {
			tc.conn.Write([]byte(p.input))
			if tc.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Sent %s for prompt %s\n", strings.TrimSpace(p.input), p.prompt)
			}
		}
	}
	return nil
}

// readUntil reads from the Telnet connection until the specified pattern is found
func (tc *TelnetClient) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := tc.conn.Read(buffer)
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if n > 0 {
			output.Write(buffer[:n])
			if tc.config.IsRawOutputEnabled() {
				fmt.Printf("Switch output: Read: %s\n", string(buffer[:n]))
			}
			if strings.Contains(output.String(), pattern) {
				return output.String(), nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output.String(), fmt.Errorf("timeout waiting for %s", pattern)
}

// Disconnect closes the Telnet connection
func (tc *TelnetClient) Disconnect() {
	if tc.conn != nil {
		tc.conn.Close()
		if tc.config.IsDebugEnabled() {
			fmt.Println("DEBUG: Disconnected")
		}
		tc.conn = nil
	}
}

func (tc *TelnetClient) IsConnected() bool {
	return tc.conn != nil
}

// ExecuteCommand sends a command to the switch and returns its output
func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Executing: %s\n", cmd)
	}
	tc.conn.Write([]byte(cmd + "\n"))
	output, err := tc.readUntil(PromptPrivileged, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n")
	} else {
		output = ""
	}
	if tc.config.IsRawOutputEnabled() {
		fmt.Printf("Switch output for '%s':\n%s\n", cmd, output)
	}
	return output, nil
}

// SwitchManager manages switch operations via a switch client
type SwitchManager struct {
	config       SwitchConfig
	globalConfig Config
	client       SwitchClient
}

// NewSwitchManager creates a new switch manager
func NewSwitchManager(config SwitchConfig, globalConfig Config) *SwitchManager {
	return &SwitchManager{
		config:       config,
		globalConfig: globalConfig,
		client:       getSwitchClient(config),
	}
}

// ProcessPorts processes switch ports and configures VLANs as needed
func (sm *SwitchManager) ProcessPorts() error {
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Switch configuration %s: DefaultVlan=%s, ExcludeMacs=%v, ExcludePorts=%v\n", sm.config.Target, sm.config.DefaultVlan, sm.config.ExcludeMacs, sm.config.ExcludePorts)
	}
	if !sm.client.IsConnected() {
		if err := sm.client.Connect(); err != nil {
			return err
		}
	}

	existingVLANs, err := sm.getVlanList()
	if err != nil {
		return err
	}

	if sm.config.CreateVLANs {
		requiredVLANs := sm.getRequiredVLANs()
		for vlan := range requiredVLANs {
			if !existingVLANs[vlan] {
				fmt.Printf("Creating VLAN %s on switch\n", vlan)
				if err := sm.createVLAN(vlan); err != nil {
					return err
				}
				existingVLANs[vlan] = true
			}
		}
	}

	trunks, err := sm.getTrunkInterfaces()
	if err != nil {
		return err
	}
	activePorts, err := sm.getActivePorts()
	if err != nil {
		return err
	}
	if len(activePorts) == 0 {
		fmt.Println("No active ports found on the switch")
		return nil
	}

	devices, err := sm.getMacTable()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No devices found in the MAC address table")
		return nil
	}

	excludedPorts := make(map[string]struct{}, len(sm.config.ExcludePorts))
	for _, port := range sm.config.ExcludePorts {
		excludedPorts[strings.ToLower(port)] = struct{}{}
	}

	var commands []string
	changed := false
	for _, port := range activePorts {
		if _, skip := excludedPorts[strings.ToLower(port.Interface)]; skip {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping excluded port %s\n", port.Interface)
			}
			continue
		}
		if trunks[port.Interface] {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring trunk interface %s\n", port.Interface)
			}
			continue
		}
		var portDevices []Device
		for _, d := range devices {
			if d.Interface == port.Interface {
				portDevices = append(portDevices, d)
			}
		}
		if len(portDevices) == 0 {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Skipping port %s with no active devices\n", port.Interface)
			}
			continue
		}
		var targetVlan string
		if len(portDevices) > 1 {
			log.Printf("Warning: Multiple MACs detected on port %s: %v. Ignoring port to avoid ambiguity.",
				port.Interface, getMacList(portDevices))
			continue
		}
		// One device found
		dev := portDevices[0]
		normDevMac := normalizeMac(dev.MacFull)
		if sm.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Checking MAC %s on port %s against exclude_macs: %v\n", dev.MacFull, port.Interface, sm.config.ExcludeMacs)
		}
		isExcluded := false
		for _, excludeMac := range sm.config.ExcludeMacs {
			if normDevMac == excludeMac {
				if sm.config.IsDebugEnabled() {
					fmt.Printf("DEBUG: MAC %s excluded, ignoring port %s\n", dev.MacFull, port.Interface)
				}
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue
		}
		// Proceed only if the MAC is not excluded
		if sm.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: MAC %s not excluded, checking mac_to_vlan for prefix %s\n", dev.MacFull, normDevMac[:6])
		}
		macPrefix := normDevMac[:6]
		targetVlan = sm.config.MacToVlan[macPrefix]
		if targetVlan == "" || targetVlan == "0" || targetVlan == "00" {
			targetVlan = sm.config.DefaultVlan
			if sm.config.IsDebugEnabled() {
				if targetVlan == "0" || targetVlan == "00" {
					fmt.Printf("DEBUG: Ignoring invalid VLAN mapping %s for MAC %s (prefix %s) on port %s, using switch default_vlan %s\n", targetVlan, dev.MacFull, macPrefix, port.Interface, sm.config.DefaultVlan)
				} else {
					fmt.Printf("DEBUG: No VLAN mapping for %s (prefix %s) on port %s, using switch default_vlan %s\n", dev.MacFull, macPrefix, port.Interface, sm.config.DefaultVlan)
				}
			}
		} else {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: MAC %s (prefix %s) mapped to VLAN %s on port %s\n", dev.MacFull, macPrefix, targetVlan, port.Interface)
			}
		}

		if !sm.config.SkipVlanCheck && !existingVLANs[targetVlan] {
			log.Printf("Error: VLAN %s does not exist on the switch, ignoring port %s\n", targetVlan, port.Interface)
			continue
		}

		if targetVlan != port.Vlan {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Changing %s from VLAN %s to %s\n", port.Interface, port.Vlan, targetVlan)
			}
			cmds := sm.configureVlan(port.Interface, targetVlan)
			commands = append(commands, cmds...)
			changed = true
		} else {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Port %s already configured for VLAN %s, no changes needed\n", port.Interface, targetVlan)
			}
		}
	}
	fmt.Printf("State before saving: Sandbox=%v, Changed=%v\n", sm.config.Sandbox, changed)
	if !sm.config.Sandbox && changed {
		_, err := sm.client.ExecuteCommand("write memory")
		if err != nil {
			log.Printf("Error saving configuration: %v", err)
		} else {
			fmt.Println("Configuration saved")
		}
	} else {
		if !changed {
			fmt.Println("No changes required")
		} else if sm.config.Sandbox {
			fmt.Println("Changes simulated (sandbox mode enabled, use -w to apply)")
		}
	}
	return nil
}

// getVlanList retrieves the list of existing VLANs on the switch
func (sm *SwitchManager) getVlanList() (map[string]bool, error) {
	output, err := sm.client.ExecuteCommand("show vlan brief")
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
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Existing VLANs: %v\n", vlans)
	}
	if len(vlans) == 0 {
		fmt.Println("Warning: No VLANs found on the switch. You may need to create the required VLANs.")
	}
	return vlans, nil
}

// getTrunkInterfaces retrieves the list of trunk interfaces on the switch
func (sm *SwitchManager) getTrunkInterfaces() (map[string]bool, error) {
	output, err := sm.client.ExecuteCommand("show interfaces trunk")
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
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

// getActivePorts retrieves the list of active ports on the switch
func (sm *SwitchManager) getActivePorts() ([]Port, error) {
	output, err := sm.client.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface status: %v", err)
	}
	// Strict regex to capture only ports with "connected" status
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+(?:\/\d+)?)\s+(?:[^\s]*\s+)?connected\s+(\d+|trunk)\s+.*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	var ports []Port
	for _, match := range matches {
		if len(match) < 3 {
			log.Printf("Warning: Ignoring malformed interface line: %s", match[0])
			continue
		}
		// Explicit check to avoid ambiguous status
		if !strings.Contains(match[0], "connected") {
			log.Printf("Warning: Line %s does not contain connected status, ignoring", match[0])
			continue
		}
		if sm.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Found active port %s with VLAN %s\n", match[1], match[2])
		}
		ports = append(ports, Port{
			Interface: match[1],
			Vlan:      match[2],
		})
	}
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d active ports\n", len(ports))
	}
	return ports, nil
}

// getMacTable retrieves the dynamic MAC address table from the switch
func (sm *SwitchManager) getMacTable() ([]Device, error) {
	output, err := sm.client.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MAC table: %v", err)
	}
	if sm.config.IsRawOutputEnabled() {
		fmt.Printf("Raw output of 'show mac address-table dynamic':\n%s\n", output)
	}
	// Get trunk interfaces to filter out trunk ports
	trunks, err := sm.getTrunkInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve trunk interfaces: %v", err)
	}
	// Adjusted regex to be more flexible with spacing
	re := regexp.MustCompile(`(?m)^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)\s*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	var devices []Device
	for _, match := range matches {
		if len(match) < 4 {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring malformed MAC table line: %s\n", match[0])
			}
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		// Validate VLAN
		if _, err := strconv.Atoi(vlan); err != nil {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid VLAN '%s' in MAC table: %s\n", vlan, match[0])
			}
			continue
		}
		// Validate MAC
		if !regexp.MustCompile(`^[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}$`).MatchString(mac) {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid MAC '%s' in MAC table: %s\n", mac, match[0])
			}
			continue
		}
		// Validate interface
		if !regexp.MustCompile(`^[A-Za-z]+\d+\/\d+(?:\/\d+)?$`).MatchString(iface) {
			if sm.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Ignoring line with invalid interface '%s' in MAC table: %s\n", iface, match[0])
			}
			continue
		}
		// Skip if the interface is a trunk
		if trunks[iface] {
			if sm.config.IsDebugEnabled() {
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
		if sm.config.IsDebugEnabled() {
			fmt.Printf("DEBUG: Adding device: VLAN=%s, MAC=%s, Interface=%s\n", vlan, macFull.String(), iface)
		}
		devices = append(devices, Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(),
			Interface: iface,
		})
	}
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

// configureVlan configures a VLAN on the specified interface
func (sm *SwitchManager) configureVlan(iface, vlan string) []string {
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
	if sm.config.Sandbox {
		fmt.Printf("SANDBOX: Simulating configuration for %s to VLAN %s\n", iface, vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return commands
	}
	for _, cmd := range commands {
		_, err := sm.client.ExecuteCommand(cmd)
		if err != nil {
			log.Printf("Error executing %s: %v", cmd, err)
		}
	}
	fmt.Printf("Configured %s to VLAN %s\n", iface, vlan)
	return commands
}

// createVLAN creates a new VLAN on the switch
func (sm *SwitchManager) createVLAN(vlan string) error {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		fmt.Sprintf("name VLAN_%s", vlan),
		"end",
	}
	for _, cmd := range commands {
		_, err := sm.client.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to create VLAN %s: %v", vlan, err)
		}
	}
	if sm.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Created VLAN %s\n", vlan)
	}
	return nil
}

// getRequiredVLANs returns the set of VLANs required by the configuration
func (sm *SwitchManager) getRequiredVLANs() map[string]bool {
	requiredVLANs := make(map[string]bool)
	requiredVLANs[sm.config.DefaultVlan] = true
	for _, vlan := range sm.config.MacToVlan {
		if vlan == "0" || vlan == "00" {
			continue
		}
		requiredVLANs[vlan] = true
	}
	return requiredVLANs
}
