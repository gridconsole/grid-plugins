package task

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/workflow"
	"go.yaml.in/yaml/v4"
)

// previewSpec is the default of the workflow key preview_requires_spec (true):
// the preview is raised on this file until workflow/parse.go reads the key.
const previewSpec = "spec.md"

// PreviewOptions is command input for raising the preview gate.
type PreviewOptions struct {
	Cwd, Task, Spec, By string
	DryRun              bool
}

// Preview describes performed or proposed actions, not a persisted task record.
type Preview struct {
	Task    string   `json:"task"`
	DryRun  bool     `json:"dry_run"`
	Spec    string   `json:"spec"`
	Actions []string `json:"actions"`
}

// RaisePreview checks the workflow, the plan approval and the spec, then raises
// the preview gate. Like Deliver it writes task.md and one receipt in a single
// commitWrite under .task.lock, then .plan.lock; it writes no approval.
func (s Store) RaisePreview(o PreviewOptions) (Preview, error) {
	out := Preview{DryRun: o.DryRun, Actions: []string{}}
	if err := checkTaskText("by", o.By, false); err != nil {
		return out, err
	}
	if strings.TrimSpace(o.By) == "" || strings.ContainsAny(o.By, "\r\n\x00") {
		return out, errors.New("by must name the author on one line")
	}
	if err := checkTaskText("spec", o.Spec, false); err != nil {
		return out, err
	}
	t, err := s.Show(o.Cwd, o.Task)
	if err != nil {
		return out, err
	}
	out.Task = t.ID
	dir, err := filepath.EvalSymlinks(t.Path)
	if err != nil {
		return out, err
	}
	p := preview{s: s, o: o, dir: dir}
	// Everything before the lock is read-only, including the dry-run path.
	spec, _, err := p.check(t)
	if err != nil {
		return out, err
	}
	out.Spec = spec
	out.Actions = append(out.Actions, "Append preview history", "Raise preview gate")
	if o.DryRun {
		return out, nil
	}
	err = s.withWrite(o.Cwd, t.ID, func(t Task) error {
		// A plan revision cannot overtake the approval check and the raised gate.
		err := plan.Store{TaskDir: t.Path}.WithHash(func(string) error { return p.write(t) })
		if errors.Is(err, plan.ErrNoPlan) {
			return errors.New("plan.md is missing; write and approve the plan before the preview")
		}
		return err
	})
	return out, err
}

type preview struct {
	s   Store
	o   PreviewOptions
	dir string
}

// check refuses a raise that cannot succeed and returns the spec's path inside
// the task and the history note. It only reads, so it serves both the dry run
// and the re-check under the locks, where task.md is the text the lock holds.
func (p preview) check(t Task) (string, string, error) {
	data, err := os.ReadFile(filepath.Join(p.dir, "task.md"))
	if err != nil {
		return "", "", err
	}
	var doc yaml.Node
	if err = yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return "", "", err
	}
	legacy := ""
	if len(doc.Content) > 0 {
		legacy = scalar(doc.Content[0], "status")
	}
	if t.State == "closed" || legacy == "closed" || legacy == "cancelled" || legacy == "stopped" {
		return "", "", errors.New("task is closed or stopped; a preview cannot reopen it")
	}
	// From paused, or from a task already at a gate, refused like a status move.
	if _, err = NextState(t.State, "status", "waiting"); err != nil {
		return "", "", err
	}
	steps, err := workflow.TaskSteps(p.s.Resolver, t.Project, string(data))
	if err != nil {
		return "", "", fmt.Errorf("Preview blocked: cannot resolve the task's workflow: %v", err)
	}
	at := -1
	for i, step := range steps {
		if step.Step == "preview" && step.Kind == "approve" {
			at = i
			break
		}
	}
	if at < 0 {
		return "", "", errors.New("Preview blocked: the task's workflow has no preview approve step")
	}
	if gate := taskfile.PlanApprovalIndex(steps); gate < 0 || gate >= at {
		return "", "", errors.New("Preview blocked: the workflow has no plan approval before preview")
	}
	if !Approved(p.dir) {
		return "", "", errors.New("Preview blocked: the plan is not approved; approve plan.md before the preview")
	}
	return p.spec()
}

// spec checks the spec file: inside the task folder, no symlink on the way, a
// regular file with a nonblank line. The note is its first heading, else its name.
func (p preview) spec() (string, string, error) {
	name := p.o.Spec
	if name == "" {
		name = previewSpec
	}
	rel := filepath.Clean(name)
	if filepath.IsAbs(name) {
		var err error
		if rel, err = filepath.Rel(p.dir, name); err != nil {
			rel = ".."
		}
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("Preview blocked: %s is not a file inside the task folder", name)
	}
	path, err := localdata.Safe(p.dir, rel)
	if err != nil {
		return "", "", fmt.Errorf("Preview blocked: %s: %v", rel, err)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("Preview blocked: %s is missing; write the spec before the preview", rel)
	}
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("Preview blocked: %s must be a regular file", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", "", fmt.Errorf("Preview blocked: %s has no nonblank line", rel)
	}
	note := filepath.ToSlash(rel)
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if heading != "" && checkTaskText("note", heading, false) == nil {
				note = heading
			}
			break
		}
	}
	return filepath.ToSlash(rel), note, nil
}

// write runs under the task and plan locks: it re-checks, then commits task.md
// and the receipt together, so the gate rises only on a passing check.
func (p preview) write(t Task) error {
	_, note, err := p.check(t)
	if err != nil {
		return err
	}
	next, err := NextState(t.State, "status", "waiting")
	if err != nil {
		return err
	}
	event := p.s.event(t, WriteOptions{Action: "preview", Note: note, Who: contract.Author{ID: p.o.By, Role: "agent"}}, next)
	data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
	if err != nil {
		return err
	}
	updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
		setScalar(mapping, "state", next, true)
		// A legacy status field mirrors state only where the task already has one.
		setScalar(mapping, "status", next, false)
		setScalar(mapping, "step", "preview", true)
		setScalar(mapping, "gate", "preview", true)
		setScalar(mapping, "next", "Review spec.md and approve the preview", true)
		setScalar(mapping, "updated", p.s.clock().UTC().Format(time.RFC3339), true)
		return nil
	}, nil)
	if err != nil {
		return err
	}
	if _, err = Fold(t.Path, string(updated), t.Intent); err != nil {
		return err
	}
	return commitWrite(t.Path, event, map[string][]byte{"task.md": updated})
}
