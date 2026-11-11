package contract

import (
	"strings"
	"testing"
)

// leQsStep2 returns step 2 of the questions skill, as kb3QStep2 reads it.
func leQsStep2(t *testing.T) string {
	t.Helper()
	return kb3QStep2(t, readPluginFile(t, "skills/questions/SKILL.md"))
}

// A Job agent with GRID_TASK_EPOCH asks as the task holder; one without it is
// still refused a person gate. Step 2 must say both.
func TestLeQsStep2NamesTheTaskEpoch(t *testing.T) {
	step := leQsStep2(t)
	for _, want := range []string{"GRID_TASK_EPOCH", "may not post person gates"} {
		if !strings.Contains(step, want) {
			t.Errorf("step 2 of skills/questions/SKILL.md lacks %q", want)
		}
	}
}

// The skill line stands on internal/cli/question.go reading GRID_TASK_EPOCH.
func TestLeQsCodeMatchesText(t *testing.T) {
	code := readPluginFile(t, "internal/cli/question.go")
	if !strings.Contains(code, "GRID_TASK_EPOCH") {
		t.Error("internal/cli/question.go no longer holds GRID_TASK_EPOCH; fix step 2 of skills/questions/SKILL.md")
	}
}
