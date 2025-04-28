package entities

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// JumpHostConfig defines an SSH bastion/jump host used to tunnel switch connections.
type JumpHostConfig struct {
	Host                 string `yaml:"host"`
	Port                 int    `yaml:"port"`
	Username             string `yaml:"username"`
	Password             string `yaml:"password"`
	PasswordCommand      string `yaml:"password_command"`
	KeyFile              string `yaml:"key_file"`
	KeyPassphrase        string `yaml:"key_passphrase"`
	KeyPassphraseCommand string `yaml:"key_passphrase_command"`
	KnownHosts           string `yaml:"known_hosts"`
	Disabled             bool   `yaml:"disabled"`
}

// IsEnabled reports whether jump host tunneling is active for this configuration.
func (j *JumpHostConfig) IsEnabled() bool {
	return j != nil && !j.Disabled && strings.TrimSpace(j.Host) != ""
}

// EffectivePort returns the configured SSH port or the default 22.
func (j *JumpHostConfig) EffectivePort() int {
	if j != nil && j.Port > 0 {
		return j.Port
	}
	return 22
}

// Addr returns the host:port address formatted for dialing and known_hosts matching.
func (j *JumpHostConfig) Addr() string {
	if j == nil {
		return ""
	}
	return net.JoinHostPort(j.Host, strconv.Itoa(j.EffectivePort()))
}

// Clone returns a shallow copy of the JumpHostConfig, or nil when receiver is nil.
func (j *JumpHostConfig) Clone() *JumpHostConfig {
	if j == nil {
		return nil
	}
	cp := *j
	return &cp
}

// ParseJumpSpec parses a CLI or scalar YAML jump host specification such as
// "[user@]host[:port]", "ssh://[user@]host[:port]", or "none"/"off"/"direct"/"false".
func ParseJumpSpec(spec string) (*JumpHostConfig, error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return nil, fmt.Errorf("empty jump host specification")
	}

	switch strings.ToLower(s) {
	case "none", "off", "direct", "false":
		return &JumpHostConfig{Disabled: true}, nil
	}

	if len(s) >= 6 && strings.EqualFold(s[:6], "ssh://") {
		s = s[6:]
	}

	if s == "" || strings.ContainsAny(s, " \t\r\n/") {
		return nil, fmt.Errorf("invalid jump host specification %q", spec)
	}

	var user string
	hostPort := s
	if idx := strings.LastIndex(s, "@"); idx >= 0 {
		user = s[:idx]
		hostPort = s[idx+1:]
		if user == "" || hostPort == "" {
			return nil, fmt.Errorf("invalid jump host specification %q", spec)
		}
	}

	var host, portStr string
	switch {
	case strings.HasPrefix(hostPort, "["):
		if strings.Contains(hostPort, "]:") {
			var err error
			host, portStr, err = net.SplitHostPort(hostPort)
			if err != nil {
				return nil, fmt.Errorf("invalid jump host specification %q: %w", spec, err)
			}
		} else if strings.HasSuffix(hostPort, "]") {
			host = hostPort[1 : len(hostPort)-1]
			if net.ParseIP(host) == nil {
				return nil, fmt.Errorf("invalid jump host IPv6 address in %q", spec)
			}
		} else {
			return nil, fmt.Errorf("invalid jump host specification %q", spec)
		}
	case strings.Count(hostPort, ":") == 1:
		var err error
		host, portStr, err = net.SplitHostPort(hostPort)
		if err != nil {
			return nil, fmt.Errorf("invalid jump host specification %q: %w", spec, err)
		}
	case strings.Count(hostPort, ":") > 1:
		if net.ParseIP(hostPort) == nil {
			return nil, fmt.Errorf("invalid jump host IPv6 address in %q", spec)
		}
		host = hostPort
	default:
		host = hostPort
	}

	if host == "" {
		return nil, fmt.Errorf("invalid jump host specification %q: missing host", spec)
	}

	port := 0
	if portStr != "" {
		n, err := strconv.Atoi(portStr)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid jump host port %q in %q", portStr, spec)
		}
		port = n
	}

	return &JumpHostConfig{
		Host:     host,
		Port:     port,
		Username: user,
	}, nil
}
