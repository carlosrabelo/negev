package dm3000

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
	"github.com/carlosrabelo/negev/negev/internal/platform"
	"github.com/carlosrabelo/negev/negev/internal/platform/parseutil"
)

var (
	vlanColonLineRegex  = regexp.MustCompile(`(?i)^\s*vlan\s*:\s*(\d{1,4})\b`)
	vlanLegacyLineRegex = regexp.MustCompile(`(?i)^\s*(?:vlan\s+)?(\d{1,4})\b`)
	infoPortRegex       = regexp.MustCompile(`(?i)^Information\s+of\s+Eth(?:ernet)?\s*(\d+\s*/\s*\d+)`)
	infoSwitchportRe    = regexp.MustCompile(`(?i)^Information\s+of\s+(Eth(?:ernet)?\s*\d+\s*/\s*\d+|(?:Port-channel|Po)\s*\d+)`)
	ifaceSwitchportRe   = regexp.MustCompile(`(?i)^interface\s+(eth(?:ernet)?\s*\d+\s*/\s*\d+|(?:port-channel|po)\s*\d+)`)
	allowedTaggedRe     = regexp.MustCompile(`(?i)\(\s*(?:[sd]\s*,\s*)?t\s*\)`)
	macLineRegex        = regexp.MustCompile(`(?i)^\s*\d+\s+(?:\S+\s+)?(Eth(?:ernet)?\s*\d+\s*/\s*\d+)\s+([0-9a-f]{2}(?:[:-][0-9a-f]{2}){5}|[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\s+(\d{1,4})\s+(?:(?:-|\d+)\s+)?(Static|Dynamic|Learned)\b`)
	ethCompactRe        = regexp.MustCompile(`^eth(?:ernet)?(\d+/\d+)$`)
	poCompactRe         = regexp.MustCompile(`^(?:port-channel|po)(\d+)$`)
	sviLineRegex        = regexp.MustCompile(`(?i)^vlan(?:-interface)?\s*(\d{1,4})\b`)
	dm3000PromptPattern = regexp.MustCompile(`(?m)^[A-Za-z0-9_.-]+(?:-[A-Za-z0-9_.-]+)*(?:\([^)]*\))?[>#]\s*$`)
	dm3000PagerPattern  = regexp.MustCompile(`(?m)--More--|---- More ----`)
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dm3000"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		return false, err
	}
	if match, reject := dm3000DetectMatch(out); reject {
		return false, nil
	} else if match {
		return true, nil
	}
	if !d.IsCommandError(out) {
		return false, nil
	}
	// Older 3324F2 images reject "show version"; "show system" carries Model: DmSwitch...
	out, err = repo.ExecuteCommand("show system")
	if err != nil {
		return false, err
	}
	match, reject := dm3000DetectMatch(out)
	if reject {
		return false, nil
	}
	return match, nil
}

func dm3000DetectMatch(out string) (match bool, reject bool) {
	lower := strings.ToLower(out)
	if strings.Contains(lower, "dm1200") || strings.Contains(lower, "1200e") {
		return false, true
	}
	if strings.Contains(lower, "dmswitch") || strings.Contains(lower, "dm3000") {
		return true, false
	}
	if strings.Contains(lower, "datacom") && (strings.Contains(lower, "3324") || strings.Contains(lower, "3224")) {
		return true, false
	}
	return false, false
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "no terminal paging\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func normalizeInterface(name string) string {
	compact := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	lower := strings.ToLower(compact)
	if m := ethCompactRe.FindStringSubmatch(lower); m != nil {
		return "ethernet" + m[1]
	}
	if m := poCompactRe.FindStringSubmatch(lower); m != nil {
		return "port-channel" + m[1]
	}
	return compact
}

func isEthernetInterface(name string) bool {
	return strings.HasPrefix(name, "ethernet") && ethCompactRe.MatchString(name)
}

func isSwitchInterface(name string) bool {
	return ethCompactRe.MatchString(name) || poCompactRe.MatchString(name)
}

func compareInterfaceNames(a, b string) bool {
	rank := func(s string) int {
		if strings.HasPrefix(s, "ethernet") {
			return 0
		}
		if strings.HasPrefix(s, "port-channel") {
			return 1
		}
		return 2
	}
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra < rb
	}
	path := func(s string) (int, int) {
		s = strings.TrimPrefix(s, "ethernet")
		s = strings.TrimPrefix(s, "port-channel")
		parts := strings.Split(s, "/")
		if len(parts) == 1 {
			n, _ := strconv.Atoi(parts[0])
			return n, 0
		}
		if len(parts) < 2 {
			return 0, 0
		}
		u, _ := strconv.Atoi(parts[0])
		p, _ := strconv.Atoi(parts[1])
		return u, p
	}
	u1, p1 := path(a)
	u2, p2 := path(b)
	if u1 != u2 {
		return u1 < u2
	}
	return p1 < p2
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan")
	if err != nil || strings.TrimSpace(out) == "" || d.IsCommandError(out) {
		out, err = repo.ExecuteCommand("show vlan brief")
		if err != nil {
			return nil, err
		}
	}
	return parseVLANs(out), nil
}

func parseVLANs(output string) []string {
	lines := strings.Split(output, "\n")
	seen := make(map[string]bool)
	var vlans []string
	add := func(id string) {
		if !isAccessVLAN(id) || seen[id] {
			return
		}
		seen[id] = true
		vlans = append(vlans, id)
	}

	// Real DmSwitch 3000 "show vlan" uses "VLAN: 10" blocks mixed with numeric settings
	// such as "Aging-time: 300 sec." and "STP: on instance(s) 1".
	hasColonBlocks := false
	for _, line := range lines {
		if m := vlanColonLineRegex.FindStringSubmatch(line); m != nil {
			hasColonBlocks = true
			add(m[1])
		}
	}
	if hasColonBlocks {
		return vlans
	}

	for _, line := range lines {
		if parseutil.IsSeparatorLine(line) {
			continue
		}
		m := vlanLegacyLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		add(m[1])
	}
	return vlans
}

func parseSwitchportHeader(trimmed string) (string, bool) {
	if m := infoSwitchportRe.FindStringSubmatch(strings.TrimSuffix(trimmed, ":")); m != nil {
		return normalizeInterface(m[1]), true
	}
	if m := ifaceSwitchportRe.FindStringSubmatch(trimmed); m != nil {
		return normalizeInterface(m[1]), true
	}
	return "", false
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := getSwitchportOutput(repo)
	if err != nil {
		return nil, err
	}
	return parseTrunkInterfaces(out), nil
}

func parseTrunkInterfaces(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	add := func(name string) {
		if name == "" || !isSwitchInterface(name) || seen[name] {
			return
		}
		seen[name] = true
		trunks = append(trunks, name)
	}

	var currentIface string
	inAllowedVLANs := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inAllowedVLANs = false
			continue
		}
		if iface, ok := parseSwitchportHeader(trimmed); ok {
			currentIface = iface
			inAllowedVLANs = false
			continue
		}
		if currentIface == "" {
			continue
		}

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "allowed vlans:") {
			inAllowedVLANs = true
			if allowedTaggedRe.MatchString(trimmed) {
				add(currentIface)
			}
			continue
		}
		if inAllowedVLANs {
			if strings.Contains(trimmed, ":") {
				inAllowedVLANs = false
			} else {
				if allowedTaggedRe.MatchString(trimmed) {
					add(currentIface)
				}
				continue
			}
		}
		if strings.Contains(lower, "trunk") && (strings.Contains(lower, "mode") || strings.Contains(lower, "trunking")) {
			add(currentIface)
		}
	}
	return trunks
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	statusOut, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, err
	}
	swOut, err := getSwitchportOutput(repo)
	if err != nil {
		return nil, err
	}
	trunks := parseTrunkInterfaces(swOut)
	trunkSet := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		trunkSet[t] = true
	}
	return parseActivePorts(statusOut, parseSwitchportVLANs(swOut), trunkSet), nil
}

func parseSwitchportVLANs(output string) map[string]string {
	result := make(map[string]string)
	var currentIface string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if iface, ok := parseSwitchportHeader(trimmed); ok {
			currentIface = iface
			continue
		}
		if currentIface == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "native vlan:") || strings.HasPrefix(lower, "pvid:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				vlan := strings.TrimSpace(parts[1])
				if isAccessVLAN(vlan) {
					result[currentIface] = vlan
				}
			}
		}
	}
	return result
}

func parseActivePorts(statusOutput string, switchportVLANs map[string]string, trunkSet map[string]bool) []entities.Port {
	var ports []entities.Port
	seen := make(map[string]bool)
	add := func(iface, vlan string) {
		if iface == "" || !isEthernetInterface(iface) || seen[iface] || trunkSet[iface] {
			return
		}
		if vlan == "" || strings.EqualFold(vlan, "trunk") || !isAccessVLAN(vlan) {
			return
		}
		seen[iface] = true
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}

	var currentIface string
	for _, line := range strings.Split(statusOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := infoPortRegex.FindStringSubmatch(strings.TrimSuffix(trimmed, ":")); m != nil {
			currentIface = normalizeInterface("Eth " + m[1])
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "link status:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "Up") && currentIface != "" {
				add(currentIface, switchportVLANs[currentIface])
			}
			currentIface = ""
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isEthernetInterface(iface) {
			continue
		}
		statusIdx := -1
		for i, f := range fields[1:] {
			lf := strings.ToLower(f)
			if lf == "up" || lf == "connected" || lf == "down" || lf == "notconnect" || lf == "disabled" {
				statusIdx = i + 1
				break
			}
		}
		if statusIdx < 0 || statusIdx+1 >= len(fields) {
			continue
		}
		status := strings.ToLower(fields[statusIdx])
		if status != "up" && status != "connected" {
			continue
		}
		vlan := fields[statusIdx+1]
		add(iface, vlan)
	}

	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func isAccessVLAN(s string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && n >= 1 && n <= 4094
}

func normalizeMAC(mac string) string {
	mac = strings.ToLower(mac)
	return strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac)
}

func (d *Driver) GetMacTable(repo ports.SwitchRepository) ([]entities.Device, error) {
	trunks, err := d.GetTrunkInterfaces(repo)
	if err != nil {
		return nil, err
	}
	trunkSet := make(map[string]bool, len(trunks))
	for _, t := range trunks {
		trunkSet[t] = true
	}

	out, err := repo.ExecuteCommand("show mac-address-table")
	if err != nil {
		return nil, err
	}
	return parseMacTable(out, trunkSet), nil
}

func parseMacTable(output string, trunkSet map[string]bool) []entities.Device {
	var devices []entities.Device
	for _, line := range strings.Split(output, "\n") {
		m := macLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		iface := normalizeInterface(m[1])
		if trunkSet[iface] {
			continue
		}
		macPlain := normalizeMAC(m[2])
		if len(macPlain) != 12 {
			continue
		}
		devices = append(devices, entities.Device{
			Vlan:      m[3],
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
			IsStatic:  strings.EqualFold(m[4], "static"),
		})
	}
	return devices
}

func formatInterfaceForCLI(name string) string {
	if strings.HasPrefix(name, "ethernet") {
		return "ethernet " + strings.TrimPrefix(name, "ethernet")
	}
	if strings.HasPrefix(name, "port-channel") {
		return "port-channel " + strings.TrimPrefix(name, "port-channel")
	}
	return name
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	iface := formatInterfaceForCLI(port.Interface)
	cmds := []string{"configure"}
	if port.Vlan != "" && port.Vlan != vlan {
		cmds = append(cmds,
			"interface vlan "+port.Vlan,
			"no set-member "+iface,
			"exit",
		)
	}
	cmds = append(cmds,
		"interface vlan "+vlan,
		"set-member untagged "+iface,
		"exit",
		"interface "+iface,
		"switchport native vlan "+vlan,
		"exit",
		"end",
	)
	return cmds
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"configure",
		"interface vlan " + vlan,
		"exit",
		"end",
	}
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	return []string{
		"configure",
		"no interface vlan " + vlan,
		"end",
	}
}

func (d *Driver) GetSessionProfile() entities.SessionProfile {
	return entities.SessionProfile{
		PromptPattern: dm3000PromptPattern,
		PagerPattern:  dm3000PagerPattern,
		PagerResponse: " ",
	}
}

func (d *Driver) NormalizeInterface(name string) string {
	return normalizeInterface(name)
}

func (d *Driver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show ip interface")
	if err != nil || d.IsCommandError(out) || strings.TrimSpace(out) == "" {
		out, err = repo.ExecuteCommand("show ip interface brief")
		if err != nil {
			return nil, err
		}
	}
	return parseL3Interfaces(out), nil
}

func parseL3Interfaces(output string) []string {
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		m := sviLineRegex.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		v := m[1]
		if !seen[v] {
			seen[v] = true
			vlans = append(vlans, v)
		}
	}
	sort.Slice(vlans, func(i, j int) bool {
		ai, _ := strconv.Atoi(vlans[i])
		bi, _ := strconv.Atoi(vlans[j])
		return ai < bi
	})
	return vlans
}

func (d *Driver) GetRunningConfig(repo ports.SwitchRepository) (string, error) {
	out, err := repo.ExecuteCommand("show running-config")
	if err != nil {
		return "", err
	}
	if d.IsCommandError(out) {
		return "", fmt.Errorf("show running-config failed")
	}
	return out, nil
}

func (d *Driver) SaveCommands() []string {
	return []string{
		"copy running-config startup-config",
		"write memory",
	}
}

func (d *Driver) ClearCache() {
	clearSwitchportCache()
}

var dm3000ErrorPatterns = []string{
	"unknown command",
	"unrecognized command",
	"invalid input",
	"incomplete command",
	"invalid command",
	"ambiguous command",
	"% invalid",
}

func (d *Driver) IsCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range dm3000ErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
