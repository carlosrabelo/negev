package entities

import "strings"

// SwitchConfig defines the configuration for a single switch
type SwitchConfig struct {
	Platform       string            `yaml:"platform"`
	LegacyPlatform string            `yaml:"vendor"`
	Target         string            `yaml:"target"`
	Transport      string            `yaml:"transport"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	ExcludePorts   []string          `yaml:"exclude_ports"`
	DefaultVlan    string            `yaml:"default_vlan"`
	NoDataVlan     string            `yaml:"no_data_vlan"`
	AllowedVlans   []string          `yaml:"allowed_vlans"`
	ProtectedVlans []string          `yaml:"protected_vlans"`
	Sandbox        bool
	VerbosityLevel int
	CreateVLANs    bool
}

// IsDebugEnabled returns true if debug logs are enabled
func (sc SwitchConfig) IsDebugEnabled() bool {
	return sc.VerbosityLevel == 1 || sc.VerbosityLevel == 3
}

// IsRawOutputEnabled returns true if raw switch output is enabled
func (sc SwitchConfig) IsRawOutputEnabled() bool {
	return sc.VerbosityLevel == 2 || sc.VerbosityLevel == 3
}

// PlatformID returns the normalized platform identifier, defaulting to ios.
func (sc SwitchConfig) PlatformID() string {
	platform := strings.ToLower(strings.TrimSpace(sc.Platform))
	if platform == "" {
		platform = strings.ToLower(strings.TrimSpace(sc.LegacyPlatform))
	}
	if platform == "" {
		return "ios"
	}
	return platform
}
