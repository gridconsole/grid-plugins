package lint_test

import (
	"strings"
	"testing"
)

// r10bAnswerProblem reports what is wrong with an agent text that reads a Job's
// result, or "". The line that runs `grid feed read ... --type result` must say
// that a person Job's answer is the `control` record that closed it, in
// `body.answer` (internal/feed/read.go personClosings, end review 0412Z S2).
func r10bAnswerProblem(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "--type result`") {
			continue
		}
		if strings.Contains(line, "person Job") && strings.Contains(line, "`control`") && strings.Contains(line, "`body.answer`") {
			return ""
		}
	}
	return "no line that reads `--type result` says a person Job's answer is the `control` record's `body.answer`"
}

func TestR10bAgentsReadPersonAnswer(t *testing.T) {
	reviewer := readText(t, "../../../agents/reviewer.md")
	orch := readText(t, orchestratorPath)
	if r10bAnswerProblem(strings.ReplaceAll(reviewer, "`body.answer`", "`body.text`")) == "" {
		t.Fatal("the check passes a text that names body.text, so it proves nothing")
	}
	if p := r10bAnswerProblem(reviewer); p != "" {
		t.Errorf("agents/reviewer.md: %s", p)
	}
	if p := r10bAnswerProblem(orch); p != "" {
		t.Errorf("agents/orchestrator.md: %s", p)
	}
}
