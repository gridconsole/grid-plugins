package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// The four events that can answer with a decision. A Go panic exits 2, which
// Claude Code reads as a block, so none of them may panic or decide on a bad
// input: PreToolUse and PermissionRequest run toolUse, Stop and SubagentStop run
// remaining, whose stop guard blocks only for a Job (no GRID_JOB is set here).
var ggGuardedEvents = []string{"PreToolUse", "PermissionRequest", "Stop", "SubagentStop"}

// The event list of TestRecordEventsNeverBlockOrDeny (records_test.go), copied
// because that list is local to its test.
var ggRecordEvents = []string{
	"SessionStart", "UserPromptSubmit", "SubagentStart", "PostToolUse", "PreCompact",
	"SessionEnd", "PostToolUseFailure",
	"heartbeat", "worker-exit", "history-log", "usage-log", "brain-read",
}

// ggIsolate points the four home variables at folders under a temp root and
// fails when HOME is not under it, before anything runs.
func ggIsolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, k := range []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"} {
		dir := filepath.Join(root, "env-"+strings.ToLower(k))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(k, dir)
	}
	if !strings.HasPrefix(os.Getenv("HOME"), root+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the temp root")
	}
	return root
}

// ggRun runs one event with the harness Getenv, which holds no key at all.
func ggRun(t *testing.T, event, input string, r where.Resolver, cwd string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	var code int
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("panic (exit 2 in the binary, read as a block): %v", p)
			}
		}()
		code = Run(Options{
			Event: event, Stdin: strings.NewReader(input), Stdout: &out, Stderr: &errOut,
			Resolver: r, Cwd: cwd, Getenv: func(string) string { return "" },
		})
	}()
	return out.String(), errOut.String(), code
}

// ggWell is a well-formed guarded event; ggCut is the same cut at 40 bytes.
func ggWell(event, cwd string) string {
	b, _ := json.Marshal(map[string]any{
		"hook_event_name": event, "session_id": "s-1", "cwd": cwd, "tool_use_id": "tu-1",
		"transcript_path": filepath.Join(cwd, "t.jsonl"), "agent_type": "grid:grid-builder", "agent_id": "a-1",
		"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(cwd, "task.md"), "content": "x"},
		"stop_hook_active": false,
	})
	return string(b)
}

func ggCut(s string) string { return s[:40] }

// Rows 1 and 2: every bad input over every home, with the event spelled as the
// table spells it and as hooks.json does (lower case).
func TestGgGuardedInputsNeverDecide(t *testing.T) {
	root := ggIsolate(t)
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("a file, not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly := filepath.Join(root, "readonly")
	if err := os.MkdirAll(filepath.Join(readOnly, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	fresh := filepath.Join(root, "fresh")
	if err := os.MkdirAll(fresh, 0o700); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(root, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	homes := map[string]string{
		"home under a file":   filepath.Join(blocker, "grid"),
		"read-only home":      readOnly,
		"home does not exist": filepath.Join(root, "missing", "grid"),
		"fresh empty home":    fresh,
	}
	inputs := map[string]func(event string) string{
		"empty":          func(string) string { return "" },
		"json null":      func(string) string { return "null" },
		"json array":     func(string) string { return "[1,2,3]" },
		"not json":       func(string) string { return "{not json" },
		"wrong types":    func(string) string { return `{"session_id":7,"cwd":false,"tool_input":"x","tool_name":[]}` },
		"truncated json": func(event string) string { return ggCut(ggWell(event, cwd)) },
	}
	for homeName, home := range homes {
		for inputName, input := range inputs {
			for _, event := range ggGuardedEvents {
				for spelling, name := range map[string]string{"table": event, "lower": strings.ToLower(event)} {
					t.Run(homeName+"/"+inputName+"/"+spelling+"/"+event, func(t *testing.T) {
						out, errOut, code := ggRun(t, name, input(event),
							where.Resolver{GridHome: home, Home: root, Machine: "m-1"}, cwd)
						if code != 0 {
							t.Fatalf("exit %d, want 0; stderr %q", code, errOut)
						}
						assertNoDecision(t, out)
					})
				}
			}
		}
	}
}

// ggMissing returns the names that none of the lists holds.
func ggMissing(names []string, lists ...[]string) []string {
	have := map[string]bool{}
	for _, l := range lists {
		for _, n := range l {
			have[n] = true
		}
	}
	var missing []string
	for _, n := range names {
		if !have[n] {
			missing = append(missing, n)
		}
	}
	return missing
}

// Row 3: a name added to Events() must be added to one of the two walks.
func TestGgGuardedEventsMatchTheRegistration(t *testing.T) {
	var names []string
	for _, e := range Events() {
		names = append(names, e.Name)
	}
	if len(names) == 0 {
		t.Fatal("Events() is empty")
	}
	if m := ggMissing(names, ggGuardedEvents, ggRecordEvents); len(m) > 0 {
		t.Fatalf("events in no input walk: %v", m)
	}
	// The check itself fails on a name neither list has.
	if m := ggMissing([]string{"PreToolUse", "NewEvent"}, ggGuardedEvents, ggRecordEvents); len(m) != 1 || m[0] != "NewEvent" {
		t.Fatalf("a name in neither list was not reported: %v", m)
	}
}

// Row 4: the walk is not vacuous. A well-formed PreToolUse Bash event with a
// session id and a tool use id leaves a history snapshot under that id (bashHistory);
// the same input cut at 40 bytes leaves none and answers no decision.
func TestGgGuardedContrastRecordWellFormedNotCut(t *testing.T) {
	ggIsolate(t)
	f := eventSetup(t)
	b, err := json.Marshal(hookio.Input{
		HookEventName: "PreToolUse", SessionID: "session-demo", ToolUseID: "tu-contrast", Cwd: f.dir, ToolName: "Bash",
		ToolInput: map[string]any{"command": "ls"},
	})
	if err != nil {
		t.Fatal(err)
	}
	well := string(b)
	snapshotted := func() bool {
		rows, err := (session.Store{TaskDir: f.dir}).Activities()
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range rows {
			if _, ok := a.Snapshots["tu-contrast"]; ok {
				return true
			}
		}
		return false
	}
	out, errOut, code := ggRun(t, "PreToolUse", ggCut(well), f.o.Resolver, f.dir)
	if code != 0 {
		t.Fatalf("cut input: exit %d, want 0; stderr %q", code, errOut)
	}
	assertNoDecision(t, out)
	if strings.Contains(out, "deny") {
		t.Fatalf("cut input answered with a decision: %q", out)
	}
	if snapshotted() {
		t.Fatal("cut input left a history snapshot")
	}
	out, errOut, code = ggRun(t, "PreToolUse", well, f.o.Resolver, f.dir)
	if code != 0 {
		t.Fatalf("well formed input: exit %d, want 0; stderr %q", code, errOut)
	}
	assertNoDecision(t, out)
	if !snapshotted() {
		t.Fatalf("well formed input left no history snapshot under its tool use id: out %q stderr %q", out, errOut)
	}
}
