package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ziutek/telnet"
	"gopkg.in/yaml.v3"
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

type Config struct {
	Host           string            `yaml:"host"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	Sandbox        bool
	Debug          bool
	ReplaceVlan    string
	SkipVlanCheck  bool
	CreateVLANs    bool
}

type Port struct {
	Interface string
	Vlan      string
}

type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

type SwitchManager struct {
	config Config
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

func loadConfig(yamlFile string, overrideHost string, sandbox, debug bool, replaceVlan string, skipVlanCheck, createVLANs bool) (*Config, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %v", yamlFile, err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}
	if overrideHost != "" {
		cfg.Host = overrideHost
	}
	cfg.Sandbox = sandbox
	cfg.Debug = debug
	cfg.ReplaceVlan = replaceVlan
	cfg.SkipVlanCheck = skipVlanCheck
	cfg.CreateVLANs = createVLANs
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" || cfg.EnablePassword == "" {
		return nil, fmt.Errorf("host, username, password, and enable_password are required")
	}

	validateVLAN := func(vlan string, context string) error {
		vlanNum, err := strconv.Atoi(vlan)
		if err != nil {
			return fmt.Errorf("invalid VLAN number in %s: %s must be a number", context, vlan)
		}
		if vlanNum < 1 || vlanNum > 4094 {
			return fmt.Errorf("invalid VLAN number in %s: %s must be between 1 and 4094", context, vlan)
		}
		return nil
	}

	if err := validateVLAN(cfg.DefaultVlan, "default_vlan"); err != nil {
		return nil, err
	}

	newMacToVlan := make(map[string]string)
	for mac, vlan := range cfg.MacToVlan {
		if err := validateVLAN(vlan, fmt.Sprintf("mac_to_vlan for MAC %s", mac)); err != nil {
			return nil, err
		}
		normalizedMac := normalizeMac(mac)
		newMacToVlan[normalizedMac[:6]] = vlan
	}
	cfg.MacToVlan = newMacToVlan

	for i, mac := range cfg.ExcludeMacs {
		cfg.ExcludeMacs[i] = normalizeMac(mac)
	}

	if cfg.ReplaceVlan != "" {
		parts := strings.Split(cfg.ReplaceVlan, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid -r format, expected 'old,new'")
		}
		oldVlan, newVlan := parts[0], parts[1]
		if err := validateVLAN(oldVlan, "-r old VLAN"); err != nil {
			return nil, err
		}
		if err := validateVLAN(newVlan, "-r new VLAN"); err != nil {
			return nil, err
		}
		if cfg.DefaultVlan == oldVlan {
			cfg.DefaultVlan = newVlan
			if cfg.Debug {
				fmt.Printf("DEBUG: Replaced default_vlan %s with %s\n", oldVlan, newVlan)
			}
		}
		for mac, vlan := range cfg.MacToVlan {
			if vlan == oldVlan {
				cfg.MacToVlan[mac] = newVlan
				if cfg.Debug {
					fmt.Printf("DEBUG: Replaced VLAN %s with %s for MAC prefix %s\n", oldVlan, newVlan, mac)
				}
			}
		}
	}
	return &cfg, nil
}

func (sm *SwitchManager) connect() error {
	conn, err := telnet.Dial("tcp", sm.config.Host+":23")
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", sm.config.Host, err)
	}
	sm.conn = conn
	sm.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	sm.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if sm.config.Debug {
		fmt.Printf("DEBUG: Connected to %s\n", sm.config.Host)
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
			if sm.config.Debug {
				fmt.Printf("DEBUG: Sent %s for prompt %s\n", strings.TrimSpace(p.input), p.prompt)
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
			if sm.config.Debug {
				fmt.Printf("DEBUG: Read: %s\n", string(buffer[:n]))
			}
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
		if sm.config.Debug {
			fmt.Println("DEBUG: Disconnected")
		}
	}
}

func (sm *SwitchManager) executeCommand(cmd string) (string, error) {
	if sm.config.Debug {
		fmt.Printf("DEBUG: Executing: %s\n", cmd)
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
	if sm.config.Debug {
		fmt.Printf("DEBUG: Output: %s\n", output)
	}
	return output, nil
}

func (sm *SwitchManager) getVlanList() (map[string]bool, error) {
	output, err := sm.executeCommand("show vlan brief")
	if err != nil {
		return nil, fmt.Errorf("failed to get VLAN list: %v", err)
	}
	if sm.config.Debug {
		fmt.Printf("DEBUG: show vlan brief output:\n%s\n", output)
	}
	re := regexp.MustCompile(`(?m)^(\d+)\s+\S+`)
	matches := re.FindAllStringSubmatch(output, -1)
	vlans := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			vlans[match[1]] = true
		}
	}
	if sm.config.Debug {
		fmt.Printf("DEBUG: Existing VLANs: %v\n", vlans)
	}
	if len(vlans) == 0 {
		fmt.Println("Warning: No VLANs found on the switch. You may need to create the required VLANs.")
	}
	return vlans, nil
}

func (sm *SwitchManager) getTrunkInterfaces() (map[string]bool, error) {
	output, err := sm.executeCommand("show interfaces trunk")
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
	if sm.config.Debug {
		fmt.Printf("DEBUG: Trunk interfaces: %v\n", trunks)
	}
	return trunks, nil
}

func (sm *SwitchManager) getActivePorts() ([]Port, error) {
	output, err := sm.executeCommand("show interfaces status")
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
		if sm.config.Debug {
			fmt.Printf("DEBUG: Found active port %s with VLAN %s\n", match[1], match[2])
		}
		ports = append(ports, Port{
			Interface: match[1],
			Vlan:      match[2],
		})
	}
	if sm.config.Debug {
		fmt.Printf("DEBUG: Found %d active ports\n", len(ports))
	}
	return ports, nil
}

func (sm *SwitchManager) getMacTable() ([]Device, error) {
	output, err := sm.executeCommand("show mac address-table dynamic")
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
	if sm.config.Debug {
		fmt.Printf("DEBUG: Found %d devices in MAC table\n", len(devices))
	}
	return devices, nil
}

func (sm *SwitchManager) configureVlan(iface, vlan string) []string {
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
	if sm.config.Sandbox {
		fmt.Printf("SANDBOX: Simulating config for %s to VLAN %s\n", iface, vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return commands
	}
	for _, cmd := range commands {
		_, err := sm.executeCommand(cmd)
		if err != nil {
			log.Printf("Error executing %s: %v", cmd, err)
		}
	}
	fmt.Printf("Configured %s to VLAN %s\n", iface, vlan)
	return commands
}

func (sm *SwitchManager) createVLAN(vlan string) error {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		fmt.Sprintf("name VLAN_%s", vlan),
		"end",
	}
	for _, cmd := range commands {
		_, err := sm.executeCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to create VLAN %s: %v", vlan, err)
		}
	}
	if sm.config.Debug {
		fmt.Printf("DEBUG: Created VLAN %s\n", vlan)
	}
	return nil
}

func (sm *SwitchManager) getRequiredVLANs() map[string]bool {
	requiredVLANs := make(map[string]bool)
	requiredVLANs[sm.config.DefaultVlan] = true
	for _, vlan := range sm.config.MacToVlan {
		requiredVLANs[vlan] = true
	}
	return requiredVLANs
}

func (sm *SwitchManager) processPorts() error {
	err := sm.connect()
	if err != nil {
		return err
	}
	defer sm.disconnect()

	existingVLANs, err := sm.getVlanList()
	if err != nil {
		return err
	}

	if sm.config.CreateVLANs {
		requiredVLANs := sm.getRequiredVLANs()
		for vlan := range requiredVLANs {
			if !existingVLANs[vlan] {
				fmt.Printf("Creating VLAN %s on the switch\n", vlan)
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

	var commands []string
	changed := false
	for _, port := range activePorts {
		if trunks[port.Interface] {
			if sm.config.Debug {
				fmt.Printf("DEBUG: Skipping trunk interface %s\n", port.Interface)
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
			if sm.config.Debug {
				fmt.Printf("DEBUG: Skipping port %s with no active device\n", port.Interface)
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
		for _, excludeMac := range sm.config.ExcludeMacs {
			if normDevMac == excludeMac {
				excluded = true
				if sm.config.Debug {
					fmt.Printf("DEBUG: Skipping port %s due to excluded MAC %s\n", port.Interface, dev.MacFull)
				}
				break
			}
		}
		if excluded {
			continue
		}
		macPrefix := normDevMac[:6]
		targetVlan := sm.config.MacToVlan[macPrefix]
		if targetVlan == "" {
			targetVlan = sm.config.DefaultVlan
			if sm.config.Debug {
				fmt.Printf("DEBUG: No VLAN mapping for %s on %s, using default %s\n", dev.MacFull, port.Interface, targetVlan)
			}
		}

		if !sm.config.SkipVlanCheck && !existingVLANs[targetVlan] {
			log.Printf("Error: VLAN %s does not exist on the switch, skipping port %s\n", targetVlan, port.Interface)
			continue
		}

		if targetVlan != port.Vlan {
			if sm.config.Debug {
				fmt.Printf("DEBUG: Changing %s from VLAN %s to %s\n", port.Interface, port.Vlan, targetVlan)
			}
			cmds := sm.configureVlan(port.Interface, targetVlan)
			commands = append(commands, cmds...)
			changed = true
		}
	}
	if !sm.config.Sandbox && changed {
		_, err := sm.executeCommand("write memory")
		if err != nil {
			log.Printf("Error saving config: %v", err)
		} else {
			fmt.Println("Configuration saved")
		}
	} else if !changed {
		fmt.Println("No changes were needed")
	}
	return nil
}

func main() {
	yamlFile := flag.String("y", "config.yaml", "YAML configuration file")
	write := flag.Bool("w", false, "Write changes (disable sandbox)")
	debug := flag.Bool("d", false, "Enable debug logging")
	host := flag.String("h", "", "Switch host (overrides YAML)")
	replaceVlan := flag.String("r", "", "Replace VLAN (format: old,new)")
	skipVlanCheck := flag.Bool("s", false, "Skip VLAN check (use with caution)")
	createVLANs := flag.Bool("c", false, "Create missing VLANs on the switch")
	flag.Parse()
	cfg, err := loadConfig(*yamlFile, *host, !*write, *debug, *replaceVlan, *skipVlanCheck, *createVLANs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Starting Negev for switch %s\n", cfg.Host)
	sm := &SwitchManager{config: *cfg}
	err = sm.processPorts()
	if err != nil {
		log.Fatal(err)
	}
}
