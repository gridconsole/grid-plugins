package cli

import (
	"encoding/json"
	"errors"
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

// ggTaskWalked is every contract member (json: true, a <task> operand or a --task
// flag) this walk runs. TestGgUnknownTaskWalkCoversTheContract compares it, plus
// ggTaskSkipped, with the set read from contract/commands.json at test time.
var ggTaskWalked = []string{
	"where", "task attach", "task close", "task decline", "task delete", "task history",
	"task move", "task rename", "task reopen", "task show", "task status", "task stop",
	"plan check", "plan diff", "plan hash", "plan items", "plan migrate", "plan write",
	"job ack", "job blocked", "job cancel", "job lint", "job next", "job order", "job post",
	"job render", "job result", "job show", "job tree", "job verify",
	"lease release", "lease status", "lease take",
	"feed fold", "feed handover", "feed post", "feed read", "feed state", "feed tick", "feed types",
	"question add", "question ask", "question list", "question sync", "question tick", "question withdraw",
	"answer", "approve", "deliver", "proof add", "proof index", "history add", "land",
	"brain decide", "brain keep", "brain propose", "brain search", "brain show",
	"person approvers",
	"session handback", "session list", "session pause", "session resume", "session send",
	"session show", "session start", "session stop", "session switch",
	"intent claim", "intent done", "intent list", "intent release", "intent renew",
	"usage", "usage post", "usage show",
	"signal act",
	"artifact import", "artifact link", "artifact list", "artifact publish",
}

// ggTaskSkipped holds members that cannot run without touching the machine; each
// value names the call the member would make.
var ggTaskSkipped = map[string]string{}

// ggTaskOpen is what one member answered on the base commit.
type ggTaskOpen struct {
	Exit int
	Seen string
}

// ggTaskKnownOpen records members that do not answer an unknown task as rows 2-3 of
// the brief ask. Shorten it when the product is fixed: the walk fails for a member
// that is listed and now answers well.
//
// Seen on base b87fc364. The first group prints its diagnostic on stderr and writes
// nothing on stdout with --json (no envelope); the second prints a JSON object of
// its own on stdout with an "error" string, and stderr stays empty; feed types
// exits 0.
var ggTaskKnownOpen = map[string]ggTaskOpen{
	"job lint":   {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example"}`},
	"job render": {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example"}`},
	"job show":   {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example"}`},
	"job verify": {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example"}`},
	"feed types": {0, `stdout: {"types":[{"type":"ack","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"answer","versio...`},
}

// ggTaskMembers reads the member set from contract/commands.json.
func ggTaskMembers(t *testing.T) map[string]contractCommandSub {
	t.Helper()
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	members := map[string]contractCommandSub{}
	for _, c := range doc.Commands {
		for _, sub := range c.Subcommands {
			if !sub.JSON {
				continue
			}
			takes := slices.ContainsFunc(sub.Args, func(a string) bool { return strings.Contains(a, "<task>") }) ||
				slices.ContainsFunc(sub.Flags, func(f contractCommandFlag) bool { return f.Name == "--task" })
			if takes {
				members[strings.TrimSpace(c.Name+" "+sub.Name)] = sub
			}
		}
	}
	return members
}

func TestGgUnknownTaskWalkCoversTheContract(t *testing.T) {
	members := ggTaskMembers(t)
	if len(members) == 0 {
		t.Fatal("no member read from contract/commands.json")
	}
	covered := map[string]bool{}
	for _, name := range ggTaskWalked {
		if covered[name] {
			t.Errorf("%q is walked twice", name)
		}
		covered[name] = true
	}
	for name, why := range ggTaskSkipped {
		if covered[name] {
			t.Errorf("%q is both walked and skipped", name)
		}
		if !strings.Contains(why, ".go:") {
			t.Errorf("skip entry %q names no path:line: %q", name, why)
		}
		covered[name] = true
	}
	var missing, extra []string
	for name := range members {
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	for name := range covered {
		if _, ok := members[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("contract members not walked or skipped: %v; walked or skipped but not in the contract: %v", missing, extra)
	}
	t.Logf("contract members: %d, walked: %d, skipped: %d", len(members), len(ggTaskWalked), len(ggTaskSkipped))
}

// ggTaskArgv builds the argv of one member without --json: the guest parity argv when
// there is one, a T-99 operand where the contract has a <task> operand, else --task T-99.
func ggTaskArgv(t *testing.T, h *harness, name string, sub contractCommandSub) []string {
	t.Helper()
	base, ok := guestParityCommands[name]
	if !ok {
		t.Fatalf("%s: no guest parity argv to start from", name)
	}
	// Two parity argvs stop at a usage check before the task is read: a question id
	// is Q-<number>, and signal act takes --task only for --act add.
	switch name {
	case "question withdraw":
		base = []string{"Q-1", "--reason", "parity"}
	case "signal act":
		base = []string{"S-01ARZ3NDEKTSV4RRFFQ69G5FAV", "--act", "add"}
	}
	file := filepath.Join(h.root, "input.md")
	h.write(file, approvedPlan)
	var args []string
	hasTask := false
	for _, a := range base {
		switch a {
		case "{task}":
			a, hasTask = "T-99", true
		case "{file}":
			a = file
		}
		args = append(args, a)
	}
	operand := slices.ContainsFunc(sub.Args, func(a string) bool { return strings.Contains(a, "<task>") })
	switch {
	case hasTask:
	case operand:
		args = append([]string{"T-99"}, args...)
	default:
		args = append(args, "--task", "T-99")
	}
	for _, f := range sub.Flags {
		if f.Required && !slices.Contains(args, f.Name) {
			args = append(args, f.Name)
			if f.Takes != "none" {
				args = append(args, "parity")
			}
		}
	}
	return append(strings.Fields(name), args...)
}

func ggTaskHead(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 110 {
		s = s[:110] + "..."
	}
	return s
}

// ggTaskAnswer runs argv with --json and without it. It returns "" when the member
// answers as the brief asks, else what is wrong, with the exit and the head seen.
func ggTaskAnswer(h *harness, cwd string, argv []string) (problem string, exit int, seen string) {
	h.env.Stdin = strings.NewReader("")
	exit = h.run(cwd, append(slices.Clone(argv), "--json")...)
	out, stderr := h.out.String(), h.errb.String()
	h.env.Stdin = strings.NewReader("")
	plainExit := h.run(cwd, argv...)
	plainStderr := h.errb.String()
	head := func(s string) string { return ggTaskHead(h.norm(s)) }
	seen = "stderr: " + head(stderr)
	if stderr == "" {
		seen = "stdout: " + head(out)
	}
	switch {
	case exit == 0:
		return "exit 0 with --json", exit, seen
	case plainExit != exit || plainStderr != stderr:
		return "--json changed exit or stderr", exit, seen
	case !strings.Contains(stderr, "T-99"):
		return "stderr does not name T-99", exit, seen
	case out == "":
		return "empty stdout", exit, seen
	}
	dec := json.NewDecoder(strings.NewReader(out))
	var got map[string]json.RawMessage
	if err := dec.Decode(&got); err != nil {
		return "stdout is not one JSON object", exit, "stdout: " + head(out)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "stdout holds more than one JSON value", exit, "stdout: " + head(out)
	}
	var code, message string
	_ = json.Unmarshal(got["code"], &code)
	_ = json.Unmarshal(got["message"], &message)
	if code == "" {
		var failure struct{ Code, Message string }
		_ = json.Unmarshal(got["error"], &failure)
		code, message = failure.Code, failure.Message
	}
	switch {
	case code != CodeNoSuchTask:
		return "code is " + code + ", not no_such_task", exit, "stdout: " + head(out)
	// The signal act envelope carries the message without the "grid: " prefix of stderr.
	case message != strings.TrimSuffix(stderr, "\n") && "grid: "+message != strings.TrimSuffix(stderr, "\n"):
		return "message differs from stderr", exit, "stdout: " + head(out)
	}
	return "", exit, seen
}

func ggTaskTasksIn(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "task.md" {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func TestGgUnknownTaskWalk(t *testing.T) {
	members := ggTaskMembers(t)
	open, walked := 0, 0
	for _, name := range ggTaskWalked {
		sub, ok := members[name]
		if !ok {
			continue // the coverage test reports it
		}
		walked++
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			// Nothing reads or writes a real home; no child process runs.
			for key, dir := range map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome,
				"CLAUDE_CONFIG_DIR": filepath.Join(h.home, ".claude"), "CODEX_HOME": filepath.Join(h.home, ".codex"),
				"PATH": h.tmp} {
				t.Setenv(key, dir)
			}
			if home := os.Getenv("HOME"); !strings.HasPrefix(home, h.root+string(filepath.Separator)) {
				t.Fatalf("HOME is not under the test root")
			}
			h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
			h.env.Getenv = func(string) string { return "" }
			cwd := filepath.Join(h.root, "checkout")
			h.mkdir(cwd)
			h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
			if name == "signal act" { // the note exists, so only the task is unknown
				h.write(filepath.Join(h.gridHome, "projects", "example", "notes", "S-01ARZ3NDEKTSV4RRFFQ69G5FAV.md"),
					"---\nid: S-01ARZ3NDEKTSV4RRFFQ69G5FAV\nsource: typed\ntitle: A signal title\nstatus: new\ncaptured: 2020-01-01T00:00:00.000Z\nupdated: 2020-01-01T00:00:00.000Z\ntier: external\nbody_kind: data\n---\n"+actBody)
			}
			argv := ggTaskArgv(t, h, name, sub)
			problem, exit, seen := ggTaskAnswer(h, cwd, argv)
			known, listed := ggTaskKnownOpen[name]
			switch {
			case problem != "" && listed:
				open++
				if known.Exit != exit || known.Seen != seen {
					t.Logf("%s: still open, but now exit %d, %s (listed: exit %d, %s)", name, exit, seen, known.Exit, known.Seen)
				}
			case problem != "":
				t.Errorf("%s: %s: grid %s --json: exit %d, %s\n\tlist it: %q: {%d, %q},", name, problem, strings.Join(argv, " "), exit, seen, name, exit, seen)
			case listed:
				t.Errorf("%s answers an unknown task as asked: remove it from ggTaskKnownOpen", name)
			}
			if found := ggTaskTasksIn(filepath.Join(h.gridHome, "projects")); len(found) != 0 {
				t.Errorf("%s left task folders behind: %v", name, found)
			}
		})
	}
	for name := range ggTaskKnownOpen {
		if _, ok := members[name]; !ok {
			t.Errorf("ggTaskKnownOpen lists %q, which is no longer in contract/commands.json", name)
		}
		if !slices.Contains(ggTaskWalked, name) {
			t.Errorf("ggTaskKnownOpen lists %q, which the walk does not run", name)
		}
	}
	t.Logf("known open: %d of %d", open, walked)
}
