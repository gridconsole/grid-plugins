package lint_test

import (
	"strings"
	"testing"
)

// r11Orchestrator and r11Skill are the sentences that tell the orchestrator to
// post no review Job for a Job the worker reviews (eval rounds 10 and 11, P5).
const (
	r11Orchestrator = "A `check` step right after a `run` step gets no Job from you either: when a Job of the run step posts its result, the worker posts that Job's review itself (`J-2.1` reviews `J-2`), so a review you plan beside it reviews the change twice."
	r11Skill        = "A `check` step right after a `run` step is not a Job you post either: the worker posts the review of a run step's Job when that Job posts its result (`J-2.1` for `J-2`)."
)

// r11Problem reports what is wrong with a text, or "": the sentence stands on the
// line that reads the workflow, and no line still plans one review per build.
func r11Problem(text, sentence string) string {
	for _, stale := range []string{"one build and one review", "review, after a build, with the build Job in its `after`"} {
		if strings.Contains(text, stale) {
			return "the text still plans a review per build: " + stale
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, tlWorkflowCommand) {
			if !strings.Contains(line, sentence) {
				return "the workflow line does not hold: " + sentence
			}
			return ""
		}
	}
	return "no line holds `" + tlWorkflowCommand + "`"
}

func TestR11OrchestratorPlansNoReviewTheWorkerPosts(t *testing.T) {
	for _, c := range []struct{ path, sentence string }{{orchestratorPath, r11Orchestrator}, {skillPath, r11Skill}} {
		text := readText(t, c.path)
		// The check must bite: the text with the sentence cut out fails.
		if r11Problem(strings.ReplaceAll(text, c.sentence, ""), c.sentence) == "" {
			t.Fatalf("%s: the check passes a text without the sentence, so it proves nothing", c.path)
		}
		if p := r11Problem(text, c.sentence); p != "" {
			t.Errorf("%s: %s", c.path, p)
		}
	}
}
