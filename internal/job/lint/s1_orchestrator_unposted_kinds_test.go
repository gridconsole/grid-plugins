package lint_test

import (
	"strings"
	"testing"
)

// TestS1OrchestratorNamesUnpostedKinds: step 7 tells the orchestrator that the
// first Job's result names each step-2 kind it did not post, with one line why.
func TestS1OrchestratorNamesUnpostedKinds(t *testing.T) {
	text := readText(t, orchestratorPath)
	var step7 string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "7. Post one result") {
			step7 = line
		}
	}
	if step7 == "" {
		t.Fatal("agents/orchestrator.md has no line starting \"7. Post one result\"")
	}
	for _, want := range []string{"names each step-2 kind", "did not post", "one line why"} {
		if !strings.Contains(step7, want) {
			t.Errorf("step 7 lacks %q", want)
		}
	}
}
