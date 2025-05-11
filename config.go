package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SwitchConfig defines the configuration for a single switch
type SwitchConfig struct {
	Target         string            `yaml:"target"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	DefaultVlan    string            `yaml:"default_vlan"` // Default VLAN per switch
	NoDataVlan     string            `yaml:"no_data_vlan"` // Quarantine VLAN per switch
	Sandbox        bool
	Verbose        bool
	Extra          bool
	SkipVlanCheck  bool
	CreateVLANs    bool
}

// Config defines the global configuration
type Config struct {
	ServerIP       string            `yaml:"server_ip"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	SnmpCommunity  string            `yaml:"snmp_community"` // SNMP community
	SnmpPort       int               `yaml:"snmp_port"`      // SNMP port for traps
	DefaultVlan    string            `yaml:"default_vlan"`   // Global default VLAN
	NoDataVlan     string            `yaml:"no_data_vlan"`   // Global quarantine VLAN
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	Switches       []SwitchConfig    `yaml:"switches"`
}

// Port represents a switch port with its VLAN
type Port struct {
	Interface string
	Vlan      string
}

// Device represents a device connected to a switch port
type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

// normalizeMac normalizes a MAC address to a consistent format
func normalizeMac(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

// getMacList returns a list of full MAC addresses from a device slice
func getMacList(devices []Device) []string {
	var macs []string
	for _, d := range devices {
		macs = append(macs, d.MacFull)
	}
	return macs
}

// loadConfig loads and validates the configuration from a YAML file
func loadConfig(yamlFile string, sandbox, verbose, extra, skipVlanCheck, createVLANs bool) (*Config, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %v", yamlFile, err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	// validateVLAN validates a VLAN number
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

	// Validate server_ip
	if cfg.ServerIP == "" {
		return nil, fmt.Errorf("server_ip is required in the YAML configuration")
	}
	if net.ParseIP(cfg.ServerIP) == nil {
		return nil, fmt.Errorf("server_ip %s is not a valid IP address", cfg.ServerIP)
	}
	// Ensure it's IPv4
	ip := net.ParseIP(cfg.ServerIP).To4()
	if ip == nil {
		return nil, fmt.Errorf("server_ip %s must be an IPv4 address", cfg.ServerIP)
	}

	// Validate snmp_community
	if cfg.SnmpCommunity == "" {
		cfg.SnmpCommunity = "public" // Default value
		if verbose {
			fmt.Printf("DEBUG: No snmp_community defined, using default 'public'\n")
		}
	}

	// Validate snmp_port
	if cfg.SnmpPort == 0 {
		cfg.SnmpPort = 162 // Default value
		if verbose {
			fmt.Printf("DEBUG: No snmp_port defined, using default 162\n")
		}
	} else if cfg.SnmpPort < 1 || cfg.SnmpPort > 65535 {
		return nil, fmt.Errorf("snmp_port %d is invalid, must be between 1 and 65535", cfg.SnmpPort)
	}

	// Validate global default_vlan
	if cfg.DefaultVlan == "" {
		return nil, fmt.Errorf("global default_vlan is required")
	}
	if err := validateVLAN(cfg.DefaultVlan, "global default_vlan"); err != nil {
		return nil, err
	}

	// Validate global no_data_vlan
	if cfg.NoDataVlan == "" {
		return nil, fmt.Errorf("global no_data_vlan is required")
	}
	if err := validateVLAN(cfg.NoDataVlan, "global no_data_vlan"); err != nil {
		return nil, err
	}

	// Log global values
	if verbose {
		fmt.Printf("DEBUG: Global values: ServerIP=%s, SnmpCommunity=%s, SnmpPort=%d, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", cfg.ServerIP, cfg.SnmpCommunity, cfg.SnmpPort, cfg.DefaultVlan, cfg.NoDataVlan, cfg.ExcludeMacs)
	}

	// Validate global fields used as fallback
	if cfg.Username == "" {
		return nil, fmt.Errorf("global username is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("global password is required")
	}
	if cfg.EnablePassword == "" {
		return nil, fmt.Errorf("global enable_password is required")
	}

	// Process each switch configuration
	for i, sw := range cfg.Switches {
		// Validate required fields
		if sw.Target == "" {
			return nil, fmt.Errorf("target is required for switch %d", i)
		}
		if sw.Username == "" && cfg.Username == "" {
			return nil, fmt.Errorf("username is required for switch %d", i)
		}
		if sw.Password == "" && cfg.Password == "" {
			return nil, fmt.Errorf("password is required for switch %d", i)
		}
		if sw.EnablePassword == "" && cfg.EnablePassword == "" {
			return nil, fmt.Errorf("enable_password is required for switch %d", i)
		}

		// Apply global defaults
		if sw.Username == "" {
			sw.Username = cfg.Username
			if verbose {
				fmt.Printf("DEBUG: No username defined for switch %s, using global %s\n", sw.Target, cfg.Username)
			}
		}
		if sw.Password == "" {
			sw.Password = cfg.Password
			if verbose {
				fmt.Printf("DEBUG: No password defined for switch %s, using global %s\n", sw.Target, cfg.Password)
			}
		}
		if sw.EnablePassword == "" {
			sw.EnablePassword = cfg.EnablePassword
			if verbose {
				fmt.Printf("DEBUG: No enable_password defined for switch %s, using global %s\n", sw.Target, cfg.EnablePassword)
			}
		}

		// Merge global and local exclude_macs
		normalizedExcludeMacs := make(map[string]bool)
		// Add global MACs
		for _, mac := range cfg.ExcludeMacs {
			normalizedExcludeMacs[normalizeMac(mac)] = true
		}
		// Add or override local MACs
		for _, mac := range sw.ExcludeMacs {
			normalizedExcludeMacs[normalizeMac(mac)] = true
		}
		// Convert back to list
		sw.ExcludeMacs = make([]string, 0, len(normalizedExcludeMacs))
		for mac := range normalizedExcludeMacs {
			sw.ExcludeMacs = append(sw.ExcludeMacs, mac)
		}
		if verbose {
			fmt.Printf("DEBUG: Merged exclude_macs for switch %s: %v\n", sw.Target, sw.ExcludeMacs)
		}

		// Merge global and local mac_to_vlan
		if sw.MacToVlan == nil {
			sw.MacToVlan = make(map[string]string)
		}
		// Copy global entries
		for mac, vlan := range cfg.MacToVlan {
			sw.MacToVlan[mac] = vlan
		}
		// Normalize and validate local entries (overriding or adding)
		newMacToVlan := make(map[string]string)
		for mac, vlan := range sw.MacToVlan {
			if vlan == "0" || vlan == "00" {
				if verbose {
					fmt.Printf("DEBUG: Ignoring invalid VLAN mapping %s for MAC %s on switch %s\n", vlan, mac, sw.Target)
				}
				continue
			}
			if err := validateVLAN(vlan, fmt.Sprintf("mac_to_vlan for MAC %s on switch %s", mac, sw.Target)); err != nil {
				return nil, err
			}
			normalizedMac := normalizeMac(mac)
			newMacToVlan[normalizedMac[:6]] = vlan
		}
		// Merge global and local entries, with local entries taking precedence
		for mac, vlan := range cfg.MacToVlan {
			normalizedMac := normalizeMac(mac)
			if _, exists := newMacToVlan[normalizedMac[:6]]; !exists {
				newMacToVlan[normalizedMac[:6]] = vlan
			}
		}
		sw.MacToVlan = newMacToVlan
		if verbose {
			fmt.Printf("DEBUG: Merged mac_to_vlan for switch %s: %v\n", sw.Target, sw.MacToVlan)
		}

		// Inherit global default_vlan if not specified
		if sw.DefaultVlan == "" {
			sw.DefaultVlan = cfg.DefaultVlan
			if verbose {
				fmt.Printf("DEBUG: No default_vlan defined for switch %s, inheriting global %s\n", sw.Target, cfg.DefaultVlan)
			}
		}
		// Validate switch default_vlan
		if err := validateVLAN(sw.DefaultVlan, fmt.Sprintf("default_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Inherit global no_data_vlan if not specified
		if sw.NoDataVlan == "" {
			sw.NoDataVlan = cfg.NoDataVlan
			if verbose {
				fmt.Printf("DEBUG: No no_data_vlan defined for switch %s, inheriting global %s\n", sw.Target, cfg.NoDataVlan)
			}
		}
		// Validate switch no_data_vlan
		if err := validateVLAN(sw.NoDataVlan, fmt.Sprintf("no_data_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Apply command-line flags
		sw.Sandbox = !sandbox // Invert logic: write=true → Sandbox=false, write=false → Sandbox=true
		sw.Verbose = verbose
		sw.Extra = extra
		sw.SkipVlanCheck = skipVlanCheck
		sw.CreateVLANs = createVLANs

		// Log final configuration for the switch
		if verbose {
			fmt.Printf("DEBUG: Final configuration for switch %s: DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v, Sandbox=%v\n", sw.Target, sw.DefaultVlan, sw.NoDataVlan, sw.ExcludeMacs, sw.Sandbox)
		}

		// Update the switch configuration in the list
		cfg.Switches[i] = sw
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}

	return &cfg, nil
}
