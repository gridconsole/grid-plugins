package guard

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// The plan guard no longer reads shell text or a plan ready line for a brain
// recall line (verdict v-6: brain entries arrive as skills, rules and Job
// pointers, not through a recall check). These tests hold a task whose plan is
// held and whose plan.md has no recall line, with plan-guard at block.

func emptyDecision(t *testing.T, d hookio.Decision) {
	t.Helper()
	if !reflect.DeepEqual(d, hookio.Decision{}) {
		t.Fatalf("decision %+v, want empty", d)
	}
}

func TestPlanGuardBashNoRecallRead(t *testing.T) {
	f := setup(t)
	planBlock(f)
	f.write(filepath.Join(f.task, "plan.md"), "## How we approach it\nNo recall line here.\n")
	f.write(filepath.Join(f.task, "history.md"), "- 2026-01-01T00:00 · task started · by person\n")
	plan := filepath.Join(f.task, "plan.md")
	history := filepath.Join(f.task, "history.md")
	for _, command := range []string{
		"node plan-hash.mjs " + plan + ` && echo "plan ready" >> ` + history,
		"node plan-hash.mjs " + plan,
		`echo '· plan ready ·' | tee ` + history,
	} {
		i := f.input("Bash", command)
		d := EvaluatePlan(i, f.root, f.env)
		emptyDecision(t, d)
		checkAdapter(t, PlanGuard, i, f.env, d)
	}
}

func TestPlanGuardEditPlanReadyNoRecallRead(t *testing.T) {
	f := setup(t)
	planBlock(f)
	f.write(filepath.Join(f.task, "plan.md"), "## How we approach it\nNo recall line here.\n")
	history := filepath.Join(f.task, "history.md")
	f.write(history, "- 2026-01-01T00:00 · task started · by person\n")
	i := f.input("Edit", history)
	i.ToolInput["old_string"] = "· task started · by person\n"
	i.ToolInput["new_string"] = "· task started · by person\n- 2026-01-01T00:01 · plan ready · by agent\n"
	d := EvaluatePlan(i, f.root, f.env)
	emptyDecision(t, d)
	checkAdapter(t, PlanGuard, i, f.env, d)

	w := f.input("Write", history)
	w.ToolInput["content"] = "- time · plan ready · by agent\n"
	d = EvaluatePlan(w, f.root, f.env)
	emptyDecision(t, d)
	checkAdapter(t, PlanGuard, w, f.env, d)
}

// The deletion did not widen plan.go: the reviewer and checker fence still denies
// at block level (the "reviewer remains inside projects" row in TestPlanHeaderRules).
func TestPlanGuardOtherRulesStillDeny(t *testing.T) {
	t.Run("reviewer fence", func(t *testing.T) {
		f := setup(t)
		planBlock(f)
		f.accept()
		i := f.input("Write", filepath.Join(f.repo, "file.go"))
		i.AgentType = "grid:grid-reviewer"
		d := EvaluatePlan(i, f.root, f.env)
		expect(t, d, "deny")
		checkAdapter(t, PlanGuard, i, f.env, d)
	})
}
