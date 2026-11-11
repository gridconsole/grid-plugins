package lint_test

import (
	"strings"
	"testing"
)

// TestGuAtReplanParagraph: the orchestrator agent file says what to do when its
// Job is a re-plan Job (title prefix from internal/orchestrator/replan.go).
func TestGuAtReplanParagraph(t *testing.T) {
	text := readText(t, orchestratorPath)
	for _, want := range []string{"Re-plan after", "`grid job cancel", "--reason", "do not write `plan.md`"} {
		if !strings.Contains(text, want) {
			t.Errorf("agents/orchestrator.md lacks %q", want)
		}
	}
}
