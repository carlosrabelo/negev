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
	Platform              string                  `yaml:"platform"`
	LegacyVendor          string                  `yaml:"vendor"`
	Transport             string                  `yaml:"transport"`
	Username              string                  `yaml:"username"`
	Password              string                  `yaml:"password"`
	EnablePassword        string                  `yaml:"enable_password"`
	PasswordCommand       string                  `yaml:"password_command"`
	EnablePasswordCommand string                  `yaml:"enable_password_command"`
	DefaultVlan           string                  `yaml:"default_vlan"`
	NoDataVlan            string                  `yaml:"no_data_vlan"`
	ExcludeMacs           []string                `yaml:"exclude_macs"`
	MacToVlan             map[string]string       `yaml:"mac_to_vlan"`
	AllowedVlans          []string                `yaml:"allowed_vlans"`
	ProtectedVlans        []string                `yaml:"protected_vlans"`
	MultiMacPolicy        string                  `yaml:"multi_mac_policy"`
	Switches              []entities.SwitchConfig `yaml:"switches"`
}

func debugf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format, args...)
	}
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

func NormalizeMAC(mac string) string {
	mac = strings.ToLower(mac)
	return strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac)
}

func isHexMAC(s string) bool {
	if len(s) < 6 {
		return false
	}
	return isHexString(s)
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// normalizeMacPrefix accepts even-length hex prefixes from 2 to 12 characters.
func normalizeMacPrefix(prefix string) (string, error) {
	norm := NormalizeMAC(prefix)
	if !isHexString(norm) {
		return "", fmt.Errorf("must be hexadecimal")
	}
	if len(norm) < 2 || len(norm) > 12 || len(norm)%2 != 0 {
		return "", fmt.Errorf("length must be even and between 2 and 12")
	}
	return norm, nil
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

func canonicalizePlatform(platform string) string {
	if platform == "dmos" {
		return "dmswitch"
	}
	return platform
}

func validatePlatform(platform string) error {
	switch platform {
	case "ios", "nxos", "dmswitch", "dm1200", "comware", "auto":
		return nil
	default:
		return fmt.Errorf("platform %s is invalid, must be 'ios', 'nxos', 'dmswitch', 'dm1200', 'comware', or 'auto'", platform)
	}
}

func normalizeMultiMacPolicy(policy, context string) (string, error) {
	policy = strings.ToLower(strings.TrimSpace(policy))
	if policy == "" {
		return entities.MultiMacIgnore, nil
	}
	switch policy {
	case entities.MultiMacIgnore, entities.MultiMacAlert, entities.MultiMacPriority:
		return policy, nil
	default:
		return "", fmt.Errorf("%s multi_mac_policy %q is invalid, must be 'ignore', 'alert', or 'priority'", context, policy)
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

	cfg.Username, cfg.Password, cfg.EnablePassword, err = expandCredentialFields(cfg.Username, cfg.Password, cfg.EnablePassword)
	if err != nil {
		return nil, err
	}
	if err := resolvePasswordCommands(&cfg.Password, cfg.PasswordCommand, &cfg.EnablePassword, cfg.EnablePasswordCommand); err != nil {
		return nil, err
	}
	for i := range cfg.Switches {
		sw := &cfg.Switches[i]
		sw.Username, sw.Password, sw.EnablePassword, err = expandCredentialFields(sw.Username, sw.Password, sw.EnablePassword)
		if err != nil {
			return nil, err
		}
	}

	primaryPlatform := cfg.Platform
	if primaryPlatform == "" {
		primaryPlatform = cfg.LegacyVendor
	}
	cfg.Platform = canonicalizePlatform(strings.ToLower(strings.TrimSpace(primaryPlatform)))

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
	if cfg.NoDataVlan != "" {
		normalized, err = normalizeVLAN(cfg.NoDataVlan, "global no_data_vlan")
		if err != nil {
			return nil, err
		}
		cfg.NoDataVlan = normalized
		if cfg.NoDataVlan == cfg.DefaultVlan {
			debugf(verbose, "DEBUG: no_data_vlan equals default_vlan (%s); quarantine and fallback are the same VLAN\n", cfg.NoDataVlan)
		}
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("global username is required")
	}

	cfg.MultiMacPolicy, err = normalizeMultiMacPolicy(cfg.MultiMacPolicy, "global")
	if err != nil {
		return nil, err
	}

	debugf(verbose, "DEBUG: Global values: Platform=%s, Transport=%s, DefaultVlan=%s, NoDataVlan=%s\n",
		cfg.Platform, cfg.Transport, cfg.DefaultVlan, cfg.NoDataVlan)

	for i := range cfg.Switches {
		sw := &cfg.Switches[i]
		swVerbose := verbose
		if target != "" && sw.Target != target {
			swVerbose = false
		}
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
		sw.Platform = canonicalizePlatform(strings.ToLower(strings.TrimSpace(rawPlatform)))
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
			if sw.PasswordCommand != "" {
				sw.Password, err = runPasswordCommand(sw.PasswordCommand)
				if err != nil {
					return nil, err
				}
			} else {
				sw.Password = cfg.Password
			}
		}
		if sw.EnablePassword == "" {
			if sw.EnablePasswordCommand != "" {
				sw.EnablePassword, err = runPasswordCommand(sw.EnablePasswordCommand)
				if err != nil {
					return nil, err
				}
			} else {
				sw.EnablePassword = cfg.EnablePassword
			}
		}

		if sw.MultiMacPolicy == "" {
			sw.MultiMacPolicy = cfg.MultiMacPolicy
		} else {
			sw.MultiMacPolicy, err = normalizeMultiMacPolicy(sw.MultiMacPolicy, fmt.Sprintf("switch %s", sw.Target))
			if err != nil {
				return nil, err
			}
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
		if sw.NoDataVlan != "" && sw.NoDataVlan == sw.DefaultVlan {
			debugf(swVerbose, "DEBUG: switch %s no_data_vlan equals default_vlan (%s); quarantine and fallback are the same VLAN\n", sw.Target, sw.NoDataVlan)
		}

		var err error
		sw.AllowedVlans, err = mergeStringSlices(cfg.AllowedVlans, sw.AllowedVlans, func(v string) (string, error) {
			return normalizeVLAN(v, fmt.Sprintf("switch %s allowed_vlans", sw.Target))
		})
		if err != nil {
			return nil, err
		}

		sw.ProtectedVlans, err = mergeStringSlices(cfg.ProtectedVlans, sw.ProtectedVlans, func(v string) (string, error) {
			return normalizeVLAN(v, fmt.Sprintf("switch %s protected_vlans", sw.Target))
		})
		if err != nil {
			return nil, err
		}

		normalizedExclude := make(map[string]bool)
		for _, mac := range append(append([]string{}, cfg.ExcludeMacs...), sw.ExcludeMacs...) {
			norm := NormalizeMAC(mac)
			if !isHexMAC(norm) {
				return nil, fmt.Errorf("invalid MAC %q for switch %s", mac, sw.Target)
			}
			normalizedExclude[norm] = true
		}
		sw.ExcludeMacs = make([]string, 0, len(normalizedExclude))
		for mac := range normalizedExclude {
			sw.ExcludeMacs = append(sw.ExcludeMacs, mac)
		}

		// Normalizar ExcludePorts: trim, lowercase, dedup
		seenPorts := make(map[string]bool)
		var normalizedPorts []string
		for _, p := range sw.ExcludePorts {
			pNorm := strings.ToLower(strings.TrimSpace(p))
			if pNorm != "" && !seenPorts[pNorm] {
				seenPorts[pNorm] = true
				normalizedPorts = append(normalizedPorts, pNorm)
			}
		}
		sw.ExcludePorts = normalizedPorts

		// Mesclar MacToVlan: chaves do switch sobrescrevem globais, chaves e valores normalizados e validados
		mergedMacToVlan := make(map[string]string)
		addMacMapping := func(prefix, vlan, context string, removeZero bool) error {
			norm, err := normalizeMacPrefix(prefix)
			if err != nil {
				return fmt.Errorf("invalid MAC prefix %q in %s: %w", prefix, context, err)
			}
			if vlan == "0" || vlan == "00" || vlan == "" {
				if removeZero {
					delete(mergedMacToVlan, norm)
				}
				return nil
			}
			canon, err := normalizeVLAN(vlan, context)
			if err != nil {
				return err
			}
			mergedMacToVlan[norm] = canon
			return nil
		}
		for prefix, vlan := range cfg.MacToVlan {
			if err := addMacMapping(prefix, vlan, fmt.Sprintf("global mac_to_vlan prefix %s", prefix), false); err != nil {
				return nil, err
			}
		}
		for prefix, vlan := range sw.MacToVlan {
			if err := addMacMapping(prefix, vlan, fmt.Sprintf("switch %s mac_to_vlan prefix %s", sw.Target, prefix), true); err != nil {
				return nil, err
			}
		}
		sw.MacToVlan = mergedMacToVlan
		debugf(swVerbose, "DEBUG: Merged exclude_macs for %s: %v\n", sw.Target, sw.ExcludeMacs)
		debugf(swVerbose, "DEBUG: Merged mac_to_vlan for %s: %v\n", sw.Target, sw.MacToVlan)
		debugf(swVerbose, "DEBUG: Normalized exclude_ports for %s: %v\n", sw.Target, sw.ExcludePorts)
		debugf(swVerbose, "DEBUG: Switch %s: Platform=%s, Transport=%s, DefaultVlan=%s\n",
			sw.Target, sw.Platform, sw.Transport, sw.DefaultVlan)
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the YAML configuration")
	}

	return &cfg, nil
}
