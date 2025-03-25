package services

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
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

func (s *VLANServiceImpl) ProcessPorts() (entities.RunResult, error) {
	var result entities.RunResult

	if err := s.repo.Connect(); err != nil {
		return result, fmt.Errorf("failed to connect: %v", err)
	}
	defer s.repo.Disconnect()

	vlans, err := s.GetVlanList()
	if err != nil {
		return result, fmt.Errorf("failed to get VLAN list: %v", err)
	}

	modified := false
	simulated := false

	if s.config.CreateVLANs {
		if len(s.config.AllowedVlans) == 0 {
			return result, fmt.Errorf("--create-vlans requires a non-empty allowed_vlans list")
		}
		allowed := s.getAllowedVLANs()
		if s.config.DefaultVlan != "" {
			allowed[s.config.DefaultVlan] = true
		}
		if s.config.NoDataVlan != "" {
			allowed[s.config.NoDataVlan] = true
		}
		for _, v := range sortedKeys(allowed) {
			if !vlans[v] {
				if err := s.CreateVLAN(v); err != nil {
					return result, fmt.Errorf("failed to create VLAN %s: %v", v, err)
				}
				vlans[v] = true
				result.VLANsCreated++
				if s.config.Sandbox {
					simulated = true
				} else {
					modified = true
				}
			}
		}
		for _, v := range sortedKeys(vlans) {
			if s.isProtected(v) || allowed[v] {
				continue
			}
			if err := s.DeleteVLAN(v); err != nil {
				return result, fmt.Errorf("failed to delete VLAN %s: %v", v, err)
			}
			delete(vlans, v)
			result.VLANsDeleted++
			if s.config.Sandbox {
				simulated = true
			} else {
				modified = true
			}
		}
	}

	trunks, err := s.GetTrunkInterfaces()
	if err != nil {
		return result, fmt.Errorf("failed to get trunks: %v", err)
	}

	ports, err := s.GetActivePorts()
	if err != nil {
		return result, fmt.Errorf("failed to get active ports: %v", err)
	}

	devices, err := s.GetMacTable()
	if err != nil {
		return result, fmt.Errorf("failed to get MAC table: %v", err)
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
			if s.config.NoDataVlan == "" {
				continue
			}
			changed, err := s.applyPortVlan(port, s.config.NoDataVlan, vlans)
			if err != nil {
				return result, err
			}
			if changed {
				changes++
				result.PortsChanged++
				if s.config.Sandbox {
					simulated = true
				} else {
					modified = true
				}
			}
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

		changed, err := s.applyPortVlan(port, targetVlan, vlans)
		if err != nil {
			return result, err
		}
		if changed {
			changes++
			result.PortsChanged++
			if s.config.Sandbox {
				simulated = true
			} else {
				modified = true
			}
		}
	}

	if changes == 0 && !modified && !simulated {
		fmt.Println("No changes required")
	} else if s.config.Sandbox {
		fmt.Printf("Changes simulated (sandbox mode, use --write to apply)\n")
	} else if modified {
		slog.Info("Saving changes to startup-config", "target", s.config.Target)
		if err := s.saveConfiguration(); err != nil {
			return result, fmt.Errorf("failed to save configuration: %v", err)
		}
		slog.Info("Configuration successfully saved", "target", s.config.Target)
	}

	return result, nil
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

func (s *VLANServiceImpl) applyPortVlan(port entities.Port, targetVlan string, vlans map[string]bool) (bool, error) {
	if !vlans[targetVlan] && !vlans[canonicalVLAN(targetVlan)] {
		slog.Error("Target VLAN does not exist on switch — skipping port", "vlan", targetVlan, "port", port.Interface, "target", s.config.Target)
		return false, nil
	}
	if targetVlan == port.Vlan || canonicalVLAN(targetVlan) == canonicalVLAN(port.Vlan) {
		return false, nil
	}
	if err := s.ConfigureVlan(port.Interface, targetVlan); err != nil {
		return false, fmt.Errorf("failed to configure VLAN on port %s: %v", port.Interface, err)
	}
	return true, nil
}

func (s *VLANServiceImpl) saveConfiguration() error {
	cmds := s.driver.SaveCommands()
	var lastErr error
	for _, cmd := range cmds {
		out, err := s.repo.ExecuteCommand(cmd)
		if err != nil {
			lastErr = fmt.Errorf("command %q failed: %v", cmd, err)
			continue
		}
		if s.driver.IsCommandError(out) {
			lastErr = fmt.Errorf("command %q returned error: %s", cmd, out)
			continue
		}
		return nil
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

func canonicalVLAN(vlan string) string {
	n, err := strconv.Atoi(strings.TrimSpace(vlan))
	if err != nil {
		return vlan
	}
	return strconv.Itoa(n)
}

func (s *VLANServiceImpl) isProtected(vlan string) bool {
	n, _ := strconv.Atoi(strings.TrimSpace(vlan))
	if n >= 1000 && n <= 4094 {
		return true
	}
	canon := canonicalVLAN(vlan)
	if canon == s.config.DefaultVlan {
		return true
	}
	if s.config.NoDataVlan != "" && canon == s.config.NoDataVlan {
		return true
	}
	for _, p := range s.config.ProtectedVlans {
		if vlan == p || canon == canonicalVLAN(p) {
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
