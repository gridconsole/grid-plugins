package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hook"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// `grid setup --yes` writes no deny rule: agents write task folders, the brain,
// project.md, a project's setup answers and the person's approval identity, with the
// projects in the Grid home and outside it. The settings package pins what it
// retires of the rules earlier setups wrote there.
func TestKgWriterDenyRulesNotWritten(t *testing.T) {
	for _, location := range []string{"default", "outside home"} {
		t.Run(location, func(t *testing.T) {
			h := claudeHarness(t)
			projects := filepath.Join(h.gridHome, "projects")
			if location == "outside home" {
				root, err := filepath.EvalSymlinks(h.root)
				if err != nil {
					t.Fatal(err)
				}
				projects = filepath.Join(root, "work")
			}
			t.Setenv("GRID_PROJECTS", projects)
			if code := h.run(h.home, "setup", "--yes"); code != 0 {
				t.Fatalf("code=%d: %s", code, h.errb)
			}
			data, err := os.ReadFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			var doc struct{ Permissions struct{ Deny []string } }
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Permissions.Deny) != 0 {
				t.Errorf("permissions.deny holds %q", doc.Permissions.Deny)
			}
		})
	}
}

// With the only-writer level at block, the hook no longer refuses a write into a
// task folder: no Grid guard stands behind the deny rules any more. A Bash call
// through `grid hook pretooluse` still leaves its snapshot in the task's session
// activity, which shows the hook ran.
func TestKgWriterHookNoLongerRefuses(t *testing.T) {
	h := newHarness(t)
	root, home, grid := h.root, h.home, h.gridHome
	cwd := filepath.Join(root, "repo")
	store := settings.Store{Home: grid}
	for _, name := range []string{"only-writer"} {
		if err := store.SetHook(name, "block"); err != nil {
			t.Fatal(err)
		}
	}
	run := func(path string) string {
		t.Helper()
		in := hookio.Input{ToolName: "Write", Cwd: cwd, ToolInput: map[string]any{"file_path": path, "content": "text"}}
		stdin, _ := json.Marshal(in)
		var out bytes.Buffer
		code := hook.Run(hook.Options{
			Event: "pretooluse", Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
			Resolver: where.Resolver{GridHome: grid, Home: home},
			Getenv: func(k string) string {
				return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": filepath.Join(root, "data")}[k]
			},
		})
		if code != 0 {
			t.Fatal(code)
		}
		return out.String()
	}
	got := run(filepath.Join(grid, "projects", "p", "tasks", "T-01-work", "brief.md"))
	if strings.Contains(got, `"deny"`) || strings.Contains(got, "written only by grid") {
		t.Fatalf("the hook still refuses a task-folder write: %s", got)
	}
	if code, out, ids := laCliBashHook(t, h, "use-1"); code != 0 || strings.Contains(out, `"deny"`) || !slices.Contains(ids, "use-1") {
		t.Fatalf("the hook left no record of the Bash call: exit %d, snapshots %q: %s", code, ids, out)
	}
}
