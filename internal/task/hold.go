package task

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// Agent text reaches history inside a JSON event ("- {..."), so only the dated
// prose line Grid itself writes, from the line start, reads as an acceptance.
var acceptedLine = regexp.MustCompile(`(?m)^- \d{4}-\d\d-\d\dT\d\d:\d\d · plan accepted · `)

// StepLookup resolves a task's steps from the task.md text it is given: the
// file, or an edit the guard is deciding. It never reads task.md again.
type StepLookup func(text string) ([]taskfile.WorkflowStep, error)

// StepsFor resolves the steps of the task in dir through its workflow: the
// inline list, the named template (the project's copy first) or the workspace
// default. The project is the folder above tasks/, as the guard places a task.
// The guard and Deliver both build their lookup here, so they resolve alike.
func StepsFor(r where.Resolver, dir string) StepLookup {
	project := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	return func(text string) ([]taskfile.WorkflowStep, error) {
		return workflow.TaskSteps(r, project, text)
	}
}

// InlineSteps reads only an inline workflow list. It is the lookup for callers
// with no Grid home to resolve a named workflow from (the session hooks).
func InlineSteps(text string) ([]taskfile.WorkflowStep, error) {
	return taskfile.WorkflowSteps(text), nil
}

// PlanStage is PlanStageWith on the inline list alone, for the hook helpers.
func PlanStage(text, history string) string {
	return PlanStageWith(InlineSteps, text, history)
}

// PlanStageWith places the task's step against its plan approval gate: "" (no
// gate or accepted), "before", "gate", "past" or "unreadable". It is the one
// reading of a task.md header and history that the guard and the writers share;
// steps the lookup cannot resolve read as "unreadable", which holds.
func PlanStageWith(lookup StepLookup, text, history string) string {
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
	steps, err := lookup(text)
	if err != nil || len(steps) == 0 {
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

// PlanHeldWith is the one answer to "does this task's plan still wait for a
// person?". An approval from `grid approve` releases it only while it still
// matches plan.md (Approved); the old "plan accepted" history line, which tasks
// migrated from the old plugin carry, releases it through PlanStageWith. The
// guard and Deliver both ask it with StepsFor, so neither can release a plan
// the other holds.
func PlanHeldWith(lookup StepLookup, dir, text, history string) bool {
	// A JSON receipt takes precedence over legacy history/step advancement:
	// stale full digests must not be released by an old prose acceptance.
	if _, err := os.Lstat(filepath.Join(dir, approvalFile)); !os.IsNotExist(err) {
		return approvalRequired(lookup(text)) && !Approved(dir)
	}
	return HeldStage(PlanStageWith(lookup, text, history))
}

// approvalRequired is taskfile.PlanApprovalRequired on resolved steps: steps
// that cannot be resolved, a plan gate, or a kind other than run, work or
// approve need a person's approval.
func approvalRequired(steps []taskfile.WorkflowStep, err error) bool {
	if err != nil || len(steps) == 0 || taskfile.PlanApprovalIndex(steps) >= 0 {
		return true
	}
	for _, step := range steps {
		switch step.Kind {
		case "run", "work", "approve":
		default:
			return true
		}
	}
	return false
}
