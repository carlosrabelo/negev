package entities

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
