package lint_test

import (
	"strings"
	"testing"
)

const questionsSkillPath = "../../../skills/questions/SKILL.md"

// TestR9OaOrchestratorOneAskPerDecision: round 9, task T-02. The orchestrator
// asked one decision with a person Job and listed it in decisions_needed as
// well, so a second Job asked it again, and it asked a short flag nobody had
// asked for. agents/orchestrator.md says one decision has one ask, asks only
// what the person's words leave open, and sizes the Jobs to the change.
func TestR9OaOrchestratorOneAskPerDecision(t *testing.T) {
	text := readText(t, orchestratorPath)
	for _, want := range []string{
		"One decision has one ask",
		"a person Job or a question, not both",
		"what the person's words leave open and the work cannot go on without",
		"left out and named in the result",
		"begins with the id of that person Job",
		"the size of the change",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("agents/orchestrator.md lacks %q", want)
		}
	}
}

// TestR9OaQuestionsSkillOneAsk: the questions skill asks only what the person's
// words leave open, never a decision a person Job or question already asks, and
// leaves a new option out and names it in the result.
func TestR9OaQuestionsSkillOneAsk(t *testing.T) {
	text := readText(t, questionsSkillPath)
	for _, want := range []string{
		"One decision has one ask",
		"what the person's words leave open and the work cannot go on without",
		"already asks",
		"left out and named in the result",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/questions/SKILL.md lacks %q", want)
		}
	}
}
