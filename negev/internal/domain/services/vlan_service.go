package services

import (
	"fmt"
	"strconv"
	"strings"
	"log/slog"

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
	if err := s.repo.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer s.repo.Disconnect()

	vlans, err := s.GetVlanList()
	if err != nil {
		return fmt.Errorf("failed to get VLAN list: %v", err)
	}
	if s.config.CreateVLANs {
		allowed := s.getAllowedVLANs()
		for v := range allowed {
			if !vlans[v] {
				if err := s.CreateVLAN(v); err != nil {
					return fmt.Errorf("failed to create VLAN %s: %v", v, err)
				}
				vlans[v] = true
			}
		}
		for v := range vlans {
			if s.isProtected(v) || allowed[v] {
				continue
			}
			if err := s.DeleteVLAN(v); err != nil {
				return fmt.Errorf("failed to delete VLAN %s: %v", v, err)
			}
			delete(vlans, v)
		}
	}
	trunks, err := s.GetTrunkInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get trunks: %v", err)
	}
	_ = trunks
	ports, err := s.GetActivePorts()
	if err != nil {
		return fmt.Errorf("failed to get active ports: %v", err)
	}
	devices, err := s.GetMacTable()
	if err != nil {
		return fmt.Errorf("failed to get MAC table: %v", err)
	}
	for _, port := range ports {
		if trunks[port.Interface] {
			continue
		}
		if s.isExcludedPort(port.Interface) {
			continue
		}
		macs := s.filterDevices(devices, port.Interface)
		if len(macs) == 0 {
			continue
		}
		if len(macs) > 1 {
			slog.Warn("Multiple MACs on port — skipping for safety", "port", port.Interface, "target", s.config.Target)
			continue
		}
		mac := macs[0]
		if len(mac.Mac) < 6 {
			continue
		}
		if s.isExcluded(mac.Mac) {
			continue
		}
		prefix := mac.Mac[:6]
		targetVlan := s.config.MacToVlan[prefix]
		if targetVlan == "" || targetVlan == "0" || targetVlan == "00" {
			targetVlan = s.config.DefaultVlan
		}
		if !vlans[targetVlan] {
			slog.Error("Target VLAN does not exist on switch — skipping port", "vlan", targetVlan, "port", port.Interface, "target", s.config.Target)
			continue
		}
		if targetVlan != "" && targetVlan != port.Vlan {
			if err := s.ConfigureVlan(port.Interface, targetVlan); err != nil {
				return err
			}
		}
		_ = mac
	}
	return nil
}

func (s *VLANServiceImpl) filterDevices(devices []entities.Device, iface string) []entities.Device {
	var result []entities.Device
	for _, d := range devices {
		if d.Interface == iface {
			result = append(result, d)
		}
	}
	return result
}

func (s *VLANServiceImpl) GetVlanList() (map[string]bool, error) {
	vlans, err := s.driver.GetVLANList(s.repo)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(vlans))
	for _, v := range vlans {
		result[v] = true
	}
	return result, nil
}

func (s *VLANServiceImpl) GetTrunkInterfaces() (map[string]bool, error) {
	trunks, err := s.driver.GetTrunkInterfaces(s.repo)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		result[t] = true
	}
	return result, nil
}

func (s *VLANServiceImpl) GetActivePorts() ([]entities.Port, error) {
	return s.driver.GetActivePorts(s.repo)
}

func (s *VLANServiceImpl) GetMacTable() ([]entities.Device, error) {
	return s.driver.GetMacTable(s.repo)
}

func (s *VLANServiceImpl) ConfigureVlan(iface, vlan string) error {
	cmds := s.driver.ConfigureAccessCommands(entities.Port{Interface: iface}, vlan)
	if s.config.Sandbox {
		for _, cmd := range cmds {
			fmt.Printf("SIMULATE: %s\n", cmd)
		}
		return nil
	}
	for _, cmd := range cmds {
		out, err := s.repo.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("command %q failed: %v", cmd, err)
		}
		if s.driver.IsCommandError(out) {
			return fmt.Errorf("command %q returned error: %s", cmd, out)
		}
	}
	return nil
}

func (s *VLANServiceImpl) CreateVLAN(vlan string) error { return nil }

func (s *VLANServiceImpl) DeleteVLAN(vlan string) error { return nil }

func (s *VLANServiceImpl) getAllowedVLANs() map[string]bool {
	result := make(map[string]bool, len(s.config.AllowedVlans))
	for _, v := range s.config.AllowedVlans {
		result[v] = true
	}
	return result
}

func (s *VLANServiceImpl) isExcluded(mac string) bool {
	for _, e := range s.config.ExcludeMacs {
		if strings.EqualFold(mac, e) {
			return true
		}
	}
	return false
}

func (s *VLANServiceImpl) isExcludedPort(iface string) bool {
	for _, e := range s.config.ExcludePorts {
		if strings.EqualFold(iface, e) {
			return true
		}
	}
	return false
}

func (s *VLANServiceImpl) isProtected(vlan string) bool {
	n, _ := strconv.Atoi(strings.TrimSpace(vlan))
	if n >= 1000 && n <= 4094 {
		return true
	}
	for _, p := range s.config.ProtectedVlans {
		if vlan == p {
			return true
		}
	}
	return false
}
