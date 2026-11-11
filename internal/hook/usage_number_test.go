package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/usage"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Usage is a number the hook never acts on: a task whose spend has passed a budget
// still starts agents, and the history snapshot still runs on its tool calls.
func TestSpentBudgetDoesNotDenyAnAgentStart(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "grid")
	task := filepath.Join(home, "projects", "example", "tasks", "T-01-demo")
	if err := os.MkdirAll(filepath.Join(task, "artifacts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "task.md"), []byte("---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := usage.Store{TaskPath: task, GridHome: home}
	dollars := 10.0
	if _, err := s.Post(usage.Input{Job: "lead", Dollars: &dollars}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(usage.Change{Budget: true, Dollars: []float64{10}}); err != nil {
		t.Fatal(err)
	}
	resolver := where.Resolver{GridHome: home, Home: root, Ceiling: root, Machine: "machine-demo"}
	run := func(tool string, input map[string]any) string {
		b, err := json.Marshal(hookio.Input{ToolName: tool, Cwd: task, SessionID: "session-demo", ToolUseID: "use-1", ToolInput: input})
		if err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		code := Run(Options{
			Event: "pretooluse", Stdin: bytes.NewReader(b), Stdout: &out, Stderr: &stderr, Resolver: resolver, Cwd: task,
			Getenv: func(k string) string {
				return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": filepath.Join(root, "data"), "GRID_PROJECTS": filepath.Join(home, "projects")}[k]
			},
		})
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("%s: exit %d: %s", tool, code, stderr.String())
		}
		return out.String()
	}
	agent := map[string]any{"prompt": "Map the settings code.", "description": "Map settings", "subagent_type": "grid:grid-scout"}
	for _, tool := range []string{"Agent", "Task"} {
		if out := run(tool, agent); bytes.Contains([]byte(out), []byte(`"deny"`)) {
			t.Fatalf("%s start denied at the budget: %s", tool, out)
		}
	}
	run("Bash", map[string]any{"command": "ls"})
	rows, err := (session.Store{TaskDir: task}).Activities()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range rows {
		if _, ok := a.Snapshots["use-1"]; ok {
			found = true
		}
	}
	if !found {
		t.Fatalf("history snapshot did not run for a PreToolUse at the budget: %+v", rows)
	}
}
