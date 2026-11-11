package lint_test

import (
	"strings"
	"testing"
)

// TestJsOoOrphanNewJob: step 6 sends the agent to post an orphan's work as a
// new Job; a Job id is never posted twice, so "repost" must not come back.
func TestJsOoOrphanNewJob(t *testing.T) {
	text := readText(t, orchestratorPath)
	if strings.Contains(text, "repost or close") {
		t.Errorf("agents/orchestrator.md still holds %q", "repost or close")
	}
	var step6 string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "6. Each tick") {
			step6 = line
		}
	}
	if step6 == "" {
		t.Fatal("agents/orchestrator.md has no line starting \"6. Each tick\"")
	}
	if !strings.Contains(step6, "as a new Job") {
		t.Errorf("step 6 lacks %q", "as a new Job")
	}
}
