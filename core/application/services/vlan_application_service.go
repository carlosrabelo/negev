package services

import (
	"github.com/carlosrabelo/negev/core/domain/entities"
	"github.com/carlosrabelo/negev/core/domain/ports"
	"github.com/carlosrabelo/negev/core/domain/services"
	"github.com/carlosrabelo/negev/core/infrastructure/transport"
)

// VLANApplicationService orchestrates the use of VLAN services
type VLANApplicationService struct {
	vlanService ports.VLANService
}

// NewVLANApplicationService creates a new instance of the VLAN application service
func NewVLANApplicationService(switchConfig entities.SwitchConfig, transportClient transport.Client) *VLANApplicationService {
	switchAdapter := transport.NewSwitchAdapter(transportClient)
	vlanService := services.NewVLANService(switchAdapter, switchConfig)

	return &VLANApplicationService{
		vlanService: vlanService,
	}
}

// ProcessPorts processes the switch ports
func (v *VLANApplicationService) ProcessPorts() error {
	return v.vlanService.ProcessPorts()
}
