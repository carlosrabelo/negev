package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"gopkg.in/yaml.v3"
)

// Config defines the global configuration
type Config struct {
	Platform       string                  `yaml:"platform"`
	LegacyVendor   string                  `yaml:"vendor"`
	Transport      string                  `yaml:"transport"`
	Username       string                  `yaml:"username"`
	Password       string                  `yaml:"password"`
	EnablePassword string                  `yaml:"enable_password"`
	DefaultVlan    string                  `yaml:"default_vlan"`
	NoDataVlan     string                  `yaml:"no_data_vlan"`
	ExcludeMacs    []string                `yaml:"exclude_macs"`
	MacToVlan      map[string]string       `yaml:"mac_to_vlan"`
	AllowedVlans   []string                `yaml:"allowed_vlans"`
	ProtectedVlans []string                `yaml:"protected_vlans"`
	Switches       []entities.SwitchConfig `yaml:"switches"`
}

// NormalizeMAC normalizes a MAC address to consistent format
func NormalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

func validatePlatform(platform string) error {
	switch platform {
	case "ios", "dmos", "auto":
		return nil
	default:
		return fmt.Errorf("platform %s is invalid, must be 'ios', 'dmos', or 'auto'", platform)
	}
}

// Load loads and validates configuration from a YAML file
func Load(yamlFile, target string, sandbox bool, verbosityLevel int, createVLANs bool) (*Config, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %v", yamlFile, err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
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

	primaryPlatform := cfg.Platform
	if primaryPlatform == "" {
		primaryPlatform = cfg.LegacyVendor
	}
	cfg.Platform = strings.ToLower(strings.TrimSpace(primaryPlatform))
	if cfg.Platform == "" {
		cfg.Platform = "ios"
	}
	if err := validatePlatform(cfg.Platform); err != nil {
		return nil, err
	}

	if cfg.Transport == "" {
		cfg.Transport = "telnet"
	}
	cfg.Transport = strings.ToLower(cfg.Transport)
	if cfg.Transport != "telnet" && cfg.Transport != "ssh" {
		return nil, fmt.Errorf("transport %s is invalid, must be 'telnet' or 'ssh'", cfg.Transport)
	}

	if cfg.DefaultVlan == "" {
		return nil, fmt.Errorf("global default_vlan is required")
	}
	if err := validateVLAN(cfg.DefaultVlan, "global default_vlan"); err != nil {
		return nil, err
	}

	if cfg.NoDataVlan == "" {
		return nil, fmt.Errorf("global no_data_vlan is required")
	}
	if err := validateVLAN(cfg.NoDataVlan, "global no_data_vlan"); err != nil {
		return nil, err
	}

	// Validate allowed_vlans if provided
	for i, vlan := range cfg.AllowedVlans {
		if err := validateVLAN(vlan, fmt.Sprintf("global allowed_vlans[%d]", i)); err != nil {
			return nil, err
		}
	}

	// Validate protected_vlans if provided
	for i, vlan := range cfg.ProtectedVlans {
		if err := validateVLAN(vlan, fmt.Sprintf("global protected_vlans[%d]", i)); err != nil {
			return nil, err
		}
	}

	if verbosityLevel == 1 || verbosityLevel == 3 {
		fmt.Printf("DEBUG: Global values: Platform=%s, Transport=%s, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", cfg.Platform, cfg.Transport, cfg.DefaultVlan, cfg.NoDataVlan, cfg.ExcludeMacs)
	}

	if cfg.Username == "" {
		return nil, fmt.Errorf("global username is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("global password is required")
	}
	if cfg.EnablePassword == "" {
		return nil, fmt.Errorf("global enable_password is required")
	}

	for i, sw := range cfg.Switches {
		switchVerbosity := verbosityLevel
		if target != "" && sw.Target != target {
			switchVerbosity = 0
		}

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

		rawPlatform := sw.Platform
		if rawPlatform == "" {
			rawPlatform = sw.LegacyPlatform
		}
		sw.Platform = strings.ToLower(strings.TrimSpace(rawPlatform))
		if sw.Platform == "" {
			sw.Platform = cfg.Platform
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No platform defined for switch %s, using global %s\n", sw.Target, cfg.Platform)
			}
		}
		if err := validatePlatform(sw.Platform); err != nil {
			return nil, fmt.Errorf("invalid platform for switch %s: %w", sw.Target, err)
		}

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

		normalizedExcludeMacs := make(map[string]bool)
		for _, mac := range cfg.ExcludeMacs {
			normalizedExcludeMacs[NormalizeMAC(mac)] = true
		}
		for _, mac := range sw.ExcludeMacs {
			normalizedExcludeMacs[NormalizeMAC(mac)] = true
		}
		sw.ExcludeMacs = make([]string, 0, len(normalizedExcludeMacs))
		for mac := range normalizedExcludeMacs {
			sw.ExcludeMacs = append(sw.ExcludeMacs, mac)
		}
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Merged exclude_macs for switch %s: %v\n", sw.Target, sw.ExcludeMacs)
		}

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

		if sw.MacToVlan == nil {
			sw.MacToVlan = make(map[string]string)
		}
		newMacToVlan := make(map[string]string)
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

		if sw.DefaultVlan == "" {
			sw.DefaultVlan = cfg.DefaultVlan
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No default_vlan defined for switch %s, inheriting global %s\n", sw.Target, cfg.DefaultVlan)
			}
		}
		if err := validateVLAN(sw.DefaultVlan, fmt.Sprintf("default_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		if sw.NoDataVlan == "" {
			sw.NoDataVlan = cfg.NoDataVlan
			if switchVerbosity == 1 || switchVerbosity == 3 {
				fmt.Printf("DEBUG: No no_data_vlan defined for switch %s, inheriting global %s\n", sw.Target, cfg.NoDataVlan)
			}
		}
		if err := validateVLAN(sw.NoDataVlan, fmt.Sprintf("no_data_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Merge allowed_vlans from global and switch-specific
		normalizedAllowedVlans := make(map[string]bool)
		for _, vlan := range cfg.AllowedVlans {
			normalizedAllowedVlans[vlan] = true
		}
		for _, vlan := range sw.AllowedVlans {
			if err := validateVLAN(vlan, fmt.Sprintf("allowed_vlans for switch %s", sw.Target)); err != nil {
				return nil, err
			}
			normalizedAllowedVlans[vlan] = true
		}
		sw.AllowedVlans = make([]string, 0, len(normalizedAllowedVlans))
		for vlan := range normalizedAllowedVlans {
			sw.AllowedVlans = append(sw.AllowedVlans, vlan)
		}
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Merged allowed_vlans for switch %s: %v\n", sw.Target, sw.AllowedVlans)
		}

		// Merge protected_vlans from global and switch-specific
		normalizedProtectedVlans := make(map[string]bool)
		for _, vlan := range cfg.ProtectedVlans {
			normalizedProtectedVlans[vlan] = true
		}
		for _, vlan := range sw.ProtectedVlans {
			if err := validateVLAN(vlan, fmt.Sprintf("protected_vlans for switch %s", sw.Target)); err != nil {
				return nil, err
			}
			normalizedProtectedVlans[vlan] = true
		}
		sw.ProtectedVlans = make([]string, 0, len(normalizedProtectedVlans))
		for vlan := range normalizedProtectedVlans {
			sw.ProtectedVlans = append(sw.ProtectedVlans, vlan)
		}
		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Merged protected_vlans for switch %s: %v\n", sw.Target, sw.ProtectedVlans)
		}

		sw.Sandbox = !sandbox
		sw.VerbosityLevel = verbosityLevel
		sw.CreateVLANs = createVLANs

		if switchVerbosity == 1 || switchVerbosity == 3 {
			fmt.Printf("DEBUG: Final configuration for switch %s: Platform=%s, Transport=%s, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v, Sandbox=%v, VerbosityLevel=%d\n", sw.Target, sw.Platform, sw.Transport, sw.DefaultVlan, sw.NoDataVlan, sw.ExcludeMacs, sw.Sandbox, sw.VerbosityLevel)
		}

		cfg.Switches[i] = sw
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}

	return &cfg, nil
}
