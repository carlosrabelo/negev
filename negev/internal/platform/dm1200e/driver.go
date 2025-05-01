package dm1200e

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
	portPathRe      = regexp.MustCompile(`^(\d+)/(\d+)/(\d+)$`)
	macTokenRe      = regexp.MustCompile(`(?i)\b([0-9a-f]{2}(?::[0-9a-f]{2}){5}|[0-9a-f]{2}(?:-[0-9a-f]{2}){5}|[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})\b`)
	portTokenRe     = regexp.MustCompile(`\b(\d+/\d+/\d+)\b`)
	vlanRangeExprRe = regexp.MustCompile(`^\d{1,4}(?:-\d{1,4})?(?:,\d{1,4}(?:-\d{1,4})?)*$`)
	vlanTokenRe     = regexp.MustCompile(`(\d{1,4})(?:-(\d{1,4}))?`)
	firstVLANRe     = regexp.MustCompile(`\b(\d{1,4})\b`)
	sviLineRegex    = regexp.MustCompile(`(?i)^vlan(?:-interface)?\s*(\d{1,4})\b`)
	// Hostname may be bare or parenthesized; Fastpath config modes append
	// another parenthetical before the mode char, e.g. "(DM1200E) (Vlan)#".
	dm1200ePromptPattern = regexp.MustCompile(`(?m)^\(?[A-Za-z0-9_.-]+(?:-[A-Za-z0-9_.-]+)*\)?(?:\s+\([^)]+\))*\s*[>#]\s*$`)
	dm1200ePagerPattern  = regexp.MustCompile(`(?m)--More--|---- More ----`)
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string {
	return "dm1200e"
}

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("show version")
	if (err != nil && isUnrecognizedCommand(err)) || (err == nil && (d.IsCommandError(out) || strings.TrimSpace(out) == "")) {
		out, err = repo.ExecuteCommand("show sysinfo")
	}
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "dmswitch") || strings.Contains(lower, "dm3000") || strings.Contains(lower, "3324") || strings.Contains(lower, "3224") {
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
	if err != nil || strings.TrimSpace(out) == "" || d.IsCommandError(out) {
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
		if strings.Contains(lower, "vlan") && (strings.Contains(lower, "name") || strings.Contains(lower, "status") || strings.Contains(lower, "id")) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || !vlanRangeExprRe.MatchString(fields[0]) {
			continue
		}
		for _, m := range vlanTokenRe.FindAllStringSubmatch(fields[0], -1) {
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

type switchportInfo struct {
	Mode           string
	AccessVLAN     string
	GeneralPVID    string
	HasTaggedVLANs bool
}

func firstVLANNumber(s string) string {
	m := firstVLANRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	if isAccessVLAN(m[1]) {
		return m[1]
	}
	return ""
}

func parseSwitchportInfo(output string) map[string]switchportInfo {
	result := make(map[string]switchportInfo)
	var currentIface string
	var current switchportInfo
	flush := func() {
		if currentIface != "" {
			result[currentIface] = current
		}
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "port:") {
			flush()
			currentIface = ""
			current = switchportInfo{}
			iface := normalizeInterface(strings.TrimSpace(trimmed[len("port:"):]))
			if isPortInterface(iface) {
				currentIface = iface
			}
			continue
		}
		if currentIface == "" {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "vlan membership mode":
			current.Mode = strings.ToLower(val)
		case "access mode vlan":
			current.AccessVLAN = firstVLANNumber(val)
		case "general mode pvid":
			current.GeneralPVID = firstVLANNumber(val)
		case "general mode tagged vlans":
			if val != "" && !strings.EqualFold(val, "none") && !strings.EqualFold(val, "(none)") {
				current.HasTaggedVLANs = true
			}
		}
	}
	flush()
	return result
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	swOut, swErr := getSwitchportOutput(repo)
	if swErr == nil && !d.IsCommandError(swOut) {
		info := parseSwitchportInfo(swOut)
		if len(info) > 0 {
			var trunks []string
			for iface, st := range info {
				if st.Mode == "trunk" || (st.Mode == "general" && st.HasTaggedVLANs) {
					trunks = append(trunks, iface)
				}
			}
			sort.Slice(trunks, func(i, j int) bool {
				return compareInterfaceNames(trunks[i], trunks[j])
			})
			return trunks, nil
		}
	}

	out, err := repo.ExecuteCommand("show vlan port all")
	if err == nil && strings.TrimSpace(out) != "" && !d.IsCommandError(out) {
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
	portOut, err := repo.ExecuteCommand("show port all")
	if err == nil && strings.TrimSpace(portOut) != "" && !d.IsCommandError(portOut) {
		swOut, swErr := getSwitchportOutput(repo)
		if swErr != nil {
			return nil, swErr
		}
		if d.IsCommandError(swOut) {
			return nil, fmt.Errorf("show interfaces switchport failed")
		}
		return parseActivePortsFromPortAll(portOut, parseSwitchportInfo(swOut)), nil
	}

	statusOut, statusErr := repo.ExecuteCommand("show interfaces status")
	if statusErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, statusErr
	}
	return parseActivePortsFromStatus(statusOut), nil
}

func parseActivePortsFromPortAll(portAllOutput string, info map[string]switchportInfo) []entities.Port {
	var ports []entities.Port
	seen := make(map[string]bool)
	for _, line := range strings.Split(portAllOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		iface := normalizeInterface(fields[0])
		if !isPortInterface(iface) || seen[iface] {
			continue
		}

		linkUp := false
		foundLink := false
		for _, f := range fields[1:] {
			lf := strings.ToLower(f)
			if lf == "up" || lf == "down" {
				linkUp = lf == "up"
				foundLink = true
				break
			}
		}
		if !foundLink || !linkUp {
			continue
		}

		st, ok := info[iface]
		if !ok {
			// CPU/management interfaces (e.g. 0/4/1) have no switchport block.
			continue
		}
		if st.Mode == "trunk" {
			continue
		}
		var vlan string
		switch st.Mode {
		case "general":
			if st.HasTaggedVLANs {
				continue
			}
			vlan = st.GeneralPVID
		default:
			vlan = st.AccessVLAN
		}
		if !isAccessVLAN(vlan) {
			continue
		}
		seen[iface] = true
		ports = append(ports, entities.Port{Interface: iface, Vlan: vlan})
	}
	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
}

func parseActivePortsFromStatus(output string) []entities.Port {
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
	n, err := strconv.Atoi(strings.TrimSpace(s))
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
	if err != nil || strings.TrimSpace(out) == "" || d.IsCommandError(out) {
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
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		lastField := fields[len(fields)-1]
		if strings.EqualFold(lastField, "management") || strings.EqualFold(lastField, "self") || strings.EqualFold(lastField, "cpu") {
			continue
		}
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
		if isAccessVLAN(fields[0]) {
			vlan = fields[0]
		} else {
			for _, f := range fields {
				if strings.Contains(f, "/") {
					break
				}
				if normalizeMAC(f) == macPlain {
					continue
				}
				if isAccessVLAN(f) {
					vlan = f
					break
				}
			}
		}
		devices = append(devices, entities.Device{
			Vlan:      vlan,
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
			IsStatic:  strings.EqualFold(lastField, "static"),
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
		PromptPattern: dm1200ePromptPattern,
		PagerPattern:  dm1200ePagerPattern,
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
	// Fastpath prompts "Are you sure you want to save? (y/n)"; answer y in-band
	// (same pattern as Comware save\ny), then ExecuteCommand appends a final \n.
	return []string{
		"copy system:running-config nvram:startup-config\ny",
		"write memory\ny",
	}
}

func (d *Driver) ClearCache() {
	clearSwitchportCache()
}

var dm1200eErrorPatterns = []string{
	"invalid input",
	"unrecognized command",
	"incomplete command",
	"ambiguous command",
	"command not found",
	"an invalid interface",
	"% invalid",
	"error:",
}

func (d *Driver) IsCommandError(output string) bool {
	lower := strings.ToLower(output)
	for _, pat := range dm1200eErrorPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
