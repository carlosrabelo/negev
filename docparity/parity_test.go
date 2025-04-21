package docparity

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func headingCount(text, prefix string) int {
	n := 0
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, prefix) {
			n++
		}
	}
	return n
}

func fenceBodies(text string) []string {
	re := regexp.MustCompile("(?s)```\\w*\\n(.*?)```")
	matches := re.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func execLines(body string) []string {
	var out []string
	for _, ln := range strings.Split(body, "\n") {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if i := strings.Index(s, " #"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		if i := strings.Index(s, "  #"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

var cliFlags = []string{
	"target", "all", "group", "concurrency", "config", "write", "verbose",
	"create-vlans", "ask-pass", "backup", "backup-dir", "audit-log", "version",
}

func TestBilingualREADMEParity(t *testing.T) {
	root := repoRoot(t)
	en := read(t, root, "README.md")
	pt := read(t, root, "README-PT.md")
	if headingCount(en, "## ") != headingCount(pt, "## ") {
		t.Fatalf("## heading count EN=%d PT=%d", headingCount(en, "## "), headingCount(pt, "## "))
	}
	fe, fp := fenceBodies(en), fenceBodies(pt)
	if len(fe) != len(fp) {
		t.Fatalf("README fence count EN=%d PT=%d", len(fe), len(fp))
	}
	for i := range fe {
		ae, ap := execLines(fe[i]), execLines(fp[i])
		if strings.Join(ae, "\n") != strings.Join(ap, "\n") {
			t.Fatalf("README fence %d executable lines differ\nEN=%v\nPT=%v", i, ae, ap)
		}
	}
	if !strings.Contains(en, "docs/cli") || !strings.Contains(pt, "docs/cli") {
		t.Fatal("README pair must link to docs/cli")
	}
	for _, flag := range []string{"--target", "--write", "--all"} {
		if !strings.Contains(en, flag) || !strings.Contains(pt, flag) {
			t.Fatalf("flag %s missing in README bilingual pair", flag)
		}
	}
}

func TestBilingualGUIDEParity(t *testing.T) {
	root := repoRoot(t)
	en := read(t, root, "docs/GUIDE.md")
	pt := read(t, root, "docs/GUIDE-PT.md")
	if headingCount(en, "## ") != headingCount(pt, "## ") {
		t.Fatalf("GUIDE ## count EN=%d PT=%d", headingCount(en, "## "), headingCount(pt, "## "))
	}
	if headingCount(en, "### ") != headingCount(pt, "### ") {
		t.Fatalf("GUIDE ### count EN=%d PT=%d", headingCount(en, "### "), headingCount(pt, "### "))
	}
	fe, fp := fenceBodies(en), fenceBodies(pt)
	if len(fe) != len(fp) {
		t.Fatalf("GUIDE fence count EN=%d PT=%d", len(fe), len(fp))
	}
	for i := range fe {
		ae, ap := execLines(fe[i]), execLines(fp[i])
		if strings.Join(ae, "\n") != strings.Join(ap, "\n") {
			t.Fatalf("GUIDE fence %d executable lines differ\nEN=%v\nPT=%v", i, ae, ap)
		}
	}
	for _, platform := range []string{"ios", "nxos", "dmswitch", "dm1200", "comware"} {
		if !strings.Contains(en, platform) || !strings.Contains(pt, platform) {
			t.Fatalf("platform %s missing in GUIDE bilingual pair", platform)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "demos")); !os.IsNotExist(err) {
		t.Fatalf("demos/ must not exist: %v", err)
	}
}

func TestCLIFlagPages(t *testing.T) {
	root := repoRoot(t)
	for _, flag := range cliFlags {
		en := filepath.Join(root, "docs", "cli", flag+".md")
		pt := filepath.Join(root, "docs", "cli-pt", flag+".md")
		if _, err := os.Stat(en); err != nil {
			t.Fatalf("missing %s: %v", en, err)
		}
		if _, err := os.Stat(pt); err != nil {
			t.Fatalf("missing %s: %v", pt, err)
		}
	}
	for _, idx := range []string{
		filepath.Join(root, "docs", "cli", "README.md"),
		filepath.Join(root, "docs", "cli-pt", "README.md"),
		filepath.Join(root, "docs", "README.md"),
		filepath.Join(root, "docs", "README-PT.md"),
	} {
		if _, err := os.Stat(idx); err != nil {
			t.Fatalf("missing %s: %v", idx, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "man")); !os.IsNotExist(err) {
		t.Fatalf("docs/man/ must not exist: %v", err)
	}
}
