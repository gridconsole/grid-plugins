package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

const pendingTask = "---\nstatus: active\nstep: plan\nworkflow: [{step: research, kind: work}, {step: plan, kind: approve}, {step: build, kind: work}]\n---\n"
const accepted = "- 2026-01-01T00:00 · plan accepted · by person\n"

type fixture struct {
	t                                              *testing.T
	root, home, grid, projects, task, repo, plugin string
	env                                            map[string]string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	// The guards report physical paths, and macOS temp folders sit behind a symlink.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root}
	f.home = filepath.Join(f.root, "home")
	f.grid = filepath.Join(f.home, ".grid")
	f.projects = filepath.Join(f.grid, "projects")
	f.task = filepath.Join(f.projects, "example", "tasks", "T-01-example")
	f.repo = filepath.Join(f.root, "repo")
	f.plugin = filepath.Join(f.root, "plugin")
	f.env = map[string]string{"HOME": f.home, "GRID_HOME": f.grid, "GRID_PROJECTS": f.projects, "PWD": f.repo, "CLAUDE_PLUGIN_DATA": filepath.Join(f.root, "data"), "CLAUDE_PLUGIN_ROOT": f.plugin}
	for _, d := range []string{f.task, f.repo, f.plugin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.write(filepath.Join(f.task, "task.md"), pendingTask)
	return f
}
func (f *fixture) write(path, text string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) link(from, to string) {
	f.t.Helper()
	if err := os.Symlink(to, from); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) input(tool, path string) hookio.Input {
	key := "file_path"
	if tool == "NotebookEdit" {
		key = "notebook_path"
	}
	if tool == "Bash" {
		key = "command"
	}
	if tool == "Agent" || tool == "Task" {
		key = "prompt"
	}
	if tool == "Glob" || tool == "Grep" {
		key = "path"
	}
	return hookio.Input{SessionID: "session", Cwd: f.task, HookEventName: "PreToolUse", ToolName: tool, ToolInput: map[string]any{key: path, "content": "text"}}
}
func (f *fixture) accept() { f.write(filepath.Join(f.task, "history.md"), accepted) }
func (f *fixture) pointer(ext string, dirs ...string) {
	f.write(filepath.Join(f.env["CLAUDE_PLUGIN_DATA"], "sessions", "session"+ext), strings.Join(dirs, "\n")+"\n")
}
func (f *fixture) otherTask(history string) string {
	d := filepath.Join(f.projects, "example", "tasks", "T-02-other")
	f.write(filepath.Join(d, "task.md"), pendingTask)
	f.write(filepath.Join(d, "history.md"), history)
	return d
}
func expect(t *testing.T, d hookio.Decision, want string) {
	t.Helper()
	if d.Behavior != want {
		t.Fatalf("decision %q, want %q: %s", d.Behavior, want, d.Reason)
	}
}

// Each header rule has paired permitted and refused cases. Silence is the
// legacy scripts' ordinary allow case; HomeAllow is the explicit grant hook.
func TestPlanHeaderRules(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*fixture, bool) hookio.Input
	}{
		{"planner own or unaccepted task", func(f *fixture, deny bool) hookio.Input {
			h := ""
			if deny {
				h = accepted
			}
			d := f.otherTask(h)
			i := f.input("Write", filepath.Join(d, "notes.md"))
			i.AgentType = "grid:grid-planner"
			return i
		}},
		{"planner artifacts exclude control files", func(f *fixture, deny bool) hookio.Input {
			p := "draft.md"
			if deny {
				p = "task.md"
			}
			i := f.input("Write", filepath.Join(f.projects, "example", "artifacts", p))
			i.AgentType = "grid-planner"
			return i
		}},
		{"planner task artifacts", func(f *fixture, deny bool) hookio.Input {
			d := f.otherTask(accepted)
			p := "draft.md"
			if deny {
				p = "history.md"
			}
			i := f.input("Write", filepath.Join(d, "artifacts", p))
			i.AgentType = "grid-planner"
			return i
		}},
		{"planner without task", func(f *fixture, deny bool) hookio.Input {
			p := filepath.Join(f.projects, "example", "notes.md")
			if deny {
				p = filepath.Join(f.repo, "file.go")
			}
			i := f.input("Write", p)
			i.Cwd = f.repo
			i.AgentType = "grid-planner"
			return i
		}},
		{"curator brain only outside own task", func(f *fixture, deny bool) hookio.Input {
			p := filepath.Join(f.grid, "knowledge", "note.md")
			if deny {
				p = filepath.Join(f.projects, "example", "notes.md")
			}
			i := f.input("Write", p)
			i.AgentType = "grid-curator"
			return i
		}},
		{"curator own task", func(f *fixture, deny bool) hookio.Input {
			d := f.task
			if deny {
				d = f.otherTask(accepted)
			}
			i := f.input("Write", filepath.Join(d, "notes.md"))
			i.AgentType = "grid-curator"
			return i
		}},
		{"worker proposes brain changes", func(f *fixture, deny bool) hookio.Input {
			p := filepath.Join(f.task, "brain-proposals", "note.md")
			if deny {
				p = filepath.Join(f.projects, "example", "Knowledge", "note.md")
			}
			i := f.input("Write", p)
			i.AgentType = "grid-builder"
			return i
		}},
		{"brain roles match exactly", func(f *fixture, deny bool) hookio.Input {
			i := f.input("Write", filepath.Join(f.grid, "KNOWLEDGE", "note.md"))
			i.AgentType = "grid:grid-curator"
			if deny {
				i.AgentType = "other:grid-curator"
			}
			return i
		}},
		{"tasks folder aliases", func(f *fixture, deny bool) hookio.Input {
			// Where tasks already exists, a folding volume reads TASKS as that same
			// folder; the other spelling only matters in a project without one.
			if deny {
				return f.input("Write", filepath.Join(f.projects, "fresh", "TASKS", "T-03", "notes.md"))
			}
			return f.input("Write", filepath.Join(f.projects, "example", "tasks", "T-03", "notes.md"))
		}},
		{"tasks folder odd sibling", func(f *fixture, deny bool) hookio.Input {
			p := "notes"
			if deny {
				p = "tasks."
			}
			return f.input("Write", filepath.Join(f.projects, "example", p, "T-03", "notes.md"))
		}},
		{"preview always needs a person", func(f *fixture, deny bool) hookio.Input {
			f.accept()
			i := f.input("Write", filepath.Join(f.task, "history.md:stream"))
			s := accepted
			if deny {
				s += "- time · preview approved · by person\n"
			}
			i.ToolInput["content"] = s
			return i
		}},
		{"plan ready needs recall", func(f *fixture, deny bool) hookio.Input {
			if !deny {
				f.write(filepath.Join(f.task, "plan.md"), "## How we approach it\nRecall: nothing found\n")
			}
			i := f.input("Write", filepath.Join(f.task, "history.md"))
			i.ToolInput["content"] = "- time · plan ready · by agent\n"
			return i
		}},
		{"gate names forbidden", func(f *fixture, deny bool) hookio.Input {
			p := "notes.md"
			if deny {
				p = "  SHIP   approved "
			}
			return f.input("Write", filepath.Join(f.task, p))
		}},
		{"reviewer remains inside projects", func(f *fixture, deny bool) hookio.Input {
			f.accept()
			p := filepath.Join(f.task, "verdict.md")
			if deny {
				p = filepath.Join(f.repo, "file.go")
			}
			i := f.input("Write", p)
			i.AgentType = "grid:grid-reviewer"
			return i
		}},
		{"checker remains inside projects", func(f *fixture, deny bool) hookio.Input {
			f.accept()
			p := filepath.Join(f.task, "verdict.md")
			if deny {
				p = filepath.Join(f.repo, "file.go")
			}
			i := f.input("Write", p)
			i.AgentType = "grid-checker"
			return i
		}},
	}
	for _, tt := range cases {
		for _, deny := range []bool{false, true} {
			suffix := "allow"
			want := ""
			if deny {
				suffix = "deny"
				want = "deny"
			}
			t.Run(tt.name+"/"+suffix, func(t *testing.T) {
				f := setup(t)
				i := tt.prepare(f, deny)
				d := EvaluatePlan(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, PlanGuard, i, f.env, d)
			})
		}
	}
}
func checkAdapter(t *testing.T, fn Adapter, in hookio.Input, env map[string]string, want hookio.Decision) {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, code := fn(b, env)
	legacyCompare(t, fn, in, env, out)
	if code != 0 || string(out) != string(want.Bytes()) {
		t.Fatalf("adapter: %s, %d; want %s", out, code, want.Bytes())
	}
}
