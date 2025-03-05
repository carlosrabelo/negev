package ports

import "github.com/carlosrabelo/negev/negev/internal/domain/entities"

type VLANService interface {
	ProcessPorts() error
	GetVlanList() (map[string]bool, error)
	GetTrunkInterfaces() (map[string]bool, error)
	GetActivePorts() ([]entities.Port, error)
	GetMacTable() ([]entities.Device, error)
	ConfigureVlan(iface, vlan string) error
	CreateVLAN(vlan string) error
	DeleteVLAN(vlan string) error
}
