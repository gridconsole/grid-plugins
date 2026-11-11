package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// These tests run grid brain write and rename through Run, the CLI entry point,
// so the verbs are reached through the brain dispatch and the person in the audit
// line comes from the environment the host sets.

type keMtFix struct {
	t        *testing.T
	home     string
	gridHome string
	vars     map[string]string
}

func keMtNew(t *testing.T, vars map[string]string) *keMtFix {
	t.Helper()
	home := t.TempDir()
	f := &keMtFix{t: t, home: home, gridHome: filepath.Join(home, ".grid"), vars: vars}
	if err := os.MkdirAll(f.gridHome, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *keMtFix) run(stdin string, args ...string) (int, string, string) {
	f.t.Helper()
	var out, errb bytes.Buffer
	vars := f.vars
	code := Run(args, Env{
		Stdout: &out, Stderr: &errb, Stdin: strings.NewReader(stdin),
		Cwd: f.home, Home: f.home, GridHome: f.gridHome, Ceiling: f.home,
		Getenv: func(key string) string { return vars[key] },
	})
	return code, out.String(), errb.String()
}

func keMtNote(name, body string, links ...brain.Link) string {
	if links == nil {
		links = []brain.Link{}
	}
	b, err := brain.Encode(brain.Note{Name: name, Topic: "decisions", Type: "reference",
		Description: "Read before changing the " + name + ".", Summary: "What the " + name + " says.",
		Source: "person:bo", Level: "home", Status: "accepted", Origin: brain.Origin{Kind: "session", ID: "s0"},
		Links: links, Body: body})
	if err != nil {
		panic(err)
	}
	return string(b)
}

var keMtRev = regexp.MustCompile(` v(\d+) rev ([0-9a-f]{16})\n`)

func keMtRevOf(t *testing.T, out string) string {
	t.Helper()
	m := keMtRev.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no rev in %q", out)
	}
	return m[2]
}

func TestKeMtWriteThroughRun(t *testing.T) {
	f := keMtNew(t, map[string]string{"GRID_WHO": "bo"})
	code, out, errb := f.run(keMtNote("x", "# X\nFirst.\n"), "brain", "write", "decisions/x", "--expect", "none", "--file", "-")
	if code != 0 || !strings.HasPrefix(out, "created decisions/x v1 rev ") {
		t.Fatalf("brain write new: %d %q %q", code, out, errb)
	}
	log, err := os.ReadFile(filepath.Join(f.gridHome, "knowledge", "log.md"))
	if err != nil || !strings.Contains(string(log), "| created |") || !strings.Contains(string(log), "person:bo") {
		t.Fatalf("log.md does not name person:bo for the write: %v %q", err, log)
	}
	rev := keMtRevOf(t, out)
	code, out, errb = f.run(keMtNote("x", "# X\nSecond.\n"), "brain", "write", "decisions/x", "--expect", rev, "--file", "-")
	if code != 0 || !strings.HasPrefix(out, "edited decisions/x v2 rev ") {
		t.Fatalf("brain write replace at its rev: %d %q %q", code, out, errb)
	}
	code, _, errb = f.run(keMtNote("x", "# X\nThird.\n"), "brain", "write", "decisions/x", "--expect", rev, "--file", "-")
	if code != 6 {
		t.Fatalf("brain write at a stale rev: %d %q, want 6", code, errb)
	}
}

func TestKeMtRenameThroughRun(t *testing.T) {
	f := keMtNew(t, map[string]string{"GRID_WHO": "bo"})
	code, out, errb := f.run(keMtNote("x", "# X\nFirst.\n"), "brain", "write", "decisions/x", "--expect", "none", "--file", "-")
	if code != 0 {
		t.Fatalf("brain write x: %d %q %q", code, out, errb)
	}
	rev := keMtRevOf(t, out)
	link := brain.Link{To: "brain:home/x", Rel: "related", Summary: "The note this one points at.", Why: "Shows why."}
	if code, out, errb := f.run(keMtNote("z", "# Z\nSee x.\n", link), "brain", "write", "decisions/z", "--expect", "none", "--file", "-"); code != 0 {
		t.Fatalf("brain write z: %d %q %q", code, out, errb)
	}
	code, out, errb = f.run("", "brain", "rename", "decisions/x", "y", "--expect", rev)
	if code != 0 || !strings.HasPrefix(out, "renamed decisions/x -> decisions/y v") || !strings.Contains(out, "linked decisions/z\n") {
		t.Fatalf("brain rename: %d %q %q", code, out, errb)
	}
	z, err := os.ReadFile(filepath.Join(f.gridHome, "knowledge", "decisions", "z.md"))
	if err != nil || !strings.Contains(string(z), "brain:home/y") || strings.Contains(string(z), "brain:home/x") {
		t.Fatalf("z does not link brain:home/y in place of brain:home/x: %v %q", err, z)
	}
}

func TestKeMtBrainHelp(t *testing.T) {
	f := keMtNew(t, nil)
	code, out, errb := f.run("", "brain", "help")
	if code != 0 {
		t.Fatalf("brain help: %d %q", code, errb)
	}
	for _, want := range []string{
		"|drop|restore|write|rename|",
		"\n  write <topic>/<name> --expect <16hex|none> --file <path|-> [--reason <why>]  ",
		"Exit 0 done, 1 store error, 2 usage or a bad note, 3 role refused, 6 stale rev, 7 not versioned, 9 refused.",
		"\n  rename <topic>/<name> <new-name> --expect <rev> [--reason <why>]  ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("brain help lacks %q:\n%s", want, out)
		}
	}
}

func TestKeMtGridHelpExits(t *testing.T) {
	f := keMtNew(t, nil)
	code, out, errb := f.run("", "help")
	if code != 0 {
		t.Fatalf("grid help: %d %q", code, errb)
	}
	for _, want := range []string{"show/search/topics/index/propose/accept/keep/decide/supersede/check/changes/revert/drop/restore/write/rename", "6 stale rev (read again)", "7 not versioned (nothing written)"} {
		if !strings.Contains(out, want) {
			t.Errorf("grid help lacks %q", want)
		}
	}
}
