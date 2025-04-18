package services

import (
	"fmt"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

type VLANServiceImpl struct {
	repo   ports.SwitchRepository
	config entities.SwitchConfig
	driver platform.SwitchDriver
}

func NewVLANService(repo ports.SwitchRepository, config entities.SwitchConfig, driver platform.SwitchDriver) *VLANServiceImpl {
	return &VLANServiceImpl{repo: repo, config: config, driver: driver}
}

var _ ports.VLANService = (*VLANServiceImpl)(nil)

func (s *VLANServiceImpl) ProcessPorts() error {
	return fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) GetVlanList() (map[string]bool, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) GetTrunkInterfaces() (map[string]bool, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) GetActivePorts() ([]entities.Port, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) GetMacTable() ([]entities.Device, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) ConfigureVlan(iface, vlan string) error {
	return fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) CreateVLAN(vlan string) error {
	return fmt.Errorf("not implemented")
}

func (s *VLANServiceImpl) DeleteVLAN(vlan string) error {
	return fmt.Errorf("not implemented")
}
