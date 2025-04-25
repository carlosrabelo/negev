package services

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

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

	modified := false

	if s.config.CreateVLANs {
		allowed := s.getAllowedVLANs()
		for v := range allowed {
			if !vlans[v] {
				if err := s.CreateVLAN(v); err != nil {
					return fmt.Errorf("failed to create VLAN %s: %v", v, err)
				}
				vlans[v] = true
				if !s.config.Sandbox {
					modified = true
				}
			}
		}
		for v := range vlans {
			if s.isProtected(v) {
				continue
			}
			if !allowed[v] {
				if err := s.DeleteVLAN(v); err != nil {
					return fmt.Errorf("failed to delete VLAN %s: %v", v, err)
				}
				delete(vlans, v)
				if !s.config.Sandbox {
					modified = true
				}
			}
		}
	}

	trunks, err := s.GetTrunkInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get trunks: %v", err)
	}

	ports, err := s.GetActivePorts()
	if err != nil {
		return fmt.Errorf("failed to get active ports: %v", err)
	}

	devices, err := s.GetMacTable()
	if err != nil {
		return fmt.Errorf("failed to get MAC table: %v", err)
	}

	changes := 0
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

		if targetVlan == port.Vlan {
			continue
		}

		if err := s.ConfigureVlan(port.Interface, targetVlan); err != nil {
			return fmt.Errorf("failed to configure VLAN on port %s: %v", port.Interface, err)
		}
		changes++
		if !s.config.Sandbox {
			modified = true
		}
	}

	if changes == 0 && !modified {
		fmt.Println("No changes required")
	} else if s.config.Sandbox {
		fmt.Printf("Changes simulated (sandbox mode, use -w to apply)\n")
	} else if modified {
		slog.Info("Saving changes to startup-config", "target", s.config.Target)
		if err := s.saveConfiguration(); err != nil {
			return fmt.Errorf("failed to save configuration: %v", err)
		}
		slog.Info("Configuration successfully saved", "target", s.config.Target)
	}

	return nil
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

func (s *VLANServiceImpl) CreateVLAN(vlan string) error {
	cmds := s.driver.CreateVLANCommands(vlan)
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

func (s *VLANServiceImpl) DeleteVLAN(vlan string) error {
	cmds := s.driver.DeleteVLANCommands(vlan)
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

func (s *VLANServiceImpl) saveConfiguration() error {
	cmds := s.driver.SaveCommands()
	var lastErr error
	for _, cmd := range cmds {
		if _, err := s.repo.ExecuteCommand(cmd); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *VLANServiceImpl) getAllowedVLANs() map[string]bool {
	result := make(map[string]bool, len(s.config.AllowedVlans))
	for _, v := range s.config.AllowedVlans {
		result[v] = true
	}
	return result
}

func (s *VLANServiceImpl) filterDevices(devices []entities.Device, iface string) []entities.Device {
	var result []entities.Device
	for _, d := range devices {
		if strings.EqualFold(d.Interface, iface) {
			result = append(result, d)
		}
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
	vlanNum := 0
	fmt.Sscanf(vlan, "%d", &vlanNum)
	if vlanNum >= 1000 && vlanNum <= 4094 {
		return true
	}
	for _, p := range s.config.ProtectedVlans {
		if vlan == p {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
