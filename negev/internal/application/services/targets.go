package services

import (
	"fmt"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
)

// TargetSelect chooses which switches to run from a loaded configuration.
type TargetSelect struct {
	Target string // comma-separated IPs and/or names
	All    bool
	Group  string
}

// ResolveTargets returns the switches selected by --target / --all / --group.
// Results are ordered: for --target, in the order requested; otherwise config order.
func ResolveTargets(cfg *config.Config, sel TargetSelect) ([]entities.SwitchConfig, error) {
	if cfg == nil || len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("no switches defined in the configuration")
	}

	modes := 0
	if sel.All {
		modes++
	}
	if strings.TrimSpace(sel.Group) != "" {
		modes++
	}
	if strings.TrimSpace(sel.Target) != "" {
		modes++
	}
	if modes == 0 {
		return nil, fmt.Errorf("one of --target, --all, or --group is required")
	}
	if modes > 1 {
		return nil, fmt.Errorf("--target, --all, and --group are mutually exclusive")
	}

	if sel.All {
		out := make([]entities.SwitchConfig, len(cfg.Switches))
		copy(out, cfg.Switches)
		return out, nil
	}

	if group := strings.TrimSpace(sel.Group); group != "" {
		var out []entities.SwitchConfig
		for _, sw := range cfg.Switches {
			if hasTag(sw.Tags, group) {
				out = append(out, sw)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no switches with tag %q", group)
		}
		return out, nil
	}

	parts := splitTargets(sel.Target)
	if len(parts) == 0 {
		return nil, fmt.Errorf("one of --target, --all, or --group is required")
	}

	seen := make(map[string]bool)
	var out []entities.SwitchConfig
	for _, ref := range parts {
		sw, err := findSwitch(cfg.Switches, ref)
		if err != nil {
			return nil, err
		}
		if seen[sw.Target] {
			continue
		}
		seen[sw.Target] = true
		out = append(out, sw)
	}
	return out, nil
}

func splitTargets(raw string) []string {
	var parts []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func findSwitch(switches []entities.SwitchConfig, ref string) (entities.SwitchConfig, error) {
	for _, sw := range switches {
		if sw.Target == ref || sw.Name == ref {
			return sw, nil
		}
	}
	return entities.SwitchConfig{}, fmt.Errorf("target %q not found in configuration", ref)
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}
