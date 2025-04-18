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

func RunTarget(cfg *config.Config, target string, sandbox bool, verbosity int, createVLANs bool) error {
	var switchCfg *entities.SwitchConfig
	for i, s := range cfg.Switches {
		if s.Target == target {
			switchCfg = &cfg.Switches[i]
			break
		}
	}
	if switchCfg == nil {
		return fmt.Errorf("target %s not found in configuration", target)
	}

	switchCfg.Sandbox = sandbox
	switchCfg.VerbosityLevel = verbosity
	switchCfg.CreateVLANs = createVLANs

	var driver platform.SwitchDriver
	platformID := switchCfg.PlatformID()
	if platformID == "auto" {
		adapter := transport.NewSwitchAdapter(*switchCfg)
		if err := adapter.Connect(); err != nil {
			return fmt.Errorf("failed to connect for auto-detection: %v", err)
		}
		defer adapter.Disconnect()

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

	adapter := transport.NewSwitchAdapter(*switchCfg)
	if authCfg, ok := interface{}(adapter).(transport.AuthConfigurable); ok {
		authCfg.SetAuthSequence(driver.GetAuthenticationSequence())
	}

	svc := domainServices.NewVLANService(adapter, *switchCfg, driver)
	return svc.ProcessPorts()
}
