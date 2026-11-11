package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// ggProjectSkip names the members that cannot run without touching the machine.
// Every entry names the member and the path:line of the call it would make. The
// walk runs with a PATH that holds only git, so a member that would start a
// program fails on the missing program; these stay empty unless a member shows
// a call that has no such guard.
var ggProjectSkip = map[string]string{}

// ggProjectKnownOpen records the members that, on the base, do not answer an
// unregistered project the way contract/README.md:38 says: member -> the exit
// and the stderr or stdout head seen. The test fails when a member misbehaves
// and is not here, and when a member here now answers as asked or is no longer
// in contract/commands.json, so shorten this map when the product is fixed.
var ggProjectKnownOpen = map[string]string{
	"task decline": `exit 1; stderr "grid task decline: scope: task T-99 not found in the Grid home"; stdout code no_such_task (the task is checked before the project)`,
	"task rename":  `exit 1; stderr "grid task rename: scope: task T-99 not found in the Grid home"; stdout code no_such_task (the task is checked before the project)`,

	"job ack":     `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job blocked": `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job cancel":  `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job next":    `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job order":   `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job post":    `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job result":  `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"job tree":    `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,
	"feed tick":   `exit 1; stderr "project \"absent\" is not registered"; stdout empty`,

	// These print their own JSON failure and leave stderr empty with --json.
	"job render": `exit 1; stderr "job: project \"absent\" is not registered" without --json, empty with --json`,
	"job show":   `exit 1; stderr "job: project \"absent\" is not registered" without --json, empty with --json`,
	"job verify": `exit 1; stderr "job: project \"absent\" is not registered" without --json, empty with --json`,
	"job trace":  `exit 1; stderr "job trace: project not found" without --json, empty with --json`,
	"job lint":   `exit 1; without --json stdout is the lint report "path: <file>\nok: false\n..." and stderr is empty`,

	"feed types": `exit 0; stdout {"types":[{"type":"ack",...}]}, the public schema, whatever the project`,

	// With --json brain writes {"error":"project \"absent\" is not registered"} to stderr.
	"brain accept":    `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain changes":   `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain check":     `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain index":     `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain keep":      `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain propose":   `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain revert":    `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain search":    `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain show":      `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain supersede": `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
	"brain topics":    `exit 1; stderr "brain: project \"absent\" is not registered" without --json, {"error":"project \"absent\" is not registered"} with --json`,
}

type ggProjectOutcome struct {
	member string
	argv   []string
	ok     bool
	seen   string
}

type ggProjectWalkResult struct {
	members []string
	ran     []string
	skipped []string
	outcome map[string]ggProjectOutcome
}

// ggProjectMembers reads contract/commands.json: every json:true subcommand with
// a --project flag, in file order.
func ggProjectMembers(t *testing.T) ([]string, map[string]contractCommandSub) {
	t.Helper()
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	var members []string
	subs := map[string]contractCommandSub{}
	for _, c := range doc.Commands {
		for _, sub := range c.Subcommands {
			if !sub.JSON || !slices.ContainsFunc(sub.Flags, func(f contractCommandFlag) bool { return f.Name == "--project" }) {
				continue
			}
			name := strings.TrimSpace(c.Name + " " + sub.Name)
			members = append(members, name)
			subs[name] = sub
		}
	}
	return members, subs
}

// ggProjectMembersRaw finds the same set from the untyped document, so the
// covering test compares two separate readings of the file.
func ggProjectMembersRaw(t *testing.T) []string {
	t.Helper()
	var doc struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range doc.Commands {
		subs, _ := c["subcommands"].([]any)
		for _, s := range subs {
			sub := s.(map[string]any)
			if on, _ := sub["json"].(bool); !on {
				continue
			}
			flags, _ := sub["flags"].([]any)
			for _, f := range flags {
				if f.(map[string]any)["name"] == "--project" {
					names = append(names, strings.TrimSpace(fmt.Sprint(c["name"], " ", sub["name"])))
					break
				}
			}
		}
	}
	return names
}

// ggProjectArgv builds the argv of one member: the guest parity argv as the base
// (read only), else one placeholder per positional; the required flags with a
// placeholder value; then the case flag.
func ggProjectArgv(member string, sub contractCommandSub, file string) []string {
	argv := strings.Fields(member)
	if base, ok := guestParityCommands[member]; ok {
		for _, a := range base {
			switch a {
			case "{task}":
				a = "T-99"
			case "{file}":
				a = file
			}
			argv = append(argv, a)
		}
	} else {
		for _, a := range sub.Args {
			if !strings.HasPrefix(a, "[") {
				argv = append(argv, "parity")
			}
		}
	}
	for _, f := range sub.Flags {
		if f.Required && f.Name != "--project" && !slices.Contains(argv, f.Name) {
			argv = append(argv, f.Name)
			if f.Takes == "value" {
				argv = append(argv, "parity")
			}
		}
	}
	return append(argv, "--project", "absent")
}

// ggProjectHead is the first 160 bytes of text, on one line.
func ggProjectHead(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", `\n`)
	if len(s) > 160 {
		s = s[:160] + "..."
	}
	return s
}

// ggProjectCode reads the code the way error_codes_test.go:57-67 does.
func ggProjectCode(out []byte) (code, message string, enveloped bool) {
	var got map[string]json.RawMessage
	if json.Unmarshal(out, &got) != nil {
		return "", "", false
	}
	_ = json.Unmarshal(got["code"], &code)
	if code != "" {
		return code, "", false
	}
	var failure struct{ Code, Message string }
	if json.Unmarshal(got["error"], &failure) != nil {
		return "", "", false
	}
	return failure.Code, failure.Message, true
}

var (
	ggProjectOnce   sync.Once
	ggProjectCached ggProjectWalkResult
)

// ggProjectWalkAll runs every member once; both tests read the one result.
func ggProjectWalkAll(t *testing.T) ggProjectWalkResult {
	t.Helper()
	ggProjectOnce.Do(func() { ggProjectCached = ggProjectWalkRun(t) })
	return ggProjectCached
}

func ggProjectWalkRun(t *testing.T) ggProjectWalkResult {
	h := newHarness(t)
	// No walk touches a real home or starts a program: the four homes are folders
	// of the harness, and PATH holds only git.
	for key, dir := range map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome,
		"CLAUDE_CONFIG_DIR": filepath.Join(h.root, "claude"), "CODEX_HOME": filepath.Join(h.root, "codex")} {
		h.mkdir(dir)
		t.Setenv(key, dir)
	}
	if home := os.Getenv("HOME"); !strings.HasPrefix(home, h.root+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the temp root: %s", home)
	}
	for _, key := range []string{"GRID_ROLE", "GRID_MEMBER", "GRID_WORKER", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_JOB", "GRID_WAKE", "GRID_ATTEMPT", "GRID_OWNER"} {
		t.Setenv(key, "")
	}
	bin := filepath.Join(h.root, "bin")
	h.mkdir(bin)
	if git, err := exec.LookPath("git"); err == nil {
		if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	h.env.Getenv = func(string) string { return "" }
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	file := filepath.Join(h.root, "input.md")
	h.write(file, approvedPlan)
	// The Grid home has its VERSION, so a verb that opens it gets past the version check.
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")

	members, subs := ggProjectMembers(t)
	res := ggProjectWalkResult{members: members, outcome: map[string]ggProjectOutcome{}}
	for _, member := range members {
		if _, skip := ggProjectSkip[member]; skip {
			res.skipped = append(res.skipped, member)
			continue
		}
		res.ran = append(res.ran, member)
		argv := ggProjectArgv(member, subs[member], file)
		o := ggProjectOutcome{member: member, argv: strings.Fields(h.norm(strings.Join(argv, " ")))}
		run := func(args []string) (int, string, string) {
			h.env.Stdin = strings.NewReader("")
			exit := h.run(cwd, args...)
			return exit, h.norm(h.out.String()), h.norm(h.errb.String())
		}
		plainExit, plainOut, plainErr := run(argv)
		exit, out, stderr := run(append(slices.Clone(argv), "--json"))
		o.ok, o.seen = true, fmt.Sprintf("exit %d; stderr %q", exit, ggProjectHead(stderr))
		fail := func(why string) {
			if o.ok {
				o.ok, o.seen = false, why+": "+o.seen
			}
		}
		switch {
		case exit == 0:
			fail("exit 0")
			o.seen += fmt.Sprintf("; stdout %q", ggProjectHead(out))
		case plainExit != exit || plainErr != stderr:
			fail("--json changed exit or stderr")
			o.seen += fmt.Sprintf("; plain exit %d, stderr %q", plainExit, ggProjectHead(plainErr))
		case plainOut != "":
			fail("plain stdout is not empty")
			o.seen += fmt.Sprintf("; plain stdout %q", ggProjectHead(plainOut))
		default:
			code, message, enveloped := ggProjectCode([]byte(out))
			o.seen += fmt.Sprintf("; stdout %q", ggProjectHead(out))
			switch {
			case out == "":
				fail("stdout is empty")
			case code != CodeNoSuchProject:
				fail(fmt.Sprintf("code %q", code))
			case enveloped && message != strings.TrimSuffix(stderr, "\n"):
				fail("message differs from stderr")
			}
		}
		res.outcome[member] = o
		// A verb that made the project folder is a finding of its own.
		if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "absent")); err == nil {
			t.Errorf("%s created <gridHome>/projects/absent", member)
			_ = os.RemoveAll(filepath.Join(h.gridHome, "projects", "absent"))
		}
	}
	if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "absent")); err == nil {
		t.Errorf("<gridHome>/projects/absent exists after the walk")
	}
	records, err := project.Load(h.gridHome)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range records {
		names = append(names, r.Name)
	}
	if !slices.Equal(names, []string{"example"}) {
		t.Errorf("registered projects after the walk = %v, want [example]", names)
	}
	return res
}

func ggProjectSorted(s []string) []string {
	s = slices.Clone(s)
	sort.Strings(s)
	return s
}

func TestGgUnknownProjectWalkCoversTheContract(t *testing.T) {
	res := ggProjectWalkAll(t)
	raw := ggProjectSorted(ggProjectMembersRaw(t))
	if len(raw) == 0 {
		t.Fatal("contract/commands.json lists no json member with --project")
	}
	got := ggProjectSorted(append(slices.Clone(res.ran), res.skipped...))
	if !slices.Equal(got, raw) {
		t.Errorf("walked plus skipped = %d members, contract/commands.json has %d\nwalked or skipped only: %v\ncontract only: %v",
			len(got), len(raw), ggProjectDiff(got, raw), ggProjectDiff(raw, got))
	}
	for member, why := range ggProjectSkip {
		if !slices.Contains(raw, member) {
			t.Errorf("skip entry %q is not a member of contract/commands.json", member)
		}
		if !strings.Contains(why, ".go:") {
			t.Errorf("skip entry %q names no path:line: %q", member, why)
		}
	}
}

func ggProjectDiff(a, b []string) []string {
	var only []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			only = append(only, x)
		}
	}
	return only
}

func TestGgUnknownProjectWalk(t *testing.T) {
	res := ggProjectWalkAll(t)
	open := 0
	for _, member := range res.ran {
		o := res.outcome[member]
		_, known := ggProjectKnownOpen[member]
		switch {
		case !o.ok:
			open++
			if !known {
				t.Errorf("%s %v: %s", member, o.argv, o.seen)
			}
		case known:
			t.Errorf("%s answers no_such_project now: remove it from ggProjectKnownOpen", member)
		}
	}
	for member := range ggProjectKnownOpen {
		if !slices.Contains(res.members, member) {
			t.Errorf("ggProjectKnownOpen names %q, which is not in contract/commands.json", member)
		}
	}
	t.Logf("known open: %d of %d", open, len(res.ran))
	for _, member := range res.ran {
		if o := res.outcome[member]; !o.ok {
			t.Logf("open %s %v -> %s", member, o.argv, o.seen)
		}
	}
	for member, why := range ggProjectSkip {
		t.Logf("skipped %s: %s", member, why)
	}
}
