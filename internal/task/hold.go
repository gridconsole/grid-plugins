package task

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// Agent text reaches history inside a JSON event ("- {..."), so only the dated
// prose line Grid itself writes, from the line start, reads as an acceptance.
var acceptedLine = regexp.MustCompile(`(?m)^- \d{4}-\d\d-\d\dT\d\d:\d\d · plan accepted · `)

// PlanStage places the task's step against its plan approval gate: "" (no gate
// or accepted), "before", "gate", "past" or "unreadable". It is the one reading
// of a task.md header and history that the guard and the writers share.
func PlanStage(text, history string) string {
	if acceptedLine.MatchString(history) {
		return ""
	}
	if text == "" {
		return "before"
	}
	fm := taskfile.ParseFrontmatter(text)
	if fm == nil || fm.Odd {
		return "unreadable"
	}
	steps := taskfile.WorkflowSteps(text)
	if len(steps) == 0 {
		return "unreadable"
	}
	current, ok := fm.Data["step"].(string)
	if !ok || current == "" {
		return "unreadable"
	}
	at, gate := -1, taskfile.PlanApprovalIndex(steps)
	for i, item := range steps {
		if item.Step == current {
			at = i
		}
	}
	if gate < 0 {
		return ""
	}
	if at < 0 || at > gate {
		return "past"
	}
	if at == gate {
		return "gate"
	}
	return "before"
}

// HeldStage reports whether a PlanStage still waits for a person.
func HeldStage(stage string) bool {
	return stage == "before" || stage == "gate" || stage == "unreadable"
}

// PlanHeld is the one answer to "does this task's plan still wait for a person?".
// An approval from `grid approve` releases it only while it still matches
// plan.md (Approved); the old "plan accepted" history line, which tasks
// migrated from the old plugin carry, releases it through PlanStage. The guard
// and Deliver both ask it, so neither can release a plan the other holds.
func PlanHeld(dir, text, history string) bool {
	// A JSON receipt takes precedence over legacy history/step advancement:
	// stale full digests must not be released by an old prose acceptance.
	if _, err := os.Lstat(filepath.Join(dir, approvalFile)); !os.IsNotExist(err) {
		return taskfile.PlanApprovalRequired(text) && !Approved(dir)
	}
	return HeldStage(PlanStage(text, history))
}
