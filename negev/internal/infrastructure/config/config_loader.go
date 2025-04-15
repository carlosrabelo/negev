package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"gopkg.in/yaml.v3"
)

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

func debugf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

func mergeStringSlices(global, local []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range global {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	for _, v := range local {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

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

func Load(yamlFile, target string, sandbox bool, verbosityLevel int, createVLANs bool) (*Config, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %v", yamlFile, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	verbose := verbosityLevel == 1 || verbosityLevel == 3

	primaryPlatform := cfg.Platform
	if primaryPlatform == "" {
		primaryPlatform = cfg.LegacyVendor
	}
	cfg.Platform = strings.ToLower(strings.TrimSpace(primaryPlatform))

	if cfg.Transport == "" {
		cfg.Transport = "telnet"
	}
	cfg.Transport = strings.ToLower(cfg.Transport)
	if cfg.Transport != "telnet" && cfg.Transport != "ssh" {
		return nil, fmt.Errorf("transport %s is invalid, must be 'telnet' or 'ssh'", cfg.Transport)
	}

	validateVLAN := func(vlan string, context string) error {
		n, err := strconv.Atoi(vlan)
		if err != nil {
			return fmt.Errorf("invalid VLAN number in %s: %s", context, vlan)
		}
		if n < 1 || n > 4094 {
			return fmt.Errorf("VLAN %s in %s must be between 1 and 4094", vlan, context)
		}
		return nil
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
	if cfg.Username == "" {
		return nil, fmt.Errorf("global username is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("global password is required")
	}
	if cfg.EnablePassword == "" {
		return nil, fmt.Errorf("global enable_password is required")
	}

	debugf(verbose, "DEBUG: Global values: Platform=%s, Transport=%s, DefaultVlan=%s, NoDataVlan=%s\n",
		cfg.Platform, cfg.Transport, cfg.DefaultVlan, cfg.NoDataVlan)

	for i := range cfg.Switches {
		sw := &cfg.Switches[i]
		if sw.Target == "" {
			return nil, fmt.Errorf("target is required for switch %d", i)
		}
		sw.Transport = strings.ToLower(strings.TrimSpace(sw.Transport))
		if sw.Transport == "" {
			sw.Transport = cfg.Transport
		}
		if sw.Transport != "telnet" && sw.Transport != "ssh" {
			return nil, fmt.Errorf("transport %s is invalid for switch %s", sw.Transport, sw.Target)
		}

		rawPlatform := sw.Platform
		if rawPlatform == "" {
			rawPlatform = sw.LegacyPlatform
		}
		sw.Platform = strings.ToLower(strings.TrimSpace(rawPlatform))
		if sw.Platform == "" {
			sw.Platform = cfg.Platform
		}
		if sw.Platform == "" {
			return nil, fmt.Errorf("platform is required for switch %s", sw.Target)
		}
		if err := validatePlatform(sw.Platform); err != nil {
			return nil, fmt.Errorf("invalid platform for switch %s: %w", sw.Target, err)
		}

		if sw.Username == "" {
			sw.Username = cfg.Username
		}
		if sw.Password == "" {
			sw.Password = cfg.Password
		}
		if sw.EnablePassword == "" {
			sw.EnablePassword = cfg.EnablePassword
		}
		if sw.DefaultVlan == "" {
			sw.DefaultVlan = cfg.DefaultVlan
		}
		if err := validateVLAN(sw.DefaultVlan, fmt.Sprintf("default_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}
		if sw.NoDataVlan == "" {
			sw.NoDataVlan = cfg.NoDataVlan
		}
		if err := validateVLAN(sw.NoDataVlan, fmt.Sprintf("no_data_vlan for switch %s", sw.Target)); err != nil {
			return nil, err
		}

		sw.AllowedVlans = mergeStringSlices(cfg.AllowedVlans, sw.AllowedVlans)
		sw.ProtectedVlans = mergeStringSlices(cfg.ProtectedVlans, sw.ProtectedVlans)

		sw.Sandbox = !sandbox
		sw.VerbosityLevel = verbosityLevel
		sw.CreateVLANs = createVLANs

		debugf(verbose, "DEBUG: Switch %s: Platform=%s, Transport=%s, DefaultVlan=%s\n",
			sw.Target, sw.Platform, sw.Transport, sw.DefaultVlan)
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}

	return &cfg, nil
}
