package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type switchEntry struct {
	Platform       string            `yaml:"platform"`
	Target         string            `yaml:"target"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	AllowedVlans   []string          `yaml:"allowed_vlans"`
	ProtectedVlans []string          `yaml:"protected_vlans"`
}

type Config struct {
	Platform       string            `yaml:"platform"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	DefaultVlan    string            `yaml:"default_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	AllowedVlans   []string          `yaml:"allowed_vlans"`
	ProtectedVlans []string          `yaml:"protected_vlans"`
	Switches       []switchEntry     `yaml:"switches"`
}

func debugf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Printf(format, args...)
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
	if cfg.Transport == "" {
		cfg.Transport = "telnet"
	}
	debugf(verbose, "DEBUG: Global values: Platform=%s, Transport=%s, DefaultVlan=%s\n",
		cfg.Platform, cfg.Transport, cfg.DefaultVlan)

	for i := range cfg.Switches {
		sw := &cfg.Switches[i]
		if sw.Transport == "" {
			sw.Transport = cfg.Transport
		}
		if sw.Platform == "" {
			sw.Platform = cfg.Platform
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
		if len(sw.AllowedVlans) == 0 {
			sw.AllowedVlans = cfg.AllowedVlans
		}
		if len(sw.ProtectedVlans) == 0 {
			sw.ProtectedVlans = cfg.ProtectedVlans
		}
		if len(sw.ExcludeMacs) == 0 {
			sw.ExcludeMacs = cfg.ExcludeMacs
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
