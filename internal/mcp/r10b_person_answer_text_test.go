package mcp

import (
	"os"
	"strings"
	"testing"
)

// r10bAnswerProblem reports what is wrong with an agent text that reads a Job's
// result, or "". The line that runs `grid feed read ... --type result` must say
// that a person Job's answer is the `control` record that closed it, in
// `body.answer`, the record feed_read and the CLI return for it
// (TestR10bMCPReadsPersonAnswerAsResult, end review 0412Z S2).
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
	read := func(rel string) string {
		raw, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	reviewer := read("../../agents/reviewer.md")
	orch := read("../../agents/orchestrator.md")
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
