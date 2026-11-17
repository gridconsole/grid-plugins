package lint_test

import (
	"strings"
	"testing"
)

// The orchestrator agent text names the never_judged orphan kind and says that
// the Job's work is done, so it is not cancelled and posted again.
func TestR23OrchestratorNamesNeverJudged(t *testing.T) {
	text := readText(t, orchestratorPath)
	for _, want := range []string{"`never_judged`", "app's Judge", "do not cancel it"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not contain %q", orchestratorPath, want)
		}
	}
}

// docs/job-contract.md says who posts the review of a submitted Job and the rework
// after a rejection: the plugin's grid judge review and grid judge rework, while the
// app's Judge keeps running until A-worker-out. A done Job that nothing judges stays
// submitted and grid job tree names it never_judged.
func TestR23JobContractSaysTheJudgePostsReviewAndRework(t *testing.T) {
	text := strings.Join(strings.Fields(readText(t, "../../../docs/job-contract.md")), " ")
	for _, want := range []string{
		"The review of a submitted Job and the rework after a rejection are posted by the plugin's `grid judge review` and `grid judge rework`, and the app's Judge keeps running until A-worker-out",
		"`never_judged`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("docs/job-contract.md does not contain %q", want)
		}
	}
}
