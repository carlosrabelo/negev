package platform

import (
	"fmt"
	"strings"

	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
	"github.com/carlosrabelo/negev/core/platform/dmos"
	"github.com/carlosrabelo/negev/core/platform/ios"
)

// SwitchDriver defines the behaviour required to support a switching platform.
type SwitchDriver interface {
	Name() string
	Detect(repo ports.SwitchRepository) (bool, error)

	// GetAuthenticationSequence returns the login sequence for this platform
	GetAuthenticationSequence(username, password, enablePassword string) []entities.AuthPrompt

	GetVLANList(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error)
	GetTrunkInterfaces(repo ports.SwitchRepository, cfg entities.SwitchConfig) (map[string]bool, error)
	GetActivePorts(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Port, error)
	GetMacTable(repo ports.SwitchRepository, cfg entities.SwitchConfig) ([]entities.Device, error)

	ConfigureAccessCommands(iface, vlan string) []string
	CreateVLANCommands(vlan string) []string
	DeleteVLANCommands(vlan string) []string
	SaveCommands() []string
}

var registry = []SwitchDriver{
	ios.New(),
	dmos.New(),
}

// Get returns a driver by normalized platform name.
func Get(name string) (SwitchDriver, error) {
	normalized := normalizeName(name)
	for _, driver := range registry {
		if driver.Name() == normalized {
			return driver, nil
		}
	}
	return nil, fmt.Errorf("unknown switch platform: %s", name)
}

// Available returns all registered drivers.
func Available() []SwitchDriver {
	out := make([]SwitchDriver, len(registry))
	copy(out, registry)
	return out
}

// Detect tries all registered drivers until one matches.
func Detect(repo ports.SwitchRepository) (SwitchDriver, error) {
	var lastErr error
	for _, driver := range registry {
		matched, err := driver.Detect(repo)
		if err != nil {
			lastErr = err
			continue
		}
		if matched {
			return driver, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("unable to detect switch platform")
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
