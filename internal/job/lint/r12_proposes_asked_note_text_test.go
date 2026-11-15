package lint_test

import (
	"strings"
	"testing"
)

const (
	runJobSkillPath = "../../../skills/run-job/SKILL.md"
	builderPath     = "../../../agents/builder.md"
)

// TestR12AskedNotePathIsNamed: eval round 12, E3. The task asked "leave a note
// about how money is handled for whoever works on it next"; the builder wrote
// README.md and never ran `grid brain propose`, so no curator Job and no brain
// note followed. The planner's Job had only a README acceptance row, and the
// builder text proposed only a fact "to leave for later tasks". The text now has
// each end of the path: plan-jobs names the note in a row of its own, run-job and
// the builder propose it with `grid brain propose --task` and list it in the
// result's `knowledge`, and a file does not stand in for it.
func TestR12AskedNotePathIsNamed(t *testing.T) {
	for _, c := range []struct {
		path  string
		wants []string
	}{
		{skillPath, []string{
			"8. Note to keep:",
			"asks for a note, a decision or knowledge to keep",
			"has an acceptance row of its own that names it",
			"proposes it with `grid brain propose --task <task>` and lists it in the result's `knowledge`",
			"a file never stands in for the proposal",
			"passed the eight checks",
		}},
		{runJobSkillPath, []string{
			"When the Job asks for a note, a decision or knowledge to keep",
			"propose it before you post the result",
			"A file in the repo (a README, a doc) does not stand in for the proposal",
			"List each proposal in the result's `knowledge`, one string each",
			"`not met` while `knowledge` is `[]`",
		}},
		{builderPath, []string{
			"asks for a note, a decision or knowledge to keep, propose it with `grid brain propose --task <task>`",
			"list it in the result's `knowledge`",
			"a file in the repo does not replace the proposal",
		}},
	} {
		text := readText(t, c.path)
		for _, want := range c.wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", c.path, want)
			}
		}
	}
	if strings.Contains(readText(t, skillPath), "passed the seven checks") {
		t.Errorf("%s still says seven checks beside eight", skillPath)
	}
}
