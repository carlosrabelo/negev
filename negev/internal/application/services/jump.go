package services

import (
	"fmt"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

// JumpAskPassFunc prompts interactively for the jump host SSH password.
type JumpAskPassFunc func() (string, error)

// JumpOpts holds CLI overrides for jump host tunneling.
type JumpOpts struct {
	Spec       string
	KeyFile    string
	KnownHosts string
	AskPass    JumpAskPassFunc
}

// ApplyJumpOpts merges CLI jump host overrides into the selected switches.
func ApplyJumpOpts(switches []entities.SwitchConfig, opts JumpOpts) ([]entities.SwitchConfig, error) {
	out := make([]entities.SwitchConfig, len(switches))
	for i := range switches {
		out[i] = switches[i]
		out[i].JumpHost = switches[i].JumpHost.Clone()
	}

	spec := strings.TrimSpace(opts.Spec)
	keyFile := strings.TrimSpace(opts.KeyFile)
	knownHosts := strings.TrimSpace(opts.KnownHosts)

	if spec != "" {
		parsed, err := entities.ParseJumpSpec(spec)
		if err != nil {
			return nil, err
		}
		if parsed.Disabled {
			if keyFile != "" {
				return nil, fmt.Errorf("--jump-key cannot be combined with --jump %s", opts.Spec)
			}
			if knownHosts != "" {
				return nil, fmt.Errorf("--jump-known-hosts cannot be combined with --jump %s", opts.Spec)
			}
			if opts.AskPass != nil {
				return nil, fmt.Errorf("--jump-ask-pass cannot be combined with --jump %s", opts.Spec)
			}
			for i := range out {
				out[i].JumpHost = nil
			}
			return out, nil
		}

		if keyFile != "" {
			parsed.KeyFile = keyFile
		}
		if knownHosts != "" {
			parsed.KnownHosts = knownHosts
		}
		for i := range out {
			out[i].JumpHost = parsed.Clone()
		}
	} else {
		hasJump := false
		for i := range out {
			if out[i].JumpHost.IsEnabled() {
				hasJump = true
				break
			}
		}
		if !hasJump {
			if keyFile != "" {
				return nil, fmt.Errorf("--jump-key requires a jump host (--jump or YAML jump_host)")
			}
			if knownHosts != "" {
				return nil, fmt.Errorf("--jump-known-hosts requires a jump host (--jump or YAML jump_host)")
			}
			if opts.AskPass != nil {
				return nil, fmt.Errorf("--jump-ask-pass requires a jump host (--jump or YAML jump_host)")
			}
			return out, nil
		}

		for i := range out {
			if !out[i].JumpHost.IsEnabled() {
				continue
			}
			if keyFile != "" {
				out[i].JumpHost.KeyFile = keyFile
			}
			if knownHosts != "" {
				out[i].JumpHost.KnownHosts = knownHosts
			}
		}
	}

	if opts.AskPass != nil {
		pw, err := opts.AskPass()
		if err != nil {
			return nil, err
		}
		for i := range out {
			if out[i].JumpHost.IsEnabled() {
				out[i].JumpHost.Password = pw
			}
		}
	}

	return out, nil
}
