package services

import (
	"fmt"
	"log"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	domainServices "github.com/carlosrabelo/negev/negev/internal/domain/services"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

type VLANApplicationService struct {
	cfg    *config.Config
	target string
}

func NewVLANApplicationService(cfg *config.Config, target string) *VLANApplicationService {
	return &VLANApplicationService{cfg: cfg, target: target}
}

func (s *VLANApplicationService) Run(sandbox bool, verbosity int, createVLANs bool) error {
	var switchCfg *entities.SwitchConfig
	for i, sc := range s.cfg.Switches {
		if sc.Target == s.target {
			switchCfg = &s.cfg.Switches[i]
			break
		}
	}
	if switchCfg == nil {
		return fmt.Errorf("target %s not found in configuration", s.target)
	}

	switchCfg.Sandbox = sandbox
	switchCfg.VerbosityLevel = verbosity
	switchCfg.CreateVLANs = createVLANs

	adapter := transport.NewSwitchAdapter(*switchCfg)

	var driver platform.SwitchDriver
	platformID := switchCfg.PlatformID()
	if platformID == "auto" {
		if err := adapter.Connect(); err != nil {
			return fmt.Errorf("failed to connect for auto-detection: %v", err)
		}
		var err error
		driver, err = platform.Detect(adapter)
		if err != nil {
			return fmt.Errorf("platform detection failed: %v", err)
		}
		log.Printf("Detected platform: %s", driver.Name())
	} else {
		driver = platform.Get(platformID)
		if driver == nil {
			return fmt.Errorf("unknown platform %q (available: %v)", platformID, platform.Available())
		}
	}

	if authCfg, ok := interface{}(adapter).(transport.AuthConfigurable); ok {
		authCfg.SetAuthSequence(driver.GetAuthenticationSequence())
	}

	driver.ClearCache()
	svc := domainServices.NewVLANService(adapter, *switchCfg, driver)
	return svc.ProcessPorts()
}
