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
	sw := cfg.Switches[0]
	sw.Sandbox = !write
	sw.VerbosityLevel = verbose
	sw.CreateVLANs = createVLANs
	adapter := transport.NewSwitchAdapter(sw)
	driver := platform.Get("ios")
	if driver == nil {
		return fmt.Errorf("ios driver not registered")
	}
	svc := domainServices.NewVLANService(adapter, sw, driver)
	return svc.ProcessPorts()
}
