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

// loadConfig loads the configuration from a YAML file and applies overrides.
// It validates required fields, VLAN numbers, and applies VLAN replacements if specified.
// Returns the parsed Config and an error if any validation fails.
func loadConfig(yamlFile string, overrideHost string, sandbox bool, debug bool, replaceVlan string) (*Config, error) {
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
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" || cfg.EnablePassword == "" {
		return nil, fmt.Errorf("host, username, password, and enable_password are required")
	}

	// Validate VLAN numbers (must be between 1 and 4094)
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

	// Validate default_vlan
	if err := validateVLAN(cfg.DefaultVlan, "default_vlan"); err != nil {
		return nil, err
	}

	// Validate VLANs in mac_to_vlan
	for mac, vlan := range cfg.MacToVlan {
		if err := validateVLAN(vlan, fmt.Sprintf("mac_to_vlan for MAC %s", mac)); err != nil {
			return nil, err
		}
	}

	if cfg.ReplaceVlan != "" {
		parts := strings.Split(cfg.ReplaceVlan, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid -r format, expected 'old,new'")
		}
		oldVlan, newVlan := parts[0], parts[1]
		// Validate old and new VLANs in -r
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
	sm.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	sm.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if sm.config.Debug {
		fmt.Printf("DEBUG: Connected to %s\n", sm.config.Host)
	}
	prompts := []struct {
		prompt string
		input  string
	}{
		{"Username:", sm.config.Username + "\n"},
		{"Password:", sm.config.Password + "\n"},
		{">", "enable\n"},
		{"Password:", sm.config.EnablePassword + "\n"},
		{"#", "terminal length 0\n"},
		{"#", ""},
	}
	for _, p := range prompts {
		output, err := sm.readUntil(p.prompt, 30*time.Second)
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
	buffer := make([]byte, 4096)
	var output strings.Builder
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
	output, err := sm.readUntil("#", 30*time.Second)
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

// getVlanList retrieves the list of existing VLANs on the switch using "show vlan brief".
// It returns a map where the key is the VLAN ID and the value is true if the VLAN exists.
// Returns an error if the command fails or the output cannot be parsed.
func (sm *SwitchManager) getVlanList() (map[string]bool, error) {
	output, err := sm.executeCommand("show vlan brief")
	if err != nil {
		return nil, fmt.Errorf("failed to get VLAN list: %v", err)
	}
	// Regex to match VLAN IDs (e.g., "1", "10", "100") at the start of each line
	re := regexp.MustCompile(`^(\d+)\s+`)
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
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+)\s+\S+\s+(connected)\s+(\d+|trunk)`)
	matches := re.FindAllStringSubmatch(output, -1)
	var ports []Port
	for _, match := range matches {
		if len(match) > 3 && match[2] == "connected" {
			ports = append(ports, Port{
				Interface: match[1],
				Vlan:      match[3],
			})
		}
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

// processPorts processes active ports and configures VLANs based on MAC addresses.
// It skips trunk interfaces, excluded MACs, and validates VLANs before applying changes.
// Returns an error if any step fails.
func (sm *SwitchManager) processPorts() error {
	err := sm.connect()
	if err != nil {
		return err
	}
	defer sm.disconnect()

	// Get the list of existing VLANs on the switch
	existingVLANs, err := sm.getVlanList()
	if err != nil {
		return err
	}

	trunks, err := sm.getTrunkInterfaces()
	if err != nil {
		return err
	}
	activePorts, err := sm.getActivePorts()
	if err != nil {
		return err
	}
	devices, err := sm.getMacTable()
	if err != nil {
		return err
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
		var dev *Device
		for _, d := range devices {
			if d.Interface == port.Interface {
				dev = &d
				break
			}
		}
		if dev == nil {
			if sm.config.Debug {
				fmt.Printf("DEBUG: Skipping port %s with no active device\n", port.Interface)
			}
			continue
		}
		excluded := false
		for _, excludeMac := range sm.config.ExcludeMacs {
			normExclude := strings.ToLower(strings.ReplaceAll(excludeMac, ":", ""))
			normDevMac := strings.ToLower(strings.ReplaceAll(dev.MacFull, ":", ""))
			if normExclude == normDevMac {
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
		macPrefix := strings.ReplaceAll(dev.Mac, ".", "")
		macPrefix = macPrefix[:6]
		var formattedPrefix strings.Builder
		for i := 0; i < len(macPrefix); i += 2 {
			if i > 0 {
				formattedPrefix.WriteString(":")
			}
			formattedPrefix.WriteString(macPrefix[i : i+2])
		}
		targetVlan := sm.config.MacToVlan[formattedPrefix.String()]
		if targetVlan == "" {
			targetVlan = sm.config.DefaultVlan
			if sm.config.Debug {
				fmt.Printf("DEBUG: No VLAN mapping for %s on %s, using default %s\n", dev.MacFull, port.Interface, targetVlan)
			}
		}

		// Validate the target VLAN exists on the switch
		if !existingVLANs[targetVlan] {
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
	}
	return nil
}

func main() {
	yamlFile := flag.String("y", "config.yaml", "YAML configuration file")
	execute := flag.Bool("x", false, "Apply changes (disable sandbox)")
	debug := flag.Bool("d", false, "Enable debug logging")
	host := flag.String("h", "", "Switch host (overrides YAML)")
	replaceVlan := flag.String("r", "", "Replace VLAN (format: old,new)")
	flag.Parse()
	cfg, err := loadConfig(*yamlFile, *host, !*execute, *debug, *replaceVlan)
	if err != nil {
		log.Fatal(err)
	}
	sm := &SwitchManager{config: *cfg}
	err = sm.processPorts()
	if err != nil {
		log.Fatal(err)
	}
}
