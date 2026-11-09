package task

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

// ErrProjectMismatch is the lookup error of a task whose task.md names another
// project than the folder it sits in. Nothing legitimate makes one: a person's
// receipt does not release it, because the receipt was given to a task in its
// own project.
var ErrProjectMismatch = errors.New("task.md names another project than its folder")

// ErrWorkflowChanged (code workflow_changed) is the lookup error of a task
// whose steps now resolve to others than the ones task.md's workflow_digest was
// made from: a project folder swapped or rebuilt under it, or a workflows.md a
// person changed. It holds like ErrProjectMismatch, and no receipt releases it.
var ErrWorkflowChanged = errors.New("workflow_changed: the task's workflow resolves to other steps than the ones it was created with")

// WorkflowDigest is the hex sha256 of the canonical resolved steps: each step's
// name, kind and whether it is the plan gate, one quoted line per step, in order.
func WorkflowDigest(steps []taskfile.WorkflowStep) string {
	gate := taskfile.PlanApprovalIndex(steps)
	var b strings.Builder
	for i, step := range steps {
		b.WriteString(strconv.Quote(step.Step) + " " + strconv.Quote(step.Kind) + " " + strconv.FormatBool(i == gate) + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// StepsFor resolves the steps of the task in dir through its workflow: the
// inline list, the named template (the project's copy first) or the workspace
// default. The project is the folder above tasks/, as the guard places a task,
// and task.md must name that same project: a folder moved under another
// project would otherwise read that project's same-name template, one that may
// have no plan gate. A mismatch is an error, which holds. An inline list needs
// no project, so it reads as it is. A folder name proves nothing once folders
// are renamed, so the steps must also match the workflow_digest the task was
// created with (ErrWorkflowChanged); a task made before that field keeps the
// folder check alone. The guard and Deliver both build their lookup here, so
// they resolve alike.
func StepsFor(r where.Resolver, dir string) StepLookup {
	project := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	return func(text string) ([]taskfile.WorkflowStep, error) {
		if err := inFolderProject(text, project); err != nil {
			return nil, err
		}
		steps, err := workflow.TaskSteps(r, project, text)
		if err != nil {
			return nil, err
		}
		if err := sameDigest(text, steps); err != nil {
			return nil, err
		}
		return steps, nil
	}
}

// sameDigest checks resolved steps against task.md's workflow_digest. A value
// that is not a string, or any other digest, is a change.
func sameDigest(text string, steps []taskfile.WorkflowStep) error {
	fm := taskfile.ParseFrontmatter(text)
	if fm == nil || fm.Odd {
		return errors.New("task.md front matter cannot be read")
	}
	raw, has := fm.Data["workflow_digest"]
	if !has {
		return nil
	}
	want, _ := raw.(string)
	if got := WorkflowDigest(steps); strings.TrimSpace(want) != got {
		return fmt.Errorf("%w (task.md has %q, the steps now give %q)", ErrWorkflowChanged, want, got)
	}
	return nil
}

// inFolderProject checks that a task whose workflow is told through its project
// names the project its folder sits in. A task of Personal may leave its
// project out, as it did before the check.
func inFolderProject(text, folder string) error {
	fm := taskfile.ParseFrontmatter(text)
	if fm == nil || fm.Odd {
		return nil // TaskSteps names this fault
	}
	if _, inline := fm.Data["workflow"].([]any); inline {
		return nil
	}
	declared, _ := fm.Data["project"].(string)
	declared = strings.TrimSpace(declared)
	if declared == folder || (declared == "" && folder == where.Personal) {
		return nil
	}
	return fmt.Errorf("%w: task.md has project %q but its folder is in project %q", ErrProjectMismatch, declared, folder)
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
		steps, err := lookup(text)
		if errors.Is(err, ErrProjectMismatch) || errors.Is(err, ErrWorkflowChanged) {
			return true
		}
		return approvalRequired(steps, err) && !Approved(dir)
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
