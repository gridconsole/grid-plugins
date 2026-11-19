package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// planBlock sets the plan-guard level of the project to block, as a person would
// with `grid hook level plan-guard block`.
func planBlock(f *fixture) {
	f.t.Helper()
	if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
		f.t.Fatal(err)
	}
	// The project's level wins where the caller's folder is in the project; the
	// personal level reaches a caller outside it (a planner in the repository).
	for _, s := range []settings.Store{{Home: f.grid, Project: "example"}, {Home: f.grid}} {
		if err := s.SetHook("plan-guard", "block"); err != nil {
			f.t.Fatal(err)
		}
	}
}

// The plan guard warns by default (needed.md:47, simplify.md:55 row 4). Its two
// approval-line denies are removed (simplify.md:40): a file at the top of a task
// folder named like a gate word, and a history.md edit that adds a preview
// approved line, are not refused at any level.
func TestPlanGuardRemovedApprovalLineDenies(t *testing.T) {
	for _, level := range []string{"default", "block"} {
		// gateLike matched the whole name, so main refused "plan accepted" but not
		// "plan accepted.md"; both are listed so the answer is shown for each.
		for _, name := range []string{"plan accepted", "plan accepted.md", "preview approved", "ship approved"} {
			t.Run("gate-word name "+name+"/"+level, func(t *testing.T) {
				f := setup(t)
				if level == "block" {
					planBlock(f)
				}
				d := EvaluatePlan(f.input("Write", filepath.Join(f.task, name)), f.root, f.env)
				if d.Behavior != "" || len(d.Warnings) != 0 {
					t.Fatalf("%s at %s: want no deny and no warning, got %+v", name, level, d)
				}
			})
		}
		t.Run("history preview approved/"+level, func(t *testing.T) {
			f := setup(t)
			f.accept()
			if level == "block" {
				planBlock(f)
			}
			history := filepath.Join(f.task, "history.md")
			in := f.input("Edit", history)
			in.ToolInput = map[string]any{
				"file_path":  history,
				"old_string": accepted,
				"new_string": accepted + "- 2026-01-01T00:01 · preview approved · by agent\n",
			}
			d := EvaluatePlan(in, f.root, f.env)
			if d.Behavior != "" || len(d.Warnings) != 0 {
				t.Fatalf("preview approved edit at %s: want no deny and no warning, got %+v", level, d)
			}
		})
	}
}

// The reviewer and checker fence answers through the plan-guard
// level: no setting is a warning that starts `plan-guard:` and no deny; block
// gives back the same deny text the fence had when it blocked by default.
func TestPlanGuardFencesWarnByDefaultAndDenyAtBlock(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
		path  func(*fixture) string
		want  func(f *fixture, given string) string
	}{
		{"reviewer outside the projects root", "grid:grid-reviewer",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") },
			func(f *fixture, given string) string {
				return "A grid-reviewer writes only its Verdict inside a task folder; " + given + " is outside it." + off
			}},
		{"checker outside the projects root", "grid:grid-checker",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") },
			func(f *fixture, given string) string {
				return "A grid-checker writes only its Verdict inside a task folder; " + given + " is outside it." + off
			}},
	} {
		t.Run(tc.name+"/default", func(t *testing.T) {
			f := setup(t)
			given := tc.path(f)
			in := f.input("Write", given)
			in.AgentType = tc.agent
			d := EvaluatePlan(in, f.root, f.env)
			if d.Behavior != "" {
				t.Fatalf("default plan-guard refused the write: %+v", d)
			}
			if len(d.Warnings) != 1 || !strings.HasPrefix(d.Warnings[0], "plan-guard:") {
				t.Fatalf("want one plan-guard warning, got %+v", d)
			}
			if want := "plan-guard: " + tc.want(f, given); d.Warnings[0] != want {
				t.Fatalf("warning %q, want %q", d.Warnings[0], want)
			}
			out := string(d.Bytes())
			if !strings.Contains(out, `"systemMessage":"plan-guard: `) || strings.Contains(out, "permissionDecision") {
				t.Fatalf("output must carry the warning and no decision: %s", out)
			}
		})
		t.Run(tc.name+"/block", func(t *testing.T) {
			f := setup(t)
			planBlock(f)
			given := tc.path(f)
			in := f.input("Write", given)
			in.AgentType = tc.agent
			d := EvaluatePlan(in, f.root, f.env)
			expect(t, d, hookio.Deny("").Behavior)
			if d.Reason != tc.want(f, given) || len(d.Warnings) != 0 {
				t.Fatalf("deny text %q (warnings %v), want %q", d.Reason, d.Warnings, tc.want(f, given))
			}
		})
	}
}

// No fence reaches a task folder, the brain, project.md or a folder name: these are
// agents' own state, and a refusal there stalled tasks on edge cases. At block too.
func TestPlanGuardLeavesTaskStateToAgents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
		path  func(*fixture) string
	}{
		{"builder writing the brain", "grid:grid-builder", func(f *fixture) string { return filepath.Join(f.grid, "knowledge", "note.md") }},
		{"builder writing project knowledge", "grid:grid-builder", func(f *fixture) string { return filepath.Join(f.projects, "example", "Knowledge", "note.md") }},
		{"folder name beside tasks", "", func(f *fixture) string { return filepath.Join(f.projects, "example", "tasks.", "T-03", "notes.md") }},
		{"tasks folder in other letters", "", func(f *fixture) string { return filepath.Join(f.projects, "fresh", "TASKS", "T-03", "notes.md") }},
		{"curator writing a repo file", "grid:grid-curator", func(f *fixture) string { return filepath.Join(f.repo, "file.go") }},
		{"planner writing a repo file", "grid:grid-planner", func(f *fixture) string { return filepath.Join(f.repo, "file.go") }},
		{"planner writing another task's folder", "grid-planner", func(f *fixture) string { return filepath.Join(f.otherTask(accepted), "notes.md") }},
		{"builder writing project.md", "grid:grid-builder", func(f *fixture) string { return filepath.Join(f.projects, "example", "project.md") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			planBlock(f)
			in := f.input("Write", tc.path(f))
			in.AgentType = tc.agent
			in.Cwd = f.repo
			if d := EvaluatePlan(in, f.root, f.env); d.Behavior != "" || len(d.Warnings) != 0 {
				t.Fatalf("want no answer, got %+v", d)
			}
		})
	}
}
