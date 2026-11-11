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

// Every role fence and the folder-name check answers through the plan-guard
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
		{"folder name beside tasks", "",
			func(f *fixture) string { return filepath.Join(f.projects, "example", "tasks.", "T-03", "notes.md") },
			func(f *fixture, given string) string {
				return "A folder beside a project's tasks folder keeps a plain name (printable ASCII, no colon, no dot or space at the end, not tasks in other letters): on macOS or Windows it can be the tasks folder under another spelling, so " + given + " is not written." + off
			}},
		{"builder writing the brain", "grid:grid-builder",
			func(f *fixture) string { return filepath.Join(f.grid, "knowledge", "note.md") },
			func(f *fixture, given string) string {
				return "Only a grid-curator (or a grid-planner) writes the brain; a grid-builder writes a proposal in its task's brain-proposals/ folder instead of " + given + "." + off
			}},
		{"curator outside the brain", "grid:grid-curator",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") },
			func(f *fixture, given string) string {
				return "A curator writes only the brain (a project's knowledge/ folder or the Grid home's) and its task folder. Write inside " + f.task + ", not " + given + "." + off
			}},
		{"planner outside its folders", "grid:grid-planner",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") },
			func(f *fixture, given string) string {
				return "A planner writes only inside a task folder (its own, or one whose plan is not accepted yet), an artifacts folder or a brain folder. Write inside " + f.task + ", not " + given + "." + off
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
