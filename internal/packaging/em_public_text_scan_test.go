package packaging

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// What the scan reports: a user home path, /srv/, an email address, an IPv4 address.
//
// Allowed, never reported:
//   - ~/ paths with no user name (they match no pattern below);
//   - email at the reserved example domains example.com, example.org, example.net
//     and the .invalid and .test top levels, and git@github.com;
//   - IPv4 loopback 127.0.0.0/8, 0.0.0.0, and the documentation ranges
//     192.0.2.0/24, 198.51.100.0/24 and 203.0.113.0/24.
var (
	emScanHomeRe  = regexp.MustCompile(`/(?:home|Users)/[^/\s"'` + "`" + `]+|[A-Za-z]:[\\/]+Users[\\/]+[^\\/\s"'` + "`" + `]+`)
	emScanSrvRe   = regexp.MustCompile(`/srv/`)
	emScanEmailRe = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+`)
	emScanIPRe    = regexp.MustCompile(`(?:^|[^0-9.])(\d{1,3}(?:\.\d{1,3}){3})(?:$|[^0-9.]|\.[^0-9])`)
)

var emScanAllowedMailDomains = []string{"example.com", "example.org", "example.net"}

var emScanAllowedNets = []string{"127.0.0.0/8", "0.0.0.0/32", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}

type emScanHit struct {
	File string
	Line int
	Text string
}

func (h emScanHit) String() string { return fmt.Sprintf("%s:%d: %s", h.File, h.Line, h.Text) }

func emScanMailAllowed(addr string) bool {
	lower := strings.ToLower(addr)
	if lower == "git@github.com" {
		return true
	}
	domain := lower[strings.LastIndex(lower, "@")+1:]
	for _, d := range emScanAllowedMailDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return strings.HasSuffix(domain, ".invalid") || strings.HasSuffix(domain, ".test")
}

func emScanIPAllowed(text string) bool {
	ip := net.ParseIP(text)
	if ip == nil {
		return true // not an address (an octet over 255, so a version string)
	}
	for _, cidr := range emScanAllowedNets {
		if _, n, _ := net.ParseCIDR(cidr); n.Contains(ip) {
			return true
		}
	}
	return false
}

// emScanLine returns the text of every disallowed match on one line.
func emScanLine(line string) []string {
	var out []string
	out = append(out, emScanHomeRe.FindAllString(line, -1)...)
	out = append(out, emScanSrvRe.FindAllString(line, -1)...)
	for _, m := range emScanEmailRe.FindAllString(line, -1) {
		if !emScanMailAllowed(m) {
			out = append(out, m)
		}
	}
	for _, m := range emScanIPRe.FindAllStringSubmatch(line, -1) {
		if !emScanIPAllowed(m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// emScanTree scans every regular UTF-8 file under tree. It returns the hits and
// the number of files read per top-level entry.
func emScanTree(t *testing.T, tree string) ([]emScanHit, map[string]int) {
	t.Helper()
	var hits []emScanHit
	counts := map[string]int{}
	err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
			return nil
		}
		rel, _ := filepath.Rel(tree, path)
		rel = filepath.ToSlash(rel)
		counts[strings.SplitN(rel, "/", 2)[0]]++
		for i, line := range strings.Split(string(data), "\n") {
			for _, text := range emScanLine(line) {
				hits = append(hits, emScanHit{File: rel, Line: i + 1, Text: text})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits, counts
}

func TestEmScan(t *testing.T) {
	plugin := filepath.Join(t.TempDir(), "plugin")
	if err := stage(moduleRoot(t), plugin, "0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	hits, counts := emScanTree(t, plugin)
	var names []string
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("files read in %s: %d", name, counts[name])
	}
	for _, want := range []string{"skills", "agents", "agent-templates", "LICENSE", "hooks", ".mcp.json", ".claude-plugin"} {
		if counts[want] == 0 {
			t.Errorf("scan read zero files in %s", want)
		}
	}
	for _, h := range hits {
		t.Errorf("private text in public tree: %s", h)
	}
}

func TestEmScanFindsPlantedHits(t *testing.T) {
	tree := t.TempDir()
	body := strings.Join([]string{
		"line one",
		"see /Users/ana/x",
		"~/.claude/agents",
		"see /home/ana/x",
		"mail sam@example.com",
		`path C:\Users\ana`,
		"host 127.0.0.1",
		"dir /srv/box",
		"docs 192.0.2.7",
		"mail ana@gmail.com",
		"git@github.com:o/r.git",
		"addr 10.1.2.3",
		"version 1.2.3",
		"",
	}, "\n")
	writeTestFile(t, filepath.Join(tree, "sub", "note.md"), body, 0o644)
	writeTestFile(t, filepath.Join(tree, "bin.dat"), "\xff\xfe /srv/skipped", 0o644)
	hits, counts := emScanTree(t, tree)
	var got []string
	for _, h := range hits {
		got = append(got, h.String())
	}
	want := []string{
		"sub/note.md:2: /Users/ana",
		"sub/note.md:4: /home/ana",
		`sub/note.md:6: C:\Users\ana`,
		"sub/note.md:8: /srv/",
		"sub/note.md:10: ana@gmail.com",
		"sub/note.md:12: 10.1.2.3",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("hits:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if counts["sub"] != 1 || counts["bin.dat"] != 0 {
		t.Fatalf("file counts %v: want one text file read, the non-UTF-8 file skipped", counts)
	}
}
