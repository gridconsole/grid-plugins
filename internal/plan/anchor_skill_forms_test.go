package plan

import (
	"os"
	"strings"
	"testing"
)

// skillExamplePlan returns the plan.md example shown in skills/plan-jobs/SKILL.md.
func skillExamplePlan(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../skills/plan-jobs/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(b), "## Pointers into plan.md")
	if !ok {
		t.Fatal("skill has no 'Pointers into plan.md' part")
	}
	_, block, ok := strings.Cut(after, "```markdown\n")
	if !ok {
		t.Fatal("skill part has no markdown example")
	}
	plan, _, ok := strings.Cut(block, "```")
	if !ok {
		t.Fatal("markdown example is not closed")
	}
	return plan
}

func TestFfAnchorFormsSkillExampleResolves(t *testing.T) {
	plan := skillExamplePlan(t)
	got, err := Anchor(plan, "step-1")
	if err != nil || !strings.HasPrefix(got, "- [ ] 1 Update install.md") {
		t.Fatalf("step-1 = %q, %v", got, err)
	}
	got, err = Anchor(plan, "verify")
	if err != nil || got != "go test ./internal/plan/" {
		t.Fatalf("verify = %q, %v", got, err)
	}
	if !strings.HasSuffix(strings.TrimSpace(plan), "## Where it stands") {
		t.Fatal("example must end with ## Where it stands")
	}
}

func TestFfAnchorFormsHeadingStepsDoNotResolve(t *testing.T) {
	plan := "## Steps\n\n### Step 1: Update install.md\n\nWrite it.\n\n## Verify\n\ngo test ./...\n"
	for _, a := range []string{"step-1", "update-install-md", "step-1-update-installmd"} {
		if _, err := Anchor(plan, a); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("anchor %q: err = %v, want not found", a, err)
		}
	}
	if _, err := Anchor(plan, "verify"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
