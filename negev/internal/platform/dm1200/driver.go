package dm1200

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
	portPathRe   = regexp.MustCompile(`^(\d+)/(\d+)/(\d+)$`)
	macTokenRe   = regexp.MustCompile(`(?i)\b([0-9a-f]{2}(?::[0-9a-f]{2}){5}|[0-9a-f]{2}(?:-[0-9a-f]{2}){5}|[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\b`)
	portTokenRe  = regexp.MustCompile(`\b(\d+/\d+/\d+)\b`)
	vlanTokenRe  = regexp.MustCompile(`\b(\d{1,4})(?:-(\d{1,4}))?\b`)
	sviNameRegex = regexp.MustCompile(`(?i)^Vlan(?:-interface)?(\d+)$`)
	// Hostname may be bare or parenthesized; Fastpath config modes append
	// another parenthetical before the mode char, e.g. "(DM1200E) (Vlan)#".
	dm1200PromptPattern = regexp.MustCompile(`(?m)^\(?[A-Za-z0-9_.-]+(?:-[A-Za-z0-9_.-]+)*\)?(?:\s+\([^)]+\))*\s*[>#]\s*$`)
	dm1200PagerPattern  = regexp.MustCompile(`(?m)--More--|---- More ----`)
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dm1200"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if err != nil {
		if !isUnrecognizedCommand(err) {
			return false, err
		}
		out, err = repo.ExecuteCommand("show sysinfo")
		if err != nil {
			return false, err
		}
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "dmswitch") || strings.Contains(lower, "3324") || strings.Contains(lower, "3224") {
		return false, nil
	}
	return strings.Contains(lower, "dm1200") || strings.Contains(lower, "1200e"), nil
}

func isUnrecognizedCommand(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "unknown command") ||
		strings.Contains(lower, "invalid command")
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "User:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "enable\n"},
		{WaitFor: "Password:", SendCmd: "ENABLE_PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	}
}

func normalizeInterface(name string) string {
	compact := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	lower := strings.ToLower(compact)
	lower = strings.TrimPrefix(lower, "ethernet")
	lower = strings.TrimPrefix(lower, "eth")
	if !portPathRe.MatchString(lower) {
		return compact
	}
	return lower
}

func isPortInterface(name string) bool {
	return portPathRe.MatchString(name)
}

func compareInterfaceNames(a, b string) bool {
	pa := strings.Split(a, "/")
	pb := strings.Split(b, "/")
	n := len(pa)
	if len(pb) < n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		out, err = repo.ExecuteCommand("show vlan brief")
		if err != nil {
			return nil, err
		}
	}
	return parseVLANs(out), nil
}

func parseVLANs(output string) []string {
	seen := make(map[int]bool)
	var ids []int
	for _, line := range strings.Split(output, "\n") {
		if parseutil.IsSeparatorLine(line) {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vlan") && strings.Contains(lower, "name") && strings.Contains(lower, "status") {
			continue
		}
		for _, m := range vlanTokenRe.FindAllStringSubmatch(line, -1) {
			start, err := strconv.Atoi(m[1])
			if err != nil || start < 1 || start > 4094 {
				continue
			}
			end := start
			if m[2] != "" {
				end, err = strconv.Atoi(m[2])
				if err != nil || end < start || end > 4094 {
					continue
				}
			}
			for id := start; id <= end; id++ {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	sort.Ints(ids)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.Itoa(id)
	}
	return out
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show vlan port all")
	if err == nil && strings.TrimSpace(out) != "" {
		trunks := parseTrunkFromVlanPortAll(out)
		if len(trunks) > 0 {
			return trunks, nil
		}
	}
	statusOut, statusErr := repo.ExecuteCommand("show interfaces status")
	if statusErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, statusErr
	}
	return parseTrunkFromStatus(statusOut), nil
}

func parseTrunkFromVlanPortAll(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isPortInterface(iface) {
			continue
		}
		if hasTaggedToken(fields[1:]) && !seen[iface] {
			seen[iface] = true
			trunks = append(trunks, iface)
		}
	}
	return trunks
}

func hasTaggedToken(fields []string) bool {
	for _, f := range fields {
		if f == "T" || strings.EqualFold(f, "tagged") || strings.EqualFold(f, "trunk") {
			return true
		}
	}
	return false
}

func hasUntaggedToken(fields []string) bool {
	for _, f := range fields {
		if f == "U" || strings.EqualFold(f, "untagged") || strings.EqualFold(f, "access") {
			return true
		}
	}
	return false
}

func parseTrunkFromStatus(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isPortInterface(iface) {
			continue
		}
		for _, f := range fields[1:] {
			if strings.EqualFold(f, "trunk") && !seen[iface] {
				seen[iface] = true
				trunks = append(trunks, iface)
				break
			}
		}
	}
	return trunks
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	out, err := repo.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, err
	}
	return parseActivePorts(out), nil
}

func parseActivePorts(output string) []entities.Port {
	var ports []entities.Port
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isPortInterface(iface) {
			continue
		}
		linkIdx := -1
		for i, f := range fields[1:] {
			lf := strings.ToLower(f)
			if lf == "up" || lf == "down" {
				linkIdx = i + 1
				break
			}
		}
		if linkIdx < 0 || !strings.EqualFold(fields[linkIdx], "up") {
			continue
		}
		vlan := fields[len(fields)-1]
		if !isAccessVLAN(vlan) {
			continue
		}
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}
	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func isAccessVLAN(s string) bool {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return err == nil && n >= 1 && n <= 4094
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

	out, err := repo.ExecuteCommand("show mac-addr-table")
	if err != nil || strings.TrimSpace(out) == "" {
		out, err = repo.ExecuteCommand("show mac address-table")
		if err != nil {
			return nil, err
		}
	}
	return parseMacTable(out, trunkSet), nil
}

func parseMacTable(output string, trunkSet map[string]bool) []entities.Device {
	var devices []entities.Device
	for _, line := range strings.Split(output, "\n") {
		macM := macTokenRe.FindStringSubmatch(line)
		portM := portTokenRe.FindStringSubmatch(line)
		if macM == nil || portM == nil {
			continue
		}
		iface := normalizeInterface(portM[1])
		if !isPortInterface(iface) || trunkSet[iface] {
			continue
		}
		macPlain := normalizeMAC(macM[1])
		if len(macPlain) != 12 {
			continue
		}
		vlan := "1"
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.Contains(f, "/") {
				continue
			}
			if normalizeMAC(f) == macPlain {
				continue
			}
			if isAccessVLAN(f) {
				vlan = f
				break
			}
		}
		devices = append(devices, entities.Device{
			Vlan:      vlan,
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
		})
	}
	return devices
}

func normalizeMAC(mac string) string {
	mac = strings.ToLower(mac)
	return strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac)
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return []string{
		"configure",
		"interface " + port.Interface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"vlan participation exclude 1",
		"exit",
		"exit",
	}
}

func (d *Driver) CreateVLANCommands(vlan string) []string {
	return []string{
		"vlan database",
		"vlan " + vlan,
		"exit",
	}
}

func (d *Driver) DeleteVLANCommands(vlan string) []string {
	if vlan == "1" {
		return nil
	}
	return []string{
		"vlan database",
		"no vlan " + vlan,
		"exit",
	}
}

func (d *Driver) GetSessionProfile() entities.SessionProfile {
	return entities.SessionProfile{
		PromptPattern: dm1200PromptPattern,
		PagerPattern:  dm1200PagerPattern,
		PagerResponse: " ",
	}
}

func (d *Driver) NormalizeInterface(name string) string {
	return normalizeInterface(name)
}

func (d *Driver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("show ip interface brief")
	if err != nil {
		return nil, err
	}
	return parseL3Interfaces(out), nil
}

func parseL3Interfaces(output string) []string {
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		m := sviNameRegex.FindStringSubmatch(fields[0])
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
	// Fastpath prompts "Are you sure you want to save? (y/n)"; answer y in-band
	// (same pattern as Comware save\ny), then ExecuteCommand appends a final \n.
	return []string{
		"copy system:running-config nvram:startup-config\ny",
		"write memory\ny",
	}
}

func (d *Driver) ClearCache() {}

var dm1200ErrorPatterns = []string{
	"invalid input",
	"unrecognized command",
	"incomplete command",
	"ambiguous command",
	"error:",
}

func (d *Driver) IsCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range dm1200ErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
