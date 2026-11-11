package lint_test

import (
	"strings"
	"testing"
)

// The orchestrator agent text names the dead_wait orphan kind and says what to
// do with it.
func TestGuDwAgentText(t *testing.T) {
	text := readText(t, orchestratorPath)
	for _, want := range []string{"`dead_wait`", "rejected or cancelled", "cancel the waiting Job"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not contain %q", orchestratorPath, want)
		}
	}
}
