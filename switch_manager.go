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

// Port representa uma porta ativa
type Port struct {
	Interface string
	Vlan      string
}

// Device representa um dispositivo na tabela MAC
type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

// SwitchManager gerencia a conexão Telnet com o switch
type SwitchManager struct {
	config SwitchConfig
	conn   *telnet.Conn
}

func normalizeMac(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

func getMacList(devices []Device) []string {
	var macs []string
	for _, d := range devices {
		macs = append(macs, d.MacFull)
	}
	return macs
}

func (sm *SwitchManager) connect(cfg *Config) error {
	conn, err := telnet.Dial("tcp", sm.config.Target+":23")
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", sm.config.Target, err)
	}
	sm.conn = conn
	sm.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	sm.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if cfg.Debug {
		log.Printf("DEBUG: Connected to %s", sm.config.Target)
	}
	prompts := []struct {
		prompt string
		input  string
	}{
		{PromptUsername, sm.config.Username + "\n"},
		{PromptPassword, sm.config.Password + "\n"},
		{PromptEnable, "enable\n"},
		{PromptPassword, sm.config.EnablePassword + "\n"},
		{PromptPrivileged, TerminalLengthCmd},
		{PromptPrivileged, ""},
	}
	for _, p := range prompts {
		output, err := sm.readUntil(p.prompt, DefaultTimeout)
		if err != nil {
			return fmt.Errorf("failed waiting for %s: %v, output: %s", p.prompt, err, output)
		}
		if p.input != "" {
			sm.conn.Write([]byte(p.input))
			if cfg.Debug {
				log.Printf("DEBUG: Sent %s for prompt %s", strings.TrimSpace(p.input), p.prompt)
			}
		}
	}
	return nil
}

func (sm *SwitchManager) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := sm.conn.Read(buffer)
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if n > 0 {
			output.Write(buffer[:n])
			if strings.Contains(output.String(), pattern) {
				return output.String(), nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output.String(), fmt.Errorf("timeout waiting for %s", pattern)
}

func (sm *SwitchManager) disconnect() {
	if sm.conn != nil {
		sm.conn.Close()
	}
}

func (sm *SwitchManager) executeCommand(cmd string, cfg *Config) (string, error) {
	if cfg.Debug {
		log.Printf("DEBUG: Executing: %s", cmd)
	}
	sm.conn.Write([]byte(cmd + "\n"))
	output, err := sm.readUntil(PromptPrivileged, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n")
	} else {
		output = ""
	}
	if cfg.Debug {
		log.Printf("DEBUG: Output: %s", output)
	}
	return output, nil
}

func (sm *SwitchManager) getVlanList(cfg *Config) (map[string]bool, error) {
	output, err := sm.executeCommand("show vlan brief", cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get VLAN list: %v", err)
	}
	re := regexp.MustCompile(`(?m)^(\d+)\s+\S+`)
	matches := re.FindAllStringSubmatch(output, -1)
	vlans := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			vlans[match[1]] = true
		}
	}
	if cfg.Debug {
		log.Printf("DEBUG: Existing VLANs: %v", vlans)
	}
	return vlans, nil
}

func (sm *SwitchManager) getTrunkInterfaces(cfg *Config) (map[string]bool, error) {
	output, err := sm.executeCommand("show interfaces trunk", cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get trunk interfaces: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z]+\d+\/\d+(?:\/\d+)?)`)
	matches := re.FindAllStringSubmatch(output, -1)
	trunks := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			trunks[match[1]] = true
		}
	}
	if cfg.Debug {
		log.Printf("DEBUG: Trunk interfaces: %v", trunks)
	}
	return trunks, nil
}

func (sm *SwitchManager) getActivePorts(cfg *Config) ([]Port, error) {
	output, err := sm.executeCommand("show interfaces status", cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces status: %v", err)
	}
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+(?:\/\d+)?)\s+(?:[^\s]*\s+)?connected\s+(\d+|trunk)\s+.*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	var ports []Port
	for _, match := range matches {
		if len(match) < 3 {
			log.Printf("Warning: Skipping malformed interface line: %s", match[0])
			continue
		}
		if cfg.Debug {
			log.Printf("DEBUG: Found active port %s with VLAN %s", match[1], match[2])
		}
		ports = append(ports, Port{
			Interface: match[1],
			Vlan:      match[2],
		})
	}
	if cfg.Debug {
		log.Printf("DEBUG: Found %d active ports", len(ports))
	}
	return ports, nil
}

func (sm *SwitchManager) getMacTable(cfg *Config) ([]Device, error) {
	output, err := sm.executeCommand("show mac address-table dynamic", cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC table: %v", err)
	}
	re := regexp.MustCompile(`(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+\w+\s+(\S+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	var devices []Device
	for _, match := range matches {
		if len(match) < 4 {
			log.Printf("Warning: Skipping malformed MAC table line: %s", match[0])
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		macNoDots := strings.ReplaceAll(mac, ".", "")
		var macFull strings.Builder
		for i := 0; i < len(macNoDots); i += 2 {
			if i > 0 {
				macFull.WriteString(":")
			}
			macFull.WriteString(macNoDots[i : i+2])
		}
		devices = append(devices, Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(),
			Interface: iface,
		})
	}
	if cfg.Debug {
		log.Printf("DEBUG: Found %d devices in MAC table", len(devices))
	}
	return devices, nil
}

func (sm *SwitchManager) configureVlan(iface string, vlan int, cfg *Config) []string {
	vlanStr := strconv.Itoa(vlan)
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlanStr,
		"end",
	}
	if cfg.Debug {
		log.Printf("DEBUG: Configuring %s to VLAN %s", iface, vlanStr)
	}
	if cfg.Sandbox {
		log.Printf("SANDBOX: Simulating config for %s to VLAN %s", iface, vlanStr)
		for _, cmd := range commands {
			log.Printf("  %s", cmd)
		}
		return commands
	}
	for _, cmd := range commands {
		_, err := sm.executeCommand(cmd, cfg)
		if err != nil {
			log.Printf("Error executing %s: %v", cmd, err)
		}
	}
	log.Printf("Configured %s to VLAN %s", iface, vlanStr)
	return commands
}

func (sm *SwitchManager) createVLAN(vlan int, cfg *Config) error {
	vlanStr := strconv.Itoa(vlan)
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlanStr),
		fmt.Sprintf("name VLAN_%s", vlanStr),
		"end",
	}
	for _, cmd := range commands {
		_, err := sm.executeCommand(cmd, cfg)
		if err != nil {
			return fmt.Errorf("failed to create VLAN %s: %v", vlanStr, err)
		}
	}
	if cfg.Debug {
		log.Printf("DEBUG: Created VLAN %s", vlanStr)
	}
	return nil
}

func (sm *SwitchManager) getRequiredVLANs(cfg *Config) map[int]bool {
	requiredVLANs := make(map[int]bool)
	requiredVLANs[cfg.DefaultVLAN] = true
	for _, mapping := range cfg.DefaultOUIMappings {
		requiredVLANs[mapping.VLAN] = true
	}
	for _, sw := range cfg.Switches {
		for _, mapping := range sw.OUIMappings {
			requiredVLANs[mapping.VLAN] = true
		}
	}
	requiredVLANs[cfg.NullVLAN] = true
	return requiredVLANs
}

func (sm *SwitchManager) processPorts(cfg *Config) error {
	err := sm.connect(cfg)
	if err != nil {
		return err
	}
	defer sm.disconnect()

	existingVLANs, err := sm.getVlanList(cfg)
	if err != nil {
		return err
	}

	if sm.config.CreateVLANs {
		requiredVLANs := sm.getRequiredVLANs(cfg)
		for vlan := range requiredVLANs {
			vlanStr := strconv.Itoa(vlan)
			if !existingVLANs[vlanStr] {
				log.Printf("Creating VLAN %d on the switch", vlan)
				if err := sm.createVLAN(vlan, cfg); err != nil {
					return err
				}
				existingVLANs[vlanStr] = true
			}
		}
	}

	trunks, err := sm.getTrunkInterfaces(cfg)
	if err != nil {
		return err
	}
	activePorts, err := sm.getActivePorts(cfg)
	if err != nil {
		return err
	}
	if len(activePorts) == 0 {
		log.Println("No active ports found on the switch")
		return nil
	}

	devices, err := sm.getMacTable(cfg)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		log.Println("No devices found in the MAC address table")
		return nil
	}

	var commands []string
	changed := false
	for _, port := range activePorts {
		if !isPortTypeAllowed(port.Interface, cfg) {
			if cfg.Debug {
				log.Printf("DEBUG: Skipping port %s, type not allowed", port.Interface)
			}
			continue
		}
		if trunks[port.Interface] {
			if cfg.Debug {
				log.Printf("DEBUG: Skipping trunk interface %s", port.Interface)
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
			if cfg.Debug {
				log.Printf("DEBUG: Skipping port %s with no active device", port.Interface)
			}
			continue
		}
		if len(portDevices) > 1 {
			log.Printf("Warning: Multiple MACs detected on port %s: %v. Skipping port to avoid ambiguity.",
				port.Interface, getMacList(portDevices))
			continue
		}
		dev := portDevices[0]
		normDevMac := normalizeMac(dev.MacFull)
		excluded := false
		for _, excludeMac := range cfg.ExcludeMacs {
			if normDevMac == excludeMac {
				excluded = true
				if cfg.Debug {
					log.Printf("DEBUG: Skipping port %s due to excluded MAC %s", port.Interface, dev.MacFull)
				}
				break
			}
		}
		if excluded {
			continue
		}
		macPrefix := normDevMac[:6]
		targetVlan := getVLANFromOUI(macPrefix, &sm.config, cfg)
		if targetVlan == 0 {
			targetVlan = cfg.DefaultVLAN
			if cfg.Debug {
				log.Printf("DEBUG: No VLAN mapping for %s on %s, using default %d", dev.MacFull, port.Interface, targetVlan)
			}
		}

		vlanStr := strconv.Itoa(targetVlan)
		if !cfg.SkipVlanCheck && !existingVLANs[vlanStr] {
			log.Printf("Error: VLAN %d does not exist on the switch, skipping port %s", targetVlan, port.Interface)
			continue
		}

		if vlanStr != port.Vlan {
			if cfg.Debug {
				log.Printf("DEBUG: Changing %s from VLAN %s to %d", port.Interface, port.Vlan, targetVlan)
			}
			cmds := sm.configureVlan(port.Interface, targetVlan, cfg)
			commands = append(commands, cmds...)
			changed = true
		}
	}
	if !cfg.Sandbox && changed {
		_, err := sm.executeCommand("write memory", cfg)
		if err != nil {
			log.Printf("Error saving config: %v", err)
		} else {
			log.Println("Configuration saved")
		}
	} else if !changed {
		log.Println("No changes were needed")
	}
	return nil
}
