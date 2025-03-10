package services

import (
	"fmt"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	domainServices "github.com/carlosrabelo/negev/negev/internal/domain/services"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

func RunTarget(target, configPath string, write bool, verbose int, createVLANs bool) error {
	cfg := &config.Config{
		Switches: []entities.SwitchConfig{{
			Target: target, Platform: "ios", Transport: "telnet",
			DefaultVlan: "1", NoDataVlan: "999",
		}},
	}
	_ = configPath
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
		return fmt.Errorf("auto-detect not wired yet")
	}
	driver = platform.Get(platformID)
	if driver == nil {
		return fmt.Errorf("unknown platform %q (available: %v)", platformID, platform.Available())
	}
	svc := domainServices.NewVLANService(adapter, sw, driver)
	return svc.ProcessPorts()
}
