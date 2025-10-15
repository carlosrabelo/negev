package ports

import "github.com/carlosrabelo/negev/core/domain/entities"

// VLANService defines the port for VLAN management services
type VLANService interface {
	ProcessPorts() error
	GetVlanList() (map[string]bool, error)
	GetTrunkInterfaces() (map[string]bool, error)
	GetActivePorts() ([]entities.Port, error)
	GetMacTable() ([]entities.Device, error)
	ConfigureVlan(iface, vlan string)
	CreateVLAN(vlan string) error
	DeleteVLAN(vlan string) error
}
