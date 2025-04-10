package services

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	domainServices "github.com/carlosrabelo/negev/negev/internal/domain/services"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

// AskPassFunc prompts for switch and optional enable passwords.
// A nil function skips interactive prompting.
type AskPassFunc func() (password, enable string, err error)

// RunTarget loads configuration and runs VLAN automation for a switch.
// When askPass is non-nil, prompted credentials override YAML/env for the target switch.
func RunTarget(target, configPath string, write bool, verbose int, createVLANs bool, askPass AskPassFunc, backup BackupOpts) (entities.RunResult, error) {
	var result entities.RunResult
	if verbose < 0 || verbose > 3 {
		return result, fmt.Errorf("--verbose must be 0-3")
	}

	var level slog.Level
	switch verbose {
	case 1, 3:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfgPath := configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.FindPath("config.yaml", verbose)
		if err != nil {
			return result, err
		}
	}

	cfg, err := config.Load(cfgPath, target, verbose)
	if err != nil {
		return result, fmt.Errorf("failed to load config: %w", err)
	}

	if askPass != nil {
		password, enable, err := askPass()
		if err != nil {
			return result, err
		}
		applyAskPass(cfg, target, password, enable)
	}

	defer transport.CloseAll()

	return NewVLANApplicationService(cfg, target).Run(!write, verbose, createVLANs, backup)
}

func applyAskPass(cfg *config.Config, target, password, enable string) {
	for i := range cfg.Switches {
		if cfg.Switches[i].Target != target {
			continue
		}
		cfg.Switches[i].Password = password
		if enable != "" {
			cfg.Switches[i].EnablePassword = enable
		}
		return
	}
}

type VLANApplicationService struct {
	cfg        *config.Config
	target     string
	newAdapter func(entities.SwitchConfig) *transport.SwitchAdapter
}

func NewVLANApplicationService(cfg *config.Config, target string) *VLANApplicationService {
	return &VLANApplicationService{
		cfg:        cfg,
		target:     target,
		newAdapter: transport.NewSwitchAdapter,
	}
}

func (s *VLANApplicationService) Run(sandbox bool, verbosity int, createVLANs bool, backup BackupOpts) (entities.RunResult, error) {
	var result entities.RunResult

	var switchCfg *entities.SwitchConfig
	for i, sc := range s.cfg.Switches {
		if sc.Target == s.target {
			switchCfg = &s.cfg.Switches[i]
			break
		}
	}
	if switchCfg == nil {
		return result, fmt.Errorf("target %s not found in configuration", s.target)
	}

	switchCfg.Sandbox = sandbox
	switchCfg.VerbosityLevel = verbosity
	switchCfg.CreateVLANs = createVLANs

	adapter := s.newAdapter(*switchCfg)

	var driver platform.SwitchDriver
	platformID := switchCfg.PlatformID()
	if platformID == "auto" {
		// Leave the auth sequence empty so Telnet/SSH use a generic login probe
		// before show version. The detected driver sequence is applied below.
		if err := adapter.Connect(); err != nil {
			return result, fmt.Errorf("failed to connect for auto-detection: %v", err)
		}
		var err error
		driver, err = platform.Detect(adapter)
		if err != nil {
			return result, fmt.Errorf("platform detection failed: %v", err)
		}
		slog.Info("Detected platform", "platform", driver.Name(), "target", switchCfg.Target)
	} else {
		driver = platform.Get(platformID)
		if driver == nil {
			return result, fmt.Errorf("unknown platform %q (available: %v)", platformID, platform.Available())
		}
	}

	adapter.SetSessionProfile(driver.GetSessionProfile())

	if authCfg, ok := interface{}(adapter).(transport.AuthConfigurable); ok {
		authCfg.SetAuthSequence(driver.GetAuthenticationSequence())
	}

	if backup.Required(!sandbox) {
		dir := backup.ResolveDir(s.cfg.BackupDir)
		path, err := takePreWriteBackup(adapter, driver, *switchCfg, dir)
		if err != nil {
			return result, fmt.Errorf("backup failed — aborting before write: %w", err)
		}
		slog.Info("Configuration backup saved", "path", path, "target", switchCfg.Target)
	}

	driver.ClearCache()
	svc := domainServices.NewVLANService(adapter, *switchCfg, driver)
	return svc.ProcessPorts()
}
