package lint_test

import (
	"strings"
	"testing"
)

// r6ShipSentence is the plan-jobs rule that the orchestrator posts no Job for an
// approve step the workflow marks `posted_by: deliver` (built-in Ship).
const r6ShipSentence = "An approve step whose `posted_by` is `deliver` is not a Job you post: `grid deliver` posts that person Job, so your Jobs end at the step before it."

// r6ShipSkillProblem reports what is wrong with the plan-jobs text, or "": the
// sentence must stand on the owner-decision line that reads the workflow with
// `grid workflow show --task <task> --json` and names the person Job.
func r6ShipSkillProblem(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, tlWorkflowCommand) && strings.Contains(line, "grid job post --person") {
			if !strings.Contains(line, r6ShipSentence) {
				return "the workflow line does not hold: " + r6ShipSentence
			}
			return ""
		}
	}
	return "no line holds `" + tlWorkflowCommand + "` and `grid job post --person`"
}

func TestR6PlanJobsShipPostedByDeliver(t *testing.T) {
	skill := readText(t, skillPath)
	// The check must bite: the text with the sentence cut out fails.
	if r6ShipSkillProblem(strings.ReplaceAll(skill, r6ShipSentence, "")) == "" {
		t.Fatal("the check passes a skill text without the sentence, so it proves nothing")
	}
	if p := r6ShipSkillProblem(skill); p != "" {
		t.Errorf("skills/plan-jobs/SKILL.md: %s", p)
	}
}
