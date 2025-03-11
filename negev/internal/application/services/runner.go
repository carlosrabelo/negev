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

func RunTarget(target, configPath string, write bool, verbose int, createVLANs bool) error {
	if verbose < 0 || verbose > 3 {
		return fmt.Errorf("--verbose must be 0-3")
	}
	if target == "" {
		return fmt.Errorf("target is required")
	}

	var level slog.Level
	switch verbose {
	case 1, 3:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	defer transport.CloseAll()
	cfgPath := configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.FindPath("config.yaml", verbose)
		if err != nil {
			return err
		}
	}
	cfg, err := config.Load(cfgPath, target, verbose)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	var switchCfg *entities.SwitchConfig
	for i, sc := range cfg.Switches {
		if sc.Target == target {
			switchCfg = &cfg.Switches[i]
			break
		}
	}
	if switchCfg == nil {
		return fmt.Errorf("target %s not found in configuration", target)
	}
	sw := *switchCfg
	sw.Sandbox = !write
	sw.VerbosityLevel = verbose
	sw.CreateVLANs = createVLANs
	adapter := transport.NewSwitchAdapter(sw)
	var driver platform.SwitchDriver
	platformID := sw.PlatformID()
	if platformID == "auto" {
		if err := adapter.Connect(); err != nil {
			return fmt.Errorf("failed to connect for auto-detection: %v", err)
		}
		var err error
		driver, err = platform.Detect(adapter)
		if err != nil {
			return fmt.Errorf("platform detection failed: %v", err)
		}
		slog.Info("Detected platform", "platform", driver.Name(), "target", sw.Target)
	}
	driver = platform.Get(platformID)
	if driver == nil {
		return fmt.Errorf("unknown platform %q (available: %v)", platformID, platform.Available())
	}
	_ = driver.Name()
	if authCfg, ok := interface{}(adapter).(transport.AuthConfigurable); ok {
		authCfg.SetAuthSequence(driver.GetAuthenticationSequence())
	}
	driver.ClearCache()
	svc := domainServices.NewVLANService(adapter, sw, driver)
	return svc.ProcessPorts()
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

func (s *VLANApplicationService) Run(sandbox bool, verbosity int, createVLANs bool) error {
	return RunTarget(s.target, "", !sandbox, verbosity, createVLANs)
}
