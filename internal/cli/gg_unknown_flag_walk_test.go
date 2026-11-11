package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// ggFlagCase is the flag no verb defines.
const ggFlagCase = "--gg-no-such-flag"

// ggFlagMember is one json:true subcommand of contract/commands.json.
type ggFlagMember struct {
	key string // "name sub", as guestParityCommands keys it
	sub contractCommandSub
}

// ggFlagSkip lists the members that are not run, each with the call it would make
// when it acts. Every one is a call that starts a process, opens a browser or
// writes a system timer, so it cannot run in a test without touching the machine.
var ggFlagSkip = map[string]string{
	"open":               "internal/cli/open.go:67 starts the system browser or app (exec.Command, cmd.Start at :68)",
	"routine run":        "internal/routine/execute.go:166 runs the routine's command (exec.CommandContext)",
	"routine schedule":   "internal/routine/schedule.go:37 runs the system scheduler program (exec.CommandContext)",
	"routine unschedule": "internal/routine/schedule.go:37 runs the system scheduler program (exec.CommandContext)",
	"land":               "internal/land/process.go:50 runs the deploy command (exec.CommandContext)",
	"eval run":           "internal/eval/process.go:58 runs the evaluation command (exec.CommandContext)",
	"account sign-in":    "internal/account/account.go:182 runs the agent program to sign in (exec.CommandContext)",
}

// ggFlagKnownOpen records, per member, what the base answers to an unknown flag
// when that is not the contract shape (exit 2, one invalid_argument object, no file
// written). Production code is left alone; shorten this map when a verb is fixed.
var ggFlagKnownOpen = map[string]string{
	"job lint":    `exit 2; stdout {"ok":false,"findings":[],"error":"unknown flag --gg-no-such-flag"} (own envelope, no code); stderr empty`,
	"job render":  `exit 2; stdout {"ok":false,"findings":[],"error":"unknown flag --gg-no-such-flag"} (own envelope, no code); stderr empty`,
	"job show":    `exit 2; stdout {"ok":false,"findings":[],"error":"unknown flag --gg-no-such-flag"} (own envelope, no code); stderr empty`,
	"job verify":  `exit 2; stdout {"ok":false,"findings":[],"error":"unknown flag --gg-no-such-flag"} (own envelope, no code); stderr empty`,
	"job trace":   `exit 2; stdout {"rows":[],"problems":[{"task":"","job":"","text":"unknown flag --gg-no-such-flag"}]} (own shape, no code); stderr empty`,
	"deliver":     `exit 2; stdout "Usage: grid deliver --outcome <text> [files...] ..." (usage on stdout, no envelope); stderr "flag provided but not defined: -gg-no-such-flag"`,
	"proof add":   `exit 2; stdout "Usage: grid proof add <item> <file|--text text> ..." (usage on stdout, no envelope); stderr "flag provided but not defined: -gg-no-such-flag -by string ..."`,
	"proof index": `exit 2; stdout "Usage: grid proof index [--task <task>] [--json] ..." (usage on stdout, no envelope); stderr "flag provided but not defined: -gg-no-such-flag -json print JSON ..."`,
	"history add": `exit 2; stdout "Usage: grid history add <line> [--task <task>] [--json] ..." (usage on stdout, no envelope); stderr "flag provided but not defined: -gg-no-such-flag -json print JSON ..."`,
}

func ggFlagMembers(t *testing.T) []ggFlagMember {
	t.Helper()
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	var out []ggFlagMember
	for _, c := range doc.Commands {
		for _, sub := range c.Subcommands {
			if sub.JSON {
				out = append(out, ggFlagMember{strings.TrimSpace(c.Name + " " + sub.Name), sub})
			}
		}
	}
	return out
}

// ggFlagCount counts json:true subcommands without the typed loader, so the
// coverage test does not share a bug with the list the walk uses.
func ggFlagCount(t *testing.T) (int, []string) {
	t.Helper()
	var raw struct {
		Commands []struct {
			Name        string
			Subcommands []struct {
				Name string
				JSON bool `json:"json"`
			}
		}
	}
	if err := json.Unmarshal(contractdata.Commands, &raw); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, c := range raw.Commands {
		for _, s := range c.Subcommands {
			if s.JSON {
				keys = append(keys, strings.TrimSpace(c.Name+" "+s.Name))
			}
		}
	}
	return len(keys), keys
}

// ggFlagSplit separates the members that are walked from the skipped ones.
func ggFlagSplit(members []ggFlagMember) (walked, skipped []ggFlagMember) {
	for _, m := range members {
		if _, ok := ggFlagSkip[m.key]; ok {
			skipped = append(skipped, m)
		} else {
			walked = append(walked, m)
		}
	}
	return
}

type ggFlagFixture struct {
	h    *harness
	cwd  string
	file string
}

// ggFlagFixtureFor makes the fake machine: every home variable under the temp root,
// one registered project example, one plain file for the {file} placeholder.
func ggFlagFixtureFor(t *testing.T) ggFlagFixture {
	t.Helper()
	h := newHarness(t)
	t.Setenv("HOME", h.home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
	t.Setenv("GRID_HOME", h.gridHome)
	if rel, err := filepath.Rel(h.root, os.Getenv("HOME")); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the temp root %s", h.root)
	}
	h.env.Getenv = func(string) string { return "" }
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	file := filepath.Join(h.root, "input.md")
	h.write(file, "parity\n")
	return ggFlagFixture{h, cwd, file}
}

// ggFlagValue is the placeholder value of a required flag that takes one.
func ggFlagValue(name string) string {
	switch name {
	case "--pid", "--pid-start", "--tokens":
		return "1"
	}
	return "parity"
}

// ggFlagBase is the argv of a member before the case flag: the guest parity argv
// when it has the member, else a placeholder for each positional, then each
// required flag that is not in it.
func ggFlagBase(m ggFlagMember, fx ggFlagFixture) []string {
	args := strings.Fields(m.key)
	if base, ok := guestParityCommands[m.key]; ok {
		repl := strings.NewReplacer("{task}", "T-01", "{file}", fx.file, "{project}", "example", "{cwd}", fx.cwd)
		for _, a := range base {
			args = append(args, repl.Replace(a))
		}
	} else {
		for _, a := range m.sub.Args {
			if strings.HasPrefix(a, "<") {
				args = append(args, "parity")
			}
		}
	}
	for _, f := range m.sub.Flags {
		if f.Required && !slices.Contains(args, f.Name) {
			args = append(args, f.Name)
			if f.Takes == "value" {
				args = append(args, ggFlagValue(f.Name))
			}
		}
	}
	return args
}

// ggFlagSnapshot records every path under root with its bytes.
func ggFlagSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(path)
			snap[rel] = "link " + target
		case d.IsDir():
			snap[rel] = "dir"
		default:
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snap[rel] = "file " + string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func ggFlagChanged(before, after map[string]string) []string {
	var paths []string
	for p, v := range after {
		if old, ok := before[p]; !ok || old != v {
			paths = append(paths, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

func ggFlagHead(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 110 {
		s = s[:110] + "..."
	}
	return s
}

// ggFlagInvalid reports what is wrong with the answer, "" when it is the contract
// shape: exit 2 and stdout exactly one JSON object whose error.code (or top-level
// code) is invalid_argument.
func ggFlagInvalid(exit int, stdout, stderr string) string {
	seen := fmt.Sprintf("exit %d; stdout %q; stderr %q", exit, ggFlagHead(stdout), ggFlagHead(stderr))
	if exit != 2 {
		return seen
	}
	dec := json.NewDecoder(strings.NewReader(stdout))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return seen
	}
	if _, err := dec.Token(); err != io.EOF {
		return seen
	}
	var code string
	if raw, ok := obj["code"]; ok {
		_ = json.Unmarshal(raw, &code)
	} else if raw, ok := obj["error"]; ok {
		var e struct{ Code string }
		_ = json.Unmarshal(raw, &e)
		code = e.Code
	}
	if code != CodeInvalidArgument {
		return seen
	}
	return ""
}

// ggFlagRun runs the member once with the case flag before or after --json and
// returns what is wrong, "" when nothing is.
func ggFlagRun(t *testing.T, m ggFlagMember, flagFirst bool) string {
	t.Helper()
	fx := ggFlagFixtureFor(t)
	args := ggFlagBase(m, fx)
	if flagFirst {
		args = append(args, ggFlagCase, "--json")
	} else {
		args = append(args, "--json", ggFlagCase)
	}
	before := ggFlagSnapshot(t, fx.h.root)
	exit := fx.h.run(fx.cwd, args...)
	stdout, stderr := fx.h.out.String(), fx.h.errb.String()
	problem := ggFlagInvalid(exit, stdout, stderr)
	if changed := ggFlagChanged(before, ggFlagSnapshot(t, fx.h.root)); len(changed) > 0 {
		if problem == "" {
			problem = fmt.Sprintf("exit %d", exit)
		}
		problem += "; changed paths " + strings.Join(changed, ", ")
	}
	if problem != "" {
		problem = fmt.Sprintf("%s: %s", strings.Join(args, " "), problem)
		problem = strings.ReplaceAll(problem, fx.h.root, "<ROOT>")
	}
	return problem
}

// The snapshot has to see a write, or the "writes nothing" row would hold for any verb.
func TestGgUnknownFlagSnapshotSeesAWrite(t *testing.T) {
	fx := ggFlagFixtureFor(t)
	before := ggFlagSnapshot(t, fx.h.root)
	if got := ggFlagChanged(before, ggFlagSnapshot(t, fx.h.root)); len(got) != 0 {
		t.Fatalf("untouched root changed: %v", got)
	}
	fx.h.write(filepath.Join(fx.h.gridHome, "new.txt"), "x")
	fx.h.write(fx.file, "changed\n")
	got := ggFlagChanged(before, ggFlagSnapshot(t, fx.h.root))
	want := []string{"home/.grid/new.txt", "input.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("changed = %v, want %v", got, want)
	}
	if err := os.Remove(fx.file); err != nil {
		t.Fatal(err)
	}
	if got := ggFlagChanged(before, ggFlagSnapshot(t, fx.h.root)); !slices.Contains(got, "input.md") {
		t.Fatalf("a removed file is not seen: %v", got)
	}
}

func TestGgUnknownFlagWalkCoversTheContract(t *testing.T) {
	members := ggFlagMembers(t)
	walked, skipped := ggFlagSplit(members)
	n, keys := ggFlagCount(t)
	if n == 0 {
		t.Fatal("contract/commands.json holds no json:true subcommand")
	}
	if got := len(walked) + len(skipped); got != n {
		t.Errorf("walked %d + skipped %d = %d, contract has %d json:true subcommands", len(walked), len(skipped), got, n)
	}
	var have []string
	for _, m := range members {
		have = append(have, m.key)
	}
	sort.Strings(have)
	sort.Strings(keys)
	if !slices.Equal(have, keys) {
		t.Errorf("walked set differs from the contract:\n got %v\nwant %v", have, keys)
	}
	for i := 1; i < len(have); i++ {
		if have[i] == have[i-1] {
			t.Errorf("duplicate member %q", have[i])
		}
	}
	for key, why := range ggFlagSkip {
		if !slices.Contains(keys, key) {
			t.Errorf("skip entry %q is not a json:true member", key)
		}
		if !strings.Contains(why, ".go:") {
			t.Errorf("skip entry %q names no path:line: %q", key, why)
		}
	}
	for key := range ggFlagKnownOpen {
		if !slices.Contains(keys, key) {
			t.Errorf("known-open entry %q is no longer in contract/commands.json", key)
		}
		if _, ok := ggFlagSkip[key]; ok {
			t.Errorf("known-open entry %q is also skipped", key)
		}
	}
	t.Logf("walked %d, skipped %d, contract %d", len(walked), len(skipped), n)
	for _, m := range skipped {
		t.Logf("skipped %s: %s", m.key, ggFlagSkip[m.key])
	}
}

func TestGgUnknownFlagWalk(t *testing.T) {
	walked, _ := ggFlagSplit(ggFlagMembers(t))
	open := 0
	for _, m := range walked {
		t.Run(m.key, func(t *testing.T) {
			var problems []string
			for _, flagFirst := range []bool{true, false} {
				if p := ggFlagRun(t, m, flagFirst); p != "" {
					problems = append(problems, p)
				}
			}
			known, listed := ggFlagKnownOpen[m.key]
			switch {
			case len(problems) > 0 && !listed:
				t.Errorf("%s misbehaves and is not in ggFlagKnownOpen:\n%s", m.key, strings.Join(problems, "\n"))
			case len(problems) == 0 && listed:
				t.Errorf("%s answers as the contract asks now; remove it from ggFlagKnownOpen (%s)", m.key, known)
			case len(problems) > 0:
				open++
				t.Logf("FINDING %s", problems[0])
			}
		})
	}
	t.Logf("known open: %d of %d", open, len(walked))
	if len(ggFlagKnownOpen) != open && !t.Failed() {
		t.Errorf("known-open map has %d entries, %d members misbehave", len(ggFlagKnownOpen), open)
	}
}
