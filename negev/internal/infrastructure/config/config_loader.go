package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type switchEntry struct {
	Platform       string            `yaml:"platform"`
	LegacyPlatform string            `yaml:"vendor"`
	Target         string            `yaml:"target"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"`
	NoDataVlan     string            `yaml:"no_data_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	AllowedVlans   []string          `yaml:"allowed_vlans"`
	ProtectedVlans []string          `yaml:"protected_vlans"`
}

type Config struct {
	Platform       string            `yaml:"platform"`
	LegacyVendor   string            `yaml:"vendor"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"`
	NoDataVlan     string            `yaml:"no_data_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	AllowedVlans   []string          `yaml:"allowed_vlans"`
	ProtectedVlans []string          `yaml:"protected_vlans"`
	Switches       []switchEntry     `yaml:"switches"`
}


func mergeStringSlices(global, local []string, normalize func(string) (string, error)) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	add := func(v string) error {
		if normalize != nil {
			nv, err := normalize(v)
			if err != nil {
				return err
			}
			v = nv
		}
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
		return nil
	}
	for _, v := range global {
		if err := add(v); err != nil {
			return nil, err
		}
	}
	for _, v := range local {
		if err := add(v); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func debugf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

func normalizeVLAN(vlan, context string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(vlan))
	if err != nil {
		return "", fmt.Errorf("invalid VLAN number in %s: %s", context, vlan)
	}
	if n < 1 || n > 4094 {
		return "", fmt.Errorf("VLAN %s in %s must be between 1 and 4094", vlan, context)
	}
	return strconv.Itoa(n), nil
}

func validatePlatform(platform string) error {
	switch platform {
	case "ios", "auto":
		return nil
	default:
		return fmt.Errorf("platform %s is invalid, must be 'ios' or 'auto'", platform)
	}
}

func Load(yamlFile, target string, verbosityLevel int) (*Config, error) {
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

	if cfg.DefaultVlan == "" {
		return nil, fmt.Errorf("global default_vlan is required")
	}
	normalized, err := normalizeVLAN(cfg.DefaultVlan, "global default_vlan")
	if err != nil {
		return nil, err
	}
	cfg.DefaultVlan = normalized
	if cfg.NoDataVlan == "" {
		return nil, fmt.Errorf("global no_data_vlan is required")
	}
	normalized, err = normalizeVLAN(cfg.NoDataVlan, "global no_data_vlan")
	if err != nil {
		return nil, err
	}
	cfg.NoDataVlan = normalized
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
		var err error
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
		} else {
			normalized, err := normalizeVLAN(sw.DefaultVlan, fmt.Sprintf("switch %s default_vlan", sw.Target))
			if err != nil {
				return nil, err
			}
			sw.DefaultVlan = normalized
		}
		if sw.NoDataVlan == "" {
			sw.NoDataVlan = cfg.NoDataVlan
		} else {
			normalized, err := normalizeVLAN(sw.NoDataVlan, fmt.Sprintf("switch %s no_data_vlan", sw.Target))
			if err != nil {
				return nil, err
			}
			sw.NoDataVlan = normalized
		}
		sw.AllowedVlans, err = mergeStringSlices(cfg.AllowedVlans, sw.AllowedVlans, nil)
		if err != nil {
			return nil, err
		}
		sw.ProtectedVlans, err = mergeStringSlices(cfg.ProtectedVlans, sw.ProtectedVlans, nil)
		if err != nil {
			return nil, err
		}
		normalizedExclude := make(map[string]bool)
		for _, mac := range append(append([]string{}, cfg.ExcludeMacs...), sw.ExcludeMacs...) {
			normalizedExclude[NormalizeMAC(mac)] = true
		}
		sw.ExcludeMacs = make([]string, 0, len(normalizedExclude))
		for mac := range normalizedExclude {
			sw.ExcludeMacs = append(sw.ExcludeMacs, mac)
		}
		if len(sw.MacToVlan) == 0 {
			sw.MacToVlan = cfg.MacToVlan
		}
		_ = target
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}
	return &cfg, nil
}

func NormalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

func macPrefix(mac string) string {
	n := NormalizeMAC(mac)
	if len(n) > 6 {
		return n[:6]
	}
	return n
}
