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
	audit  ports.Auditor

	l3Vlans        map[string]bool
	portVlansInUse map[string]bool
}

func NewVLANService(repo ports.SwitchRepository, config entities.SwitchConfig, driver platform.SwitchDriver) *VLANServiceImpl {
	return &VLANServiceImpl{repo: repo, config: config, driver: driver}
}

// SetAuditor enables JSONL audit logging for port VLAN changes.
func (s *VLANServiceImpl) SetAuditor(a ports.Auditor) {
	s.audit = a
}

var _ ports.VLANService = (*VLANServiceImpl)(nil)

func (s *VLANServiceImpl) ProcessPorts() (entities.RunResult, error) {
	var result entities.RunResult

	s.repo.SetSessionProfile(s.driver.GetSessionProfile())

	if err := s.repo.Connect(); err != nil {
		return result, fmt.Errorf("failed to connect: %v", err)
	}
	defer s.repo.Disconnect()

	vlans, err := s.GetVlanList()
	if err != nil {
		return result, fmt.Errorf("failed to get VLAN list: %v", err)
	}

	trunks, err := s.GetTrunkInterfaces()
	if err != nil {
		return result, fmt.Errorf("failed to get trunks: %v", err)
	}

	s.l3Vlans = make(map[string]bool)
	if l3List, err := s.driver.GetL3Interfaces(s.repo); err != nil {
		slog.Warn("Failed to discover L3 interfaces", "error", err, "target", s.config.Target)
	} else {
		for _, v := range l3List {
			s.l3Vlans[canonicalVLAN(v)] = true
		}
	}

	ports, err := s.GetActivePorts()
	if err != nil {
		return result, fmt.Errorf("failed to get active ports: %v", err)
	}

	devices, err := s.GetMacTable()
	if err != nil {
		return result, fmt.Errorf("failed to get MAC table: %v", err)
	}

	modified := false
	simulated := false

	var allowed map[string]bool
	if s.config.CreateVLANs {
		if len(s.config.AllowedVlans) == 0 {
			return result, fmt.Errorf("--create-vlans requires a non-empty allowed_vlans list")
		}
		allowed = s.getAllowedVLANs()
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
	}

	applied := make(map[string]string) // canonical iface → new vlan
	for _, port := range ports {
		if port.IsRouted || port.IsTrunk {
			continue
		}
		ifaceKey := s.canonicalIface(port.Interface)
		if trunks[ifaceKey] {
			continue
		}
		if s.isExcludedPort(port.Interface) {
			continue
		}

		targetVlan, ok := s.resolveTargetVlan(port, devices)
		if !ok {
			continue
		}

		mac := primaryMAC(s.filterActiveMacs(devices, port.Interface))
		changed, err := s.applyPortVlan(port, targetVlan, vlans)
		if err != nil {
			slog.Error("Port configuration failed — continuing", "port", port.Interface, "error", err, "target", s.config.Target)
			result.PortsFailed++
			result.Errors = append(result.Errors, err)
			s.auditPortChange(port, mac, targetVlan, "FAILED")
			continue
		}
		if changed {
			applied[ifaceKey] = targetVlan
			result.PortsChanged++
			if s.config.Sandbox {
				simulated = true
			} else {
				modified = true
			}
			s.auditPortChange(port, mac, targetVlan, "SUCCESS")
		}
	}

	s.portVlansInUse = s.effectivePortVlans(ports, trunks, applied)

	if s.config.CreateVLANs {
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

	if result.PortsChanged == 0 && !modified && !simulated && result.PortsFailed == 0 {
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

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("completed with %d port failure(s): %v", result.PortsFailed, result.Errors)
	}
	return result, nil
}

func (s *VLANServiceImpl) resolveTargetVlan(port entities.Port, devices []entities.Device) (string, bool) {
	macs := s.filterActiveMacs(devices, port.Interface)
	if len(macs) == 0 {
		if s.config.NoDataVlan == "" {
			return "", false
		}
		return s.config.NoDataVlan, true
	}
	if len(macs) > 1 {
		return s.resolveMultiMacVlan(port, macs)
	}
	mac := macs[0]
	if len(mac.Mac) < 2 {
		return "", false
	}
	targetVlan := resolveMacToVlan(mac.Mac, s.config.MacToVlan, s.config.DefaultVlan)
	if targetVlan == "" {
		return "", false
	}
	return targetVlan, true
}

func (s *VLANServiceImpl) filterActiveMacs(devices []entities.Device, iface string) []entities.Device {
	var result []entities.Device
	for _, d := range s.filterDevices(devices, iface) {
		if s.isExcluded(d.Mac) {
			continue
		}
		result = append(result, d)
	}
	return result
}

func (s *VLANServiceImpl) resolveMultiMacVlan(port entities.Port, macs []entities.Device) (string, bool) {
	policy := s.config.MultiMacPolicy
	if policy == "" {
		policy = entities.MultiMacIgnore
	}

	switch policy {
	case entities.MultiMacAlert:
		parts := make([]string, 0, len(macs))
		for _, mac := range macs {
			vlan := resolveMacToVlan(mac.Mac, s.config.MacToVlan, s.config.DefaultVlan)
			if vlan == "" {
				vlan = "?"
			}
			parts = append(parts, fmt.Sprintf("%s (VLAN %s)", mac.Mac, vlan))
		}
		slog.Warn(fmt.Sprintf("Port %s has multiple MACs: %s; skipped", port.Interface, strings.Join(parts, ", ")),
			"port", port.Interface, "target", s.config.Target)
		return "", false

	case entities.MultiMacPriority:
		bestVlan := ""
		bestLen := -1
		for _, mac := range macs {
			if len(mac.Mac) < 2 {
				continue
			}
			vlan, prefixLen, explicit := matchMacToVlan(mac.Mac, s.config.MacToVlan, s.config.DefaultVlan)
			if !explicit || vlan == "" {
				continue
			}
			if prefixLen > bestLen {
				bestLen = prefixLen
				bestVlan = vlan
			}
		}
		if bestVlan == "" {
			return "", false
		}
		return bestVlan, true

	default: // ignore
		return "", false
	}
}

// resolveMacToVlan picks the longest matching even-length prefix (12..2) in mapping.
func resolveMacToVlan(mac string, mapping map[string]string, defaultVlan string) string {
	vlan, _, _ := matchMacToVlan(mac, mapping, defaultVlan)
	return vlan
}

// matchMacToVlan returns the resolved VLAN, matched prefix length, and whether the
// VLAN came from an explicit mac_to_vlan entry (vs default_vlan fallback).
func matchMacToVlan(mac string, mapping map[string]string, defaultVlan string) (vlan string, prefixLen int, explicit bool) {
	for length := 12; length >= 2; length -= 2 {
		if len(mac) < length {
			continue
		}
		v := mapping[mac[:length]]
		if v != "" && v != "0" && v != "00" {
			return v, length, true
		}
	}
	return defaultVlan, 0, false
}

func (s *VLANServiceImpl) effectivePortVlans(ports []entities.Port, trunks map[string]bool, applied map[string]string) map[string]bool {
	inUse := make(map[string]bool)
	for _, port := range ports {
		if port.IsRouted {
			continue
		}
		ifaceKey := s.canonicalIface(port.Interface)
		if port.IsTrunk || trunks[ifaceKey] {
			continue
		}
		vlan := port.Vlan
		if next, ok := applied[ifaceKey]; ok {
			vlan = next
		}
		if vlan == "" || strings.EqualFold(vlan, "trunk") || strings.EqualFold(vlan, "routed") {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(vlan)); err != nil {
			continue
		}
		inUse[canonicalVLAN(vlan)] = true
	}
	return inUse
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
		result[s.canonicalIface(t)] = true
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

func (s *VLANServiceImpl) auditPortChange(port entities.Port, mac, newVlan, status string) {
	if s.audit == nil {
		return
	}
	entry := ports.PortVLANAudit{
		Target:     s.config.Target,
		SwitchName: s.config.Name,
		Port:       port.Interface,
		MAC:        mac,
		OldVLAN:    vlanInt(port.Vlan),
		NewVLAN:    vlanInt(newVlan),
		Action:     "SET_PORT_VLAN",
		Sandbox:    s.config.Sandbox,
		Status:     status,
	}
	if err := s.audit.AppendPortVLAN(entry); err != nil {
		slog.Warn("Failed to append audit log entry", "error", err, "port", port.Interface, "target", s.config.Target)
	}
}

func primaryMAC(macs []entities.Device) string {
	if len(macs) == 0 {
		return ""
	}
	return macs[0].Mac
}

func vlanInt(vlan string) int {
	n, err := strconv.Atoi(strings.TrimSpace(vlan))
	if err != nil {
		return 0
	}
	return n
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

func (s *VLANServiceImpl) canonicalIface(name string) string {
	return strings.ToLower(s.driver.NormalizeInterface(name))
}

func (s *VLANServiceImpl) filterDevices(devices []entities.Device, iface string) []entities.Device {
	key := s.canonicalIface(iface)
	var result []entities.Device
	for _, d := range devices {
		if s.canonicalIface(d.Interface) == key {
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
	key := s.canonicalIface(iface)
	for _, e := range s.config.ExcludePorts {
		if s.canonicalIface(e) == key {
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
	canon := canonicalVLAN(vlan)
	if canon == "1" {
		return true
	}
	n, _ := strconv.Atoi(strings.TrimSpace(vlan))
	if n >= 1000 && n <= 4094 {
		return true
	}
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
	if s.l3Vlans[canon] {
		return true
	}
	if s.portVlansInUse[canon] {
		return true
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
