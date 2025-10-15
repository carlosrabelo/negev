package entities

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
	DefaultVlan    string            `yaml:"default_vlan"`
	NoDataVlan     string            `yaml:"no_data_vlan"`
	Sandbox        bool
	VerbosityLevel int
	SkipVlanCheck  bool
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
