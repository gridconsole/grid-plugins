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

// The case-folded path segments ClaudeBlock writes (claudeFold in
// internal/settings/claude.go), spelled out so a change to them shows here.
const (
	kgWriterProjects      = "[pP][rR][oO][jJ][eE][cC][tT][sS]"
	kgWriterTasks         = "[tT][aA][sS][kK][sS]"
	kgWriterPersonalTasks = "[pP][eE][rR][sS][oO][nN][aA][lL]/[tT][aA][sS][kK][sS]"
	kgWriterKnowledge     = "[kK][nN][oO][wW][lL][eE][dD][gG][eE]"
	kgWriterProjectMD     = "[pP][rR][oO][jJ][eE][cC][tT].[mM][dD]"
)

// kgWriterRules is the five Claude Code deny rules that carry what the only-writer
// hook guard used to refuse: project task folders, personal task folders, project
// knowledge, home knowledge and project.md. projects is the anchor of the projects
// folder, home the anchor of the Grid home.
func kgWriterRules(projects, home string) []string {
	return []string{
		"Edit(" + projects + "/*/" + kgWriterTasks + "/[Tt]-[0-9]*/**)",
		"Edit(" + home + "/" + kgWriterPersonalTasks + "/[Tt]-[0-9]*/**)",
		"Edit(" + projects + "/*/" + kgWriterKnowledge + "/**)",
		"Edit(" + home + "/" + kgWriterKnowledge + "/**)",
		"Edit(" + projects + "/*/" + kgWriterProjectMD + ")",
	}
}

// `grid setup --yes` writes the five rules into permissions.deny of the user
// settings.json, with the projects in the Grid home and outside it.
func TestKgWriterDenyRulesWritten(t *testing.T) {
	for _, location := range []string{"default", "outside home"} {
		t.Run(location, func(t *testing.T) {
			h := claudeHarness(t)
			projects := filepath.Join(h.gridHome, "projects")
			anchor := "~/.grid/" + kgWriterProjects
			if location == "outside home" {
				root, err := filepath.EvalSymlinks(h.root)
				if err != nil {
					t.Fatal(err)
				}
				projects = filepath.Join(root, "work")
				anchor = "//" + strings.TrimLeft(filepath.ToSlash(projects), "/")
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
			for _, want := range kgWriterRules(anchor, "~/.grid") {
				if !slices.Contains(doc.Permissions.Deny, want) {
					t.Errorf("permissions.deny lacks %s; it has %q", want, doc.Permissions.Deny)
				}
			}
		})
	}
}

// With the only-writer level at block, the hook no longer refuses a write into a
// task folder: no Grid guard stands behind the deny rules any more. A write to
// the host folder is still refused at host-guard block, which shows the hook ran.
func TestKgWriterHookNoLongerRefuses(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	grid := filepath.Join(home, ".grid")
	cwd := filepath.Join(root, "repo")
	store := settings.Store{Home: grid}
	for _, name := range []string{"only-writer", "host-guard"} {
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
	if control := run(filepath.Join(grid, ".host", "x")); !strings.Contains(control, `"deny"`) {
		t.Fatalf("the hook did not judge the control write: %s", control)
	}
}
