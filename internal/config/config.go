package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SwitchConfig defines the configuration for a single switch
type SwitchConfig struct {
	Target         string            `yaml:"target"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	ExcludePorts   []string          `yaml:"exclude_ports"`
	DefaultVlan    string            `yaml:"default_vlan"` // Default VLAN per switch
	NoDataVlan     string            `yaml:"no_data_vlan"` // Quarantine VLAN per switch
	Sandbox        bool
	VerbosityLevel int
	SkipVlanCheck  bool
	CreateVLANs    bool
}

// IsDebugEnabled returns true if debug logs are enabled (VerbosityLevel 1 or 3)
func (sc SwitchConfig) IsDebugEnabled() bool {
	return sc.VerbosityLevel == 1 || sc.VerbosityLevel == 3
}

// IsRawOutputEnabled returns true if raw switch output is enabled (VerbosityLevel 2 or 3)
func (sc SwitchConfig) IsRawOutputEnabled() bool {
	return sc.VerbosityLevel == 2 || sc.VerbosityLevel == 3
}

// Config defines the global configuration
type Config struct {
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"` // Global default VLAN
	NoDataVlan     string            `yaml:"no_data_vlan"` // Global quarantine VLAN
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	Switches       []SwitchConfig    `yaml:"switches"`
}

// NormalizeMAC normalizes a MAC address to a consistent format
func NormalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

// Load loads and validates the configuration from a YAML file
// The target parameter filters debug logs to the specified switch
func Load(yamlFile, target string, sandbox bool, verbosityLevel int, skipVlanCheck, createVLANs bool) (*Config, error) {
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

	// Validate global transport
	if cfg.Transport == "" {
		cfg.Transport = "telnet"
	}
	cfg.Transport = strings.ToLower(cfg.Transport)
	if cfg.Transport != "telnet" && cfg.Transport != "ssh" {
		return nil, fmt.Errorf("transport %s is invalid, must be 'telnet' or 'ssh'", cfg.Transport)
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

	// Log global configuration values
	if verbosityLevel == 1 || verbosityLevel == 3 {
		fmt.Printf("DEBUG: Global values: Transport=%s, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", cfg.Transport, cfg.DefaultVlan, cfg.NoDataVlan, cfg.ExcludeMacs)
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
		// Enable debug logs only for the target switch or if no target is specified
		switchVerbosity := verbosityLevel
		if target != "" && sw.Target != target {
			switchVerbosity = 0 // Disable logs for non-target switches
		}

		// Inherit or validate transport
		sw.Transport = strings.ToLower(strings.TrimSpace(sw.Transport))
		if sw.Transport == "" {
			sw.Transport = cfg.Transport
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No transport defined for switch %s, using global %s\n", sw.Target, cfg.Transport)
			}
		}
		if sw.Transport != "telnet" && sw.Transport != "ssh" {
			return nil, fmt.Errorf("transport %s is invalid for switch %s, must be 'telnet' or 'ssh'", sw.Transport, sw.Target)
		}

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
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No username defined for switch %s, using global %s\n", sw.Target, cfg.Username)
			}
		}
		if sw.Password == "" {
			sw.Password = cfg.Password
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No password defined for switch %s, using global %s\n", sw.Target, cfg.Password)
			}
		}
		if sw.EnablePassword == "" {
			sw.EnablePassword = cfg.EnablePassword
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No enable_password defined for switch %s, using global %s\n", sw.Target, cfg.EnablePassword)
			}
		}

		// Merge global and local exclude_macs
		normalizedExcludeMacs := make(map[string]bool)
		// Add global MACs
		for _, mac := range cfg.ExcludeMacs {
			normalizedExcludeMacs[NormalizeMAC(mac)] = true
		}
		// Add or override local MACs
		for _, mac := range sw.ExcludeMacs {
			normalizedExcludeMacs[NormalizeMAC(mac)] = true
		}
		// Convert back to list
		sw.ExcludeMacs = make([]string, 0, len(normalizedExcludeMacs))
		for mac := range normalizedExcludeMacs {
			sw.ExcludeMacs = append(sw.ExcludeMacs, mac)
		}
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Merged exclude_macs for switch %s: %v\n", sw.Target, sw.ExcludeMacs)
		}

		// Normalize exclude_ports
		if len(sw.ExcludePorts) > 0 {
			normalizedExcludePorts := make(map[string]struct{})
			for _, port := range sw.ExcludePorts {
				trimmed := strings.TrimSpace(port)
				if trimmed == "" {
					continue
				}
				normalizedExcludePorts[strings.ToLower(trimmed)] = struct{}{}
			}
			sw.ExcludePorts = make([]string, 0, len(normalizedExcludePorts))
			for port := range normalizedExcludePorts {
				sw.ExcludePorts = append(sw.ExcludePorts, port)
			}
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: Normalized exclude_ports for switch %s: %v\n", sw.Target, sw.ExcludePorts)
			}
		}

		// Merge global and local mac_to_vlan
		if sw.MacToVlan == nil {
			sw.MacToVlan = make(map[string]string)
		}
		newMacToVlan := make(map[string]string)
		// Process switch-specific mac_to_vlan entries first
		for mac, vlan := range sw.MacToVlan {
			if vlan == "0" || vlan == "00" {
				if switchVerbosity == 1 || switchVerbosity == 3 {
					fmt.Printf("DEBUG: Ignoring invalid VLAN mapping %s for MAC %s on switch %s\n", vlan, mac, sw.Target)
				}
				continue
			}
			if err := validateVLAN(vlan, fmt.Sprintf("mac_to_vlan for MAC %s on switch %s", mac, sw.Target)); err != nil {
				return nil, err
			}
			normalizedMac := NormalizeMAC(mac)
			newMacToVlan[normalizedMac[:6]] = vlan
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: Added switch-specific mac_to_vlan mapping for %s: %s on switch %s\n", normalizedMac[:6], vlan, sw.Target)
			}
		}
		// Add global mac_to_vlan entries only for prefixes not defined by the switch
		for mac, vlan := range cfg.MacToVlan {
			if vlan == "0" || vlan == "00" {
				if switchVerbosity == 1 || switchVerbosity == 3 {
					fmt.Printf("DEBUG: Ignoring invalid global VLAN mapping %s for MAC %s\n", vlan, mac)
				}
				continue
			}
			if err := validateVLAN(vlan, fmt.Sprintf("global mac_to_vlan for MAC %s", mac)); err != nil {
				if switchVerbosity == 1 || switchVerbosity == 3 {
					fmt.Printf("DEBUG: Skipping invalid global VLAN %s for MAC %s: %v\n", vlan, mac, err)
				}
				continue
			}
			normalizedMac := NormalizeMAC(mac)
			if _, exists := newMacToVlan[normalizedMac[:6]]; !exists {
				newMacToVlan[normalizedMac[:6]] = vlan
				if switchVerbosity == 1 || switchVerbosity == 3 {
					fmt.Printf("DEBUG: Inherited global mac_to_vlan mapping for %s: %s on switch %s\n", normalizedMac[:6], vlan, sw.Target)
				}
			}
		}
		sw.MacToVlan = newMacToVlan
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Merged mac_to_vlan for switch %s: %v\n", sw.Target, sw.MacToVlan)
		}

		// Inherit global default_vlan if not specified
		if sw.DefaultVlan == "" {
			sw.DefaultVlan = cfg.DefaultVlan
			if switchVerbosity == 1 || switchVerbosity == 3 {
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
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No no_data_vlan defined for switch %s, inheriting global %s\n", sw.Target, cfg.NoDataVlan)
			}
		}
		// Validate switch no_data_vlan
		if err := validateVLAN(sw.NoDataVlan, fmt.Sprintf("no_data_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Apply command-line flags
		sw.Sandbox = !sandbox // Invert logic: write=true → Sandbox=false, write=false → Sandbox=true
		sw.VerbosityLevel = verbosityLevel
		sw.SkipVlanCheck = skipVlanCheck
		sw.CreateVLANs = createVLANs

		// Log final configuration for the switch
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Final configuration for switch %s: Transport=%s, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v, Sandbox=%v, VerbosityLevel=%d\n", sw.Target, sw.Transport, sw.DefaultVlan, sw.NoDataVlan, sw.ExcludeMacs, sw.Sandbox, sw.VerbosityLevel)
		}

		// Update the switch configuration in the list
		cfg.Switches[i] = sw
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}

	return &cfg, nil
}
