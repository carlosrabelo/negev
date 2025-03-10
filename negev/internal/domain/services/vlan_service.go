package services

import (
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

type VLANServiceImpl struct {
	repo   ports.SwitchRepository
	config entities.SwitchConfig
	driver platform.SwitchDriver
}

var _ ports.VLANService = (*VLANServiceImpl)(nil)

func (s *VLANServiceImpl) ProcessPorts() error { return nil }

func (s *VLANServiceImpl) GetVlanList() (map[string]bool, error) { return nil, nil }

func (s *VLANServiceImpl) GetTrunkInterfaces() (map[string]bool, error) { return nil, nil }

func (s *VLANServiceImpl) GetActivePorts() ([]entities.Port, error) { return nil, nil }

func (s *VLANServiceImpl) GetMacTable() ([]entities.Device, error) { return nil, nil }

func (s *VLANServiceImpl) ConfigureVlan(iface, vlan string) error { return nil }

func (s *VLANServiceImpl) CreateVLAN(vlan string) error { return nil }

func (s *VLANServiceImpl) DeleteVLAN(vlan string) error { return nil }
