package comware

import (
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
	comwareIfaceRegex = regexp.MustCompile(`(?i)^(?:Ethernet|GigabitEthernet|Ten-GigabitEthernet|Eth|GE|TENGE)\d+/\d+/\d+$`)
	macLineRegex      = regexp.MustCompile(`(?i)^\s*([0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4})\s+(\d+)\s+Learned\s+(\S+)`)
	vlanIDFallbackRe  = regexp.MustCompile(`(?i)VLAN\s+ID:\s*(\d+)`)
	sviNameRegex      = regexp.MustCompile(`(?i)^Vlan(?:-interface)?(\d+)$`)
	comwarePromptRe   = regexp.MustCompile(`(?m)^(?:<[^>\n]+>|\[[^\]\n]+\])\s*$`)
	comwarePagerRe    = regexp.MustCompile(`(?m)---- More ----`)
)

type Driver struct{}

var _ platform.SwitchDriver = (*Driver)(nil)

func init() {
	platform.Register(&Driver{})
}

func (d *Driver) Name() string { return "comware" }

func (d *Driver) Detect(repo ports.SwitchRepository) (bool, error) {
	out, err := repo.ExecuteCommand("display version")
	if err != nil {
		out, err = repo.ExecuteCommand("show version")
		if err != nil {
			return false, err
		}
	}
	lower := strings.ToLower(out)
	return strings.Contains(lower, "3com") ||
		strings.Contains(lower, "comware") ||
		strings.Contains(lower, "h3c"), nil
}

func (d *Driver) GetSessionProfile() entities.SessionProfile {
	return entities.SessionProfile{
		PromptPattern: comwarePromptRe,
		PagerPattern:  comwarePagerRe,
		PagerResponse: " ",
	}
}

func (d *Driver) GetAuthenticationSequence() []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "system-view\n"},
		{WaitFor: "]", SendCmd: "user-interface vty 0 4\n"},
		{WaitFor: "]", SendCmd: "screen-length 0\n"},
		{WaitFor: "]", SendCmd: "return\n"},
		{WaitFor: ">", SendCmd: ""},
	}
}

func normalizeInterface(name string) string {
	compact := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	lower := strings.ToLower(compact)

	try := func(prefixes []string, canonical string) (string, bool) {
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				rest := compact[len(p):]
				if portPathRe.MatchString(rest) {
					return canonical + rest, true
				}
			}
		}
		return "", false
	}

	if out, ok := try([]string{"ten-gigabitethernet", "tengigabitethernet", "tenge"}, "Ten-GigabitEthernet"); ok {
		return out
	}
	if out, ok := try([]string{"gigabitethernet", "ge"}, "GigabitEthernet"); ok {
		return out
	}
	if out, ok := try([]string{"ethernet", "eth"}, "Ethernet"); ok {
		return out
	}
	return compact
}

var portPathRe = regexp.MustCompile(`^\d+/\d+/\d+$`)

func (d *Driver) NormalizeInterface(name string) string {
	return normalizeInterface(name)
}

func compareInterfaceNames(a, b string) bool {
	path := func(s string) (int, int, int) {
		s = strings.TrimPrefix(s, "Ten-GigabitEthernet")
		s = strings.TrimPrefix(s, "GigabitEthernet")
		s = strings.TrimPrefix(s, "Ethernet")
		parts := strings.Split(s, "/")
		nums := make([]int, 3)
		for i := 0; i < len(parts) && i < 3; i++ {
			nums[i], _ = strconv.Atoi(parts[i])
		}
		return nums[0], nums[1], nums[2]
	}
	rank := func(s string) int {
		switch {
		case strings.HasPrefix(s, "Ethernet"):
			return 0
		case strings.HasPrefix(s, "GigabitEthernet"):
			return 1
		case strings.HasPrefix(s, "Ten-GigabitEthernet"):
			return 2
		default:
			return 3
		}
	}
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra < rb
	}
	u1, s1, p1 := path(a)
	u2, s2, p2 := path(b)
	if u1 != u2 {
		return u1 < u2
	}
	if s1 != s2 {
		return s1 < s2
	}
	return p1 < p2
}

func (d *Driver) GetVLANList(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("display vlan")
	if err != nil {
		return nil, err
	}
	return parseVLANs(out), nil
}

func parseVLANs(output string) []string {
	seen := make(map[string]bool)
	var vlans []string
	add := func(v string) {
		if v == "" || seen[v] {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 4094 {
			return
		}
		seen[v] = true
		vlans = append(vlans, v)
	}

	inList := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "The following VLANs exist:") {
			inList = true
			continue
		}
		if m := vlanIDFallbackRe.FindStringSubmatch(trimmed); m != nil {
			add(m[1])
			continue
		}
		if !inList {
			continue
		}
		if trimmed == "" {
			continue
		}
		for _, tok := range strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		}) {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			if i := strings.Index(tok, "("); i >= 0 {
				tok = tok[:i]
			}
			if strings.Contains(tok, "-") {
				parts := strings.SplitN(tok, "-", 2)
				if len(parts) != 2 {
					continue
				}
				start, err1 := strconv.Atoi(parts[0])
				end, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil || start > end {
					continue
				}
				for n := start; n <= end; n++ {
					add(strconv.Itoa(n))
				}
				continue
			}
			add(tok)
		}
	}

	sort.Slice(vlans, func(i, j int) bool {
		ai, _ := strconv.Atoi(vlans[i])
		bi, _ := strconv.Atoi(vlans[j])
		return ai < bi
	})
	return vlans
}

func (d *Driver) GetTrunkInterfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("display port trunk")
	if err != nil {
		return nil, err
	}
	trunks := parseTrunkInterfaces(out)
	if len(trunks) == 0 {
		brief, berr := repo.ExecuteCommand("display brief interface")
		if berr == nil {
			trunks = parseTrunkFromBrief(brief)
		}
	}
	return trunks, nil
}

func parseTrunkInterfaces(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	inList := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "The following trunk ports exist:") {
			inList = true
			continue
		}
		if !inList {
			continue
		}
		for _, tok := range strings.Fields(trimmed) {
			if !comwareIfaceRegex.MatchString(tok) {
				continue
			}
			canon := normalizeInterface(tok)
			if !seen[canon] {
				seen[canon] = true
				trunks = append(trunks, canon)
			}
		}
	}
	return trunks
}

func parseTrunkFromBrief(output string) []string {
	seen := make(map[string]bool)
	var trunks []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !comwareIfaceRegex.MatchString(fields[0]) {
			continue
		}
		if !strings.EqualFold(fields[4], "trunk") {
			continue
		}
		canon := normalizeInterface(fields[0])
		if !seen[canon] {
			seen[canon] = true
			trunks = append(trunks, canon)
		}
	}
	return trunks
}

func (d *Driver) GetActivePorts(repo ports.SwitchRepository) ([]entities.Port, error) {
	out, err := repo.ExecuteCommand("display brief interface")
	if err != nil {
		return nil, err
	}
	return parseActivePorts(out), nil
}

func parseActivePorts(output string) []entities.Port {
	var ports []entities.Port
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		if !comwareIfaceRegex.MatchString(fields[0]) {
			continue
		}
		// Link state: UP required; skip DOWN / ADM DOWN
		if fields[1] == "ADM" {
			continue
		}
		if !strings.EqualFold(fields[1], "UP") {
			continue
		}
		mode := strings.ToLower(fields[4])
		pvid := fields[5]
		port := entities.Port{
			Interface: normalizeInterface(fields[0]),
			Vlan:      pvid,
		}
		if mode == "trunk" || mode == "hybrid" {
			port.IsTrunk = true
		}
		ports = append(ports, port)
	}
	sort.Slice(ports, func(i, j int) bool {
		return compareInterfaceNames(ports[i].Interface, ports[j].Interface)
	})
	return ports
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
	out, err := repo.ExecuteCommand("display mac-address")
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
		iface := normalizeInterface(m[3])
		if trunkSet[iface] {
			continue
		}
		macPlain := strings.ToLower(strings.ReplaceAll(m[1], "-", ""))
		devices = append(devices, entities.Device{
			Vlan:      m[2],
			Mac:       macPlain,
			MacFull:   parseutil.FormatPlainMac(macPlain),
			Interface: iface,
		})
	}
	return devices
}

func (d *Driver) GetL3Interfaces(repo ports.SwitchRepository) ([]string, error) {
	out, err := repo.ExecuteCommand("display brief interface")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var vlans []string
	for _, line := range strings.Split(out, "\n") {
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
	return vlans, nil
}

func (d *Driver) ConfigureAccessCommands(port entities.Port, vlan string) []string {
	return nil
}

func (d *Driver) CreateVLANCommands(vlan string) []string { return nil }

func (d *Driver) DeleteVLANCommands(vlan string) []string { return nil }

func (d *Driver) SaveCommands() []string { return nil }

func (d *Driver) ClearCache() {}

func (d *Driver) IsCommandError(output string) bool { return false }
