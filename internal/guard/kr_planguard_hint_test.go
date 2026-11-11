package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// krPlanGuardHint is the hint written out, not built from the constants in plan.go,
// so a change of the text or of the verb spelling fails here.
const krPlanGuardHint = " To turn this guard off, run grid hook level plan-guard off (or start Claude Code with GRID_PLAN_GUARD=off)."

func TestKrPlanGuardOffHintNamesTheCommand(t *testing.T) {
	if PlanGuardOffCommand != "grid hook level plan-guard off" {
		t.Fatalf("PlanGuardOffCommand = %q", PlanGuardOffCommand)
	}
	f := setup(t)
	given := filepath.Join(f.repo, "file.go")
	in := f.input("Write", given)
	in.AgentType = "grid:grid-planner"
	d := EvaluatePlan(in, f.root, f.env)
	if d.Behavior != "" {
		t.Fatalf("default plan-guard refused the write: %+v", d)
	}
	if len(d.Warnings) != 1 || !strings.HasPrefix(d.Warnings[0], "plan-guard: ") {
		t.Fatalf("want one plan-guard warning, got %+v", d)
	}
	if !strings.HasSuffix(d.Warnings[0], krPlanGuardHint) {
		t.Fatalf("warning %q does not end with %q", d.Warnings[0], krPlanGuardHint)
	}
}

func TestKrPlanGuardOffCommandTurnsTheGuardOff(t *testing.T) {
	f := setup(t)
	if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
		t.Fatal(err)
	}
	given := filepath.Join(f.repo, "file.go")
	in := f.input("Write", given)
	in.AgentType = "grid:grid-planner"
	if d := EvaluatePlan(in, f.root, f.env); len(d.Warnings) != 1 {
		t.Fatalf("before the setting the planner write should warn once, got %+v", d)
	}
	for _, s := range []settings.Store{{Home: f.grid, Project: "example"}, {Home: f.grid}} {
		if err := s.SetHook("plan-guard", "off"); err != nil {
			t.Fatal(err)
		}
	}
	d := EvaluatePlan(in, f.root, f.env)
	if d.Behavior != "" || len(d.Warnings) != 0 {
		t.Fatalf("plan-guard off: want no deny and no warning, got %+v", d)
	}
}
