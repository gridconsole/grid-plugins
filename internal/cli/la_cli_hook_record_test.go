package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// laCliBashHook makes a task in the harness's Grid home, runs `grid hook pretooluse`
// through Run with a Bash input whose cwd is that task folder, and returns the exit
// code, the hook's answer and the tool use ids the task's session activity holds
// a snapshot for. The snapshot is the PreToolUse record bashHistory writes; it does
// not depend on any guard's decision.
func laCliBashHook(t *testing.T, h *harness, toolUseID string) (int, string, []string) {
	t.Helper()
	r := where.Resolver{GridHome: h.gridHome, Home: h.root, Ceiling: h.root, Machine: "01LAPTOP"}
	made, err := (task.Store{Resolver: r}).New(task.NewOptions{Cwd: h.root, Words: "Record the hook", Project: where.Personal})
	if err != nil {
		t.Fatal(err)
	}
	h.env.Getenv = func(string) string { return "" }
	h.env.Cwd = made.Path
	b, err := json.Marshal(hookio.Input{
		ToolName: "Bash", Cwd: made.Path, SessionID: "session-demo", ToolUseID: toolUseID,
		ToolInput: map[string]any{"command": "ls"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.env.Stdin = bytes.NewReader(b)
	code := Run([]string{"hook", "pretooluse"}, h.env)
	rows, err := (session.Store{TaskDir: made.Path}).Activities()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, a := range rows {
		for id := range a.Snapshots {
			ids = append(ids, id)
		}
	}
	return code, h.out.String(), ids
}

// `grid hook pretooluse` through Run leaves a snapshot under the tool use id in the
// task's session activity: the record that the hook ran, kept when every guard goes.
// With an empty tool use id bashHistory returns early and nothing is recorded.
func TestLaCliHookRecordsBash(t *testing.T) {
	code, out, ids := laCliBashHook(t, newHarness(t), "use-1")
	if code != 0 || strings.Contains(out, `"deny"`) {
		t.Fatalf("exit %d: %s", code, out)
	}
	if len(ids) != 1 || ids[0] != "use-1" {
		t.Fatalf("snapshots %q, want [use-1]", ids)
	}
	code, out, ids = laCliBashHook(t, newHarness(t), "")
	if code != 0 || strings.Contains(out, `"deny"`) {
		t.Fatalf("empty id: exit %d: %s", code, out)
	}
	if len(ids) != 0 {
		t.Fatalf("snapshots %q with an empty tool use id, want none", ids)
	}
}
