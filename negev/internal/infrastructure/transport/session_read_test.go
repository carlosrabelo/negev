package transport

import (
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestPromptDoesNotMatchHashInDescription(t *testing.T) {
	profile := defaultSessionProfile()

	partial := "Interface GigabitEthernet1/0/1\n Description: uplink #1 to core\n"
	if promptMatched(profile, partial) {
		t.Fatal("must not treat '#' inside a description as the CLI prompt")
	}

	complete := partial + "Switch#\n"
	if !promptMatched(profile, complete) {
		t.Fatal("expected prompt match on hostname# line")
	}
}

func TestPagerAdvanceAndStrip(t *testing.T) {
	profile := defaultSessionProfile()

	buf := "line1\n--More--\nline2\n"
	loc := pagerMatchLoc(profile, buf)
	if loc == nil {
		t.Fatal("expected --More-- pager match")
	}
	cleaned := stripPager(buf, loc)
	if strings.Contains(cleaned, "--More--") {
		t.Fatalf("pager text should be stripped, got %q", cleaned)
	}
	if !strings.Contains(cleaned, "line1") || !strings.Contains(cleaned, "line2") {
		t.Fatalf("expected payload retained, got %q", cleaned)
	}
	if profile.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q; expected space", profile.PagerResponse)
	}

	comware := "page1\n---- More ----\npage2\n"
	if pagerMatchLoc(profile, comware) == nil {
		t.Fatal("expected ---- More ---- pager match")
	}
}

func TestStripCommandEchoAndPrompt(t *testing.T) {
	raw := "show interfaces status\nGi1/0/1 connected 10\nSwitch#\n"
	got := stripCommandEchoAndPrompt(raw)
	if got != "Gi1/0/1 connected 10" {
		t.Fatalf("strip = %q", got)
	}
}

func TestEffectiveProfileFillsDefaults(t *testing.T) {
	p := effectiveProfile(entities.SessionProfile{})
	if p.PromptPattern == nil || p.PagerPattern == nil || p.PagerResponse != " " {
		t.Fatalf("expected defaults, got %+v", p)
	}
}

func TestComwarePromptAndPager(t *testing.T) {
	profile := defaultSessionProfile()

	cases := []struct {
		name string
		buf  string
		want bool
	}{
		{"user view", "display version\n<3Com>\n", true},
		{"system view", "system-view\n[3Com]\n", true},
		{"system subview", "interface Vlan-interface1\n[3Com-Vlan-interface1]\n", true},
		{"incomplete angle", "display version\n<3Com\n", false},
		{"hash in body still ignored", "Description: port #1\n", false},
	}
	for _, tc := range cases {
		if got := promptMatched(profile, tc.buf); got != tc.want {
			t.Errorf("%s: promptMatched = %v; expected %v", tc.name, got, tc.want)
		}
	}

	pagerBuf := "vlan 1\n---- More ----\nvlan 10\n"
	loc := pagerMatchLoc(profile, pagerBuf)
	if loc == nil {
		t.Fatal("expected ---- More ---- pager match for Comware")
	}
	cleaned := stripPager(pagerBuf, loc)
	if strings.Contains(cleaned, "---- More ----") {
		t.Fatalf("Comware pager text should be stripped, got %q", cleaned)
	}
	if profile.PagerResponse != " " {
		t.Fatalf("PagerResponse = %q; expected space to advance Comware pager", profile.PagerResponse)
	}
}
