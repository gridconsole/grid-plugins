package cli

import (
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// erDeliverNewText cuts out the text this lane adds to the orchestrator: the last
// sentence of step 5 and the paragraph after "8. Stop.".
func erDeliverNewText(t *testing.T, text string) []string {
	t.Helper()
	var out []string
	for _, start := range []string{"In your first Job, post one last Job", "When your Job is the one titled \"Deliver the task\""} {
		i := strings.Index(text, start)
		if i < 0 {
			t.Errorf("agents/orchestrator.md lacks the text that starts %q", start)
			continue
		}
		rest := text[i:]
		if j := strings.Index(rest, "\n"); j >= 0 {
			rest = rest[:j]
		}
		out = append(out, rest)
	}
	return out
}

func TestErDeliverTextTellsTheOrchestratorToPostAndRunTheDeliverJob(t *testing.T) {
	text := orchestratorText(t)
	for _, want := range []string{
		"Deliver the task",
		"`grid deliver --task <task> --outcome",
		"whose `after` names every other Job you posted",
		"role orchestrator",
		"names the curator Job it printed, or says none waited",
		"`grid job show <job> --task <task>`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("AgentFor(orchestrator) lacks %q", want)
		}
	}
	// The new text sits at the end of step 5 and after "8. Stop.", and renumbers nothing.
	step5, stop := strings.Index(text, "In your first Job, post one last Job"), strings.Index(text, "8. Stop.")
	para := strings.Index(text, "When your Job is the one titled")
	if step5 < 0 || stop < 0 || para < 0 || !(step5 < strings.Index(text, "\n6. ") && stop < para) {
		t.Errorf("new text is in the wrong place (step 5 sentence %d, stop %d, paragraph %d)", step5, stop, para)
	}
	if n := strings.Count(text, "\n8. Stop."); n != 1 {
		t.Errorf("want one line \"8. Stop.\", got %d", n)
	}
	if strings.Contains(text, "\n9. ") {
		t.Error("a step 9 was added")
	}
	for _, part := range erDeliverNewText(t, text) {
		low := strings.ToLower(part)
		for _, banned := range []string{"approve", "gate", "ask the person"} {
			if strings.Contains(low, banned) {
				t.Errorf("the new text holds %q: %s", banned, part)
			}
		}
	}
}

func TestErDeliverSkillNamesTheDeliverJob(t *testing.T) {
	skill, ok := gridplugins.SkillFor("plan-jobs")
	if !ok {
		t.Fatal("no skills/plan-jobs/SKILL.md")
	}
	var line string
	for _, l := range strings.Split(skill, "\n") {
		if strings.Contains(l, "Deliver the task") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("skills/plan-jobs/SKILL.md does not name the deliver Job")
	}
	for _, want := range []string{"last Job", "`after` names every other Job", "`grid deliver --task <task> --outcome"} {
		if !strings.Contains(line, want) {
			t.Errorf("the deliver Job line lacks %q: %s", want, line)
		}
	}
	low := strings.ToLower(line)
	for _, banned := range []string{"approve", "gate", "ask the person"} {
		if strings.Contains(low, banned) {
			t.Errorf("the deliver Job line holds %q", banned)
		}
	}
}
