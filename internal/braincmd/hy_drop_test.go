package braincmd_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/cli"
)

// These tests run grid brain drop and restore through the CLI entry point, so
// the person in the audit line comes from the environment the host sets.

type hyDrFixture struct {
	t        *testing.T
	home     string
	gridHome string
	vars     map[string]string
}

func hyDrNew(t *testing.T, vars map[string]string) *hyDrFixture {
	t.Helper()
	home := t.TempDir()
	f := &hyDrFixture{t: t, home: home, gridHome: filepath.Join(home, ".grid"), vars: vars}
	if err := os.MkdirAll(f.gridHome, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *hyDrFixture) run(args ...string) (int, string, string) {
	f.t.Helper()
	var out, errb bytes.Buffer
	vars := f.vars
	code := cli.Run(append([]string{"brain"}, args...), cli.Env{
		Stdout: &out, Stderr: &errb, Stdin: strings.NewReader(""),
		Cwd: f.home, Home: f.home, GridHome: f.gridHome, Ceiling: f.home,
		Getenv: func(key string) string { return vars[key] },
	})
	return code, out.String(), errb.String()
}

// note proposes and accepts one home entry in the decisions topic, as the
// person and role of the fixture.
func (f *hyDrFixture) note(name string) {
	f.t.Helper()
	code, _, errb := f.run("propose", "--level", "home", "--name", name, "--topic", "decisions",
		"--description", "Before changing quokka "+name+".", "--summary", "Quokka "+name+" summary.",
		"--source", "job:T-01/J-01", "--body", "# Quokka\nRun make test.\n")
	if code != 0 {
		f.t.Fatalf("propose %s: %d %s", name, code, errb)
	}
	if code, _, errb := f.run("accept", name, "--level", "home"); code != 0 {
		f.t.Fatalf("accept %s: %d %s", name, code, errb)
	}
}

func (f *hyDrFixture) path(name string) string {
	return filepath.Join(f.gridHome, "knowledge", "decisions", name+".md")
}

func (f *hyDrFixture) read(path string) []byte {
	f.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

// rev is the app's vaultRev, computed here apart from the plugin's own helper.
func (f *hyDrFixture) rev(name string) string {
	sum := sha256.Sum256(f.read(f.path(name)))
	return hex.EncodeToString(sum[:8])
}

func (f *hyDrFixture) log() []byte { return f.read(filepath.Join(f.gridHome, "knowledge", "log.md")) }

// tree reads every file under the Grid home but the lock files, so a refusal
// can be shown to have written nothing.
func (f *hyDrFixture) tree() map[string]string {
	f.t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(f.gridHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(d.Name(), "lock") {
			return err
		}
		rel, _ := filepath.Rel(f.gridHome, path)
		b, err := os.ReadFile(path)
		tree[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return tree
}

func (f *hyDrFixture) versions(name string) []string {
	f.t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.gridHome, "knowledge", "versions", "decisions", name))
	if err != nil {
		f.t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// listed reports whether search and the index both show the entry.
func (f *hyDrFixture) listed() (search, index bool) {
	f.t.Helper()
	code, out, errb := f.run("search", "quokka", "--level", "home", "--json")
	if code != 0 {
		f.t.Fatalf("search: %d %s", code, errb)
	}
	search = strings.Contains(out, "Quokka checks summary.")
	code, out, errb = f.run("index", "--level", "home", "--read-only", "--json")
	if code != 0 {
		f.t.Fatalf("index: %d %s", code, errb)
	}
	// The index lists a topic's entry count; the fixture's one entry is in decisions.
	var idx struct{ Topics []brain.TopicRow }
	if err := json.Unmarshal([]byte(out), &idx); err != nil {
		f.t.Fatalf("index JSON: %v: %s", err, out)
	}
	for _, row := range idx.Topics {
		if row.Topic == "decisions" {
			index = row.Entries == 1
		}
	}
	return search, index
}

func (f *hyDrFixture) changes() []brain.Change {
	f.t.Helper()
	code, out, errb := f.run("changes", "--level", "home", "--json")
	if code != 0 {
		f.t.Fatalf("changes: %d %s", code, errb)
	}
	var got struct{ Changes []brain.Change }
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		f.t.Fatalf("changes JSON: %v: %s", err, out)
	}
	return got.Changes
}

var hyDrMember = map[string]string{"GRID_MEMBER": "ana", "GRID_ROLE": "member"}

func hyDrVars(extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range hyDrMember {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestHyDrDropAndRestore(t *testing.T) {
	f := hyDrNew(t, hyDrVars(nil))
	f.note("checks")
	rev := f.rev("checks")
	before := f.read(f.path("checks"))
	versions := f.versions("checks")
	if s, i := f.listed(); !s || !i {
		t.Fatalf("the entry is not listed before the drop: search %v, index %v", s, i)
	}

	code, out, errb := f.run("drop", "decisions/checks", "--expect", rev, "--reason", "out of date", "--level", "home", "--json")
	if code != 0 {
		t.Fatalf("drop: %d %s", code, errb)
	}
	var res struct {
		Status  string
		Version int
		Rev     string
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Status != "dropped" || res.Version < 1 || res.Rev != rev {
		t.Fatalf("drop output %q: %+v, %v", out, res, err)
	}
	if !bytes.Equal(f.read(f.path("checks")), before) {
		t.Fatal("the note bytes changed")
	}
	if got := f.versions("checks"); !reflect.DeepEqual(got, versions) {
		t.Fatalf("versions %v, want %v", got, versions)
	}
	if s, i := f.listed(); s || i {
		t.Fatalf("a dropped entry is still listed: search %v, index %v", s, i)
	}
	if code, out, errb := f.run("show", "checks", "--level", "home", "--json"); code != 0 || !strings.Contains(out, `"dropped":true`) {
		t.Fatalf("show: %d %q %q", code, out, errb)
	}
	c := f.changes()[0]
	if c.Status != "dropped" || c.OriginKind != "person" || c.OriginID != "ana" || c.Reason != "out of date" || c.Topic != "decisions" || c.Name != "checks" {
		t.Fatalf("newest change: %+v", c)
	}

	code, out, errb = f.run("restore", "decisions/checks", "--expect", rev, "--level", "home", "--json")
	if code != 0 || !strings.Contains(out, `"status":"restored"`) {
		t.Fatalf("restore: %d %q %q", code, out, errb)
	}
	if s, i := f.listed(); !s || !i {
		t.Fatalf("a restored entry is not listed: search %v, index %v", s, i)
	}
	if code, out, _ := f.run("show", "checks", "--level", "home", "--json"); code != 0 || strings.Contains(out, `"dropped"`) {
		t.Fatalf("show after restore: %d %q", code, out)
	}
	if c := f.changes()[0]; c.Status != "restored" || c.OriginID != "ana" {
		t.Fatalf("newest change: %+v", c)
	}
	if !bytes.Equal(f.read(f.path("checks")), before) {
		t.Fatal("the note bytes changed after restore")
	}

	// Plain output names the result.
	if code, out, errb := f.run("drop", "decisions/checks", "--expect", rev, "--level", "home"); code != 0 || !strings.HasPrefix(out, "dropped decisions/checks v") {
		t.Fatalf("plain drop: %d %q %q", code, out, errb)
	}
	if code, _, _ := f.run("restore", "decisions/checks", "--expect", rev, "--level", "home"); code != 0 {
		t.Fatal("plain restore failed")
	}

	// The person comes from GRID_WHO, else GRID_ACTOR, else GRID_MEMBER, else the
	// session. Each case drops and then restores as ana.
	for _, c := range []struct {
		name         string
		vars         map[string]string
		kind, id     string
		sessionNoned bool
	}{
		{"who wins", map[string]string{"GRID_WHO": "bo", "GRID_ACTOR": "cy", "GRID_MEMBER": "ana"}, "person", "bo", false},
		{"actor beside member", map[string]string{"GRID_ACTOR": "cy", "GRID_MEMBER": "ana"}, "person", "cy", false},
		{"member alone", map[string]string{"GRID_MEMBER": "ana"}, "person", "ana", false},
		{"none is a session", map[string]string{}, "session", "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			other := hyDrFixture{t: t, home: f.home, gridHome: f.gridHome, vars: hyDrVars(nil)}
			for _, k := range []string{"GRID_WHO", "GRID_ACTOR", "GRID_MEMBER"} {
				delete(other.vars, k)
			}
			for k, v := range c.vars {
				other.vars[k] = v
			}
			if code, _, errb := other.run("drop", "decisions/checks", "--expect", rev, "--level", "home"); code != 0 {
				t.Fatalf("drop: %d %s", code, errb)
			}
			got := other.changes()[0]
			if got.Status != "dropped" || got.OriginKind != c.kind || (!c.sessionNoned && got.OriginID != c.id) || got.OriginID == "" {
				t.Fatalf("origin %s:%s, want %s:%s", got.OriginKind, got.OriginID, c.kind, c.id)
			}
			other.vars = hyDrVars(nil)
			if code, _, errb := other.run("restore", "decisions/checks", "--expect", rev, "--level", "home"); code != 0 {
				t.Fatalf("restore: %d %s", code, errb)
			}
		})
	}

	// No flag names the person.
	if code, _, _ := f.run("drop", "decisions/checks", "--expect", rev, "--by", "ana", "--level", "home"); code != 2 {
		t.Fatalf("--by exit %d, want 2", code)
	}
}

func TestHyDrRoles(t *testing.T) {
	for _, verb := range []string{"drop", "restore"} {
		for _, c := range []struct {
			role    string
			allowed bool
		}{
			{"owner", true}, {"member", true}, {"", true}, {"guest", false},
		} {
			t.Run(verb+"/role="+c.role, func(t *testing.T) {
				f := hyDrNew(t, hyDrVars(nil))
				f.note("checks")
				rev := f.rev("checks")
				if verb == "restore" {
					if code, _, errb := f.run("drop", "decisions/checks", "--expect", rev, "--level", "home"); code != 0 {
						t.Fatalf("setup drop: %d %s", code, errb)
					}
				}
				before := f.log()
				f.vars = hyDrVars(map[string]string{"GRID_ROLE": c.role})
				code, out, errb := f.run(verb, "decisions/checks", "--expect", rev, "--level", "home")
				if c.allowed {
					if code != 0 {
						t.Fatalf("exit %d, stderr %q", code, errb)
					}
					return
				}
				want := "grid brain " + verb + ": caller role is not allowed to " + verb + " brain entries\n"
				if code != 3 || out != "" || errb != want {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errb)
				}
				if !bytes.Equal(before, f.log()) {
					t.Fatal("log.md changed")
				}
			})
		}
	}
}

func TestHyDrRefusals(t *testing.T) {
	const zero = "0000000000000000"
	type refusal struct {
		name   string
		verb   string
		prep   func(f *hyDrFixture)
		args   func(f *hyDrFixture) []string
		code   int
		stderr func(f *hyDrFixture) map[string]string // nil: no JSON is printed
	}
	level := []string{"--level", "home"}
	target := func(rev func(f *hyDrFixture) string) func(f *hyDrFixture) []string {
		return func(f *hyDrFixture) []string {
			return append([]string{"decisions/checks", "--expect", rev(f)}, level...)
		}
	}
	current := func(f *hyDrFixture) string { return f.rev("checks") }
	for _, c := range []refusal{
		{name: "stale rev", verb: "drop", args: target(func(*hyDrFixture) string { return zero }), code: 6,
			stderr: func(f *hyDrFixture) map[string]string { return map[string]string{"error": "conflict", "rev": f.rev("checks")} }},
		{name: "stale rev on restore", verb: "restore", args: target(func(*hyDrFixture) string { return zero }), code: 6,
			stderr: func(f *hyDrFixture) map[string]string { return map[string]string{"error": "conflict", "rev": f.rev("checks")} }},
		{name: "no such note", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"decisions/nosuch", "--expect", zero}, level...) }, code: 8,
			stderr: func(*hyDrFixture) map[string]string { return map[string]string{"error": "no_such_note"} }},
		{name: "no such note on restore", verb: "restore", args: func(*hyDrFixture) []string { return append([]string{"decisions/nosuch", "--expect", zero}, level...) }, code: 8,
			stderr: func(*hyDrFixture) map[string]string { return map[string]string{"error": "no_such_note"} }},
		{name: "drop a dropped entry", verb: "drop", args: target(current), code: 9,
			prep: func(f *hyDrFixture) {
				if code, _, errb := f.run("drop", "decisions/checks", "--expect", f.rev("checks"), "--level", "home"); code != 0 {
					f.t.Fatalf("setup drop: %d %s", code, errb)
				}
			},
			stderr: func(*hyDrFixture) map[string]string {
				return map[string]string{"error": "refused", "reason": "note is already dropped"}
			}},
		{name: "restore one never dropped", verb: "restore", args: target(current), code: 9,
			stderr: func(*hyDrFixture) map[string]string {
				return map[string]string{"error": "refused", "reason": "note is not dropped"}
			}},
		{name: "drop a superseded entry", verb: "drop", args: target(current), code: 9,
			prep: func(f *hyDrFixture) {
				f.note("checks-v2")
				if code, _, errb := f.run("supersede", "checks", "--by", "checks-v2", "--reason", "moved to the v2 checks", "--level", "home"); code != 0 {
					f.t.Fatalf("setup supersede: %d %s", code, errb)
				}
			},
			stderr: func(*hyDrFixture) map[string]string {
				return map[string]string{"error": "refused", "reason": "note is superseded by brain:home/checks-v2"}
			}},
		{name: "topic machines", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"machines/laptop", "--expect", zero}, level...) }, code: 9},
		{name: "topic skills", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"skills/review", "--expect", zero}, level...) }, code: 9},
		{name: "topic machines on restore", verb: "restore", args: func(*hyDrFixture) []string { return append([]string{"machines/laptop", "--expect", zero}, level...) }, code: 9},
		{name: "path operand", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"../x", "--expect", zero}, level...) }, code: 2},
		{name: "no operand", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"--expect", zero}, level...) }, code: 2},
		{name: "no expect", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"decisions/checks"}, level...) }, code: 2},
		{name: "expect not hex", verb: "drop", args: func(*hyDrFixture) []string { return append([]string{"decisions/checks", "--expect", "xyz"}, level...) }, code: 2},
		{name: "expect too short", verb: "restore", args: func(*hyDrFixture) []string { return append([]string{"decisions/checks", "--expect", "abc123"}, level...) }, code: 2},
		{name: "unregistered project", verb: "drop", args: func(*hyDrFixture) []string {
			return []string{"decisions/checks", "--expect", zero, "--project", "no-such-project"}
		}, code: 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := hyDrNew(t, hyDrVars(nil))
			f.note("checks")
			if c.prep != nil {
				c.prep(f)
			}
			noteBefore := f.read(f.path("checks"))
			logBefore := f.log()
			treeBefore := f.tree()
			args := c.args(f)
			code, out, errb := f.run(append([]string{c.verb}, args...)...)
			if code != c.code {
				t.Fatalf("exit %d, want %d; stdout %q, stderr %q", code, c.code, out, errb)
			}
			if out != "" {
				t.Fatalf("stdout %q", out)
			}
			if c.stderr != nil {
				var got map[string]string
				if err := json.Unmarshal([]byte(errb), &got); err != nil || !reflect.DeepEqual(got, c.stderr(f)) {
					t.Fatalf("stderr %q: %v, want %v", errb, got, c.stderr(f))
				}
			}
			if c.code == 2 && !strings.Contains(errb, "Usage: grid brain "+c.verb) {
				t.Fatalf("a usage error prints the usage line: %q", errb)
			}
			if !bytes.Equal(noteBefore, f.read(f.path("checks"))) || !bytes.Equal(logBefore, f.log()) {
				t.Fatal("the note or log.md changed")
			}
			if !reflect.DeepEqual(treeBefore, f.tree()) {
				t.Fatal("the Grid home changed")
			}
		})
	}
}
