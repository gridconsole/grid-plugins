package lint_test

import (
	"strings"
	"testing"
)

// r10VerifySentence is the plan-jobs rule (real run r10 P2): a whole-suite verify
// is run once on the base first; a base that is already red gets a verify that
// names the touched tests, and the pre-existing failure is written out of scope.
const r10VerifySentence = "Before you name a whole-suite verify (`go test ./...`), run it once on the base; when it is already red, name the tests the change touches in verify and write the pre-existing failure in the Job's out of scope, so no builder is rejected for it."

// r10VerifySkillProblem reports what is wrong with the plan-jobs text, or "": the
// sentence must stand on the numbered Verify check (item 7).
func r10VerifySkillProblem(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "7. Verify:") {
			if !strings.Contains(line, r10VerifySentence) {
				return "the Verify check does not hold: " + r10VerifySentence
			}
			return ""
		}
	}
	return "no line starts with `7. Verify:`"
}

func TestR10PlanJobsVerifyPreexistingFail(t *testing.T) {
	skill := readText(t, skillPath)
	// The check must bite: the text with the sentence cut out fails.
	if r10VerifySkillProblem(strings.ReplaceAll(skill, r10VerifySentence, "")) == "" {
		t.Fatal("the check passes a skill text without the sentence, so it proves nothing")
	}
	if p := r10VerifySkillProblem(skill); p != "" {
		t.Errorf("skills/plan-jobs/SKILL.md: %s", p)
	}
}
