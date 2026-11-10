package task

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/artifact"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// ErrPlanNotApproved identifies a delivery refused because the task's plan has
// no valid approval yet.
var ErrPlanNotApproved = errors.New("plan not approved")

// DeliverOptions supplies the person's outcome and the files to copy. It is
// command input; existing task records remain owned by task.Read and Fold.
type DeliverOptions struct {
	Cwd, Task, Outcome, By string
	Files                  []string
	DryRun                 bool
}

// Delivery describes performed or proposed actions, not a persisted task record.
type Delivery struct {
	Task      string            `json:"task"`
	DryRun    bool              `json:"dry_run"`
	Actions   []string          `json:"actions"`
	Artifacts []artifact.Record `json:"artifacts"`
}

// Deliver checks the keeper and proof gates, then writes the Outcome and raises
// the ship gate. The task record is written only through the task writer: one
// commitWrite carries task.md and the delivery receipt, under .task.lock. The
// lock order is .task.lock, .plan.lock, the brain levels' locks, .proof.lock;
// no writer takes them the other way round.
func (s Store) Deliver(o DeliverOptions) (Delivery, error) {
	out := Delivery{DryRun: o.DryRun, Actions: []string{}, Artifacts: []artifact.Record{}}
	if err := checkTaskText("outcome", o.Outcome, true); err != nil {
		return out, err
	}
	if err := checkTaskText("by", o.By, false); err != nil {
		return out, err
	}
	if strings.TrimSpace(o.Outcome) == "" || strings.ContainsRune(o.Outcome, 0) {
		return out, errors.New("supply a nonempty Outcome with --outcome")
	}
	if strings.TrimSpace(o.By) == "" || strings.ContainsAny(o.By, "\r\n\x00") {
		return out, errors.New("by must name the author on one line")
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
	project := t.Project
	if project == where.Personal {
		project = ""
	}
	res, err := s.Resolver.Resolve(where.Query{Cwd: dir})
	if err != nil {
		return out, err
	}
	d := delivery{
		s: s, o: o, dir: dir,
		brain:     brain.Store{GridHome: s.Resolver.GridHome, Project: project},
		proof:     proof.Store{TaskDir: dir},
		artifacts: artifact.Store{TaskPath: dir, ProjectFolder: res.Folder, Now: s.Now},
	}
	for _, file := range o.Files {
		if !filepath.IsAbs(file) {
			file = filepath.Join(o.Cwd, file)
		}
		d.files = append(d.files, file)
	}

	// Everything before the lock is read-only, including the dry-run path.
	if err = d.check(t); err != nil {
		return out, err
	}
	out.Actions = append(out.Actions, "Write Outcome in task.md")
	for _, file := range d.files {
		out.Actions = append(out.Actions, "Copy "+filepath.Base(file)+" into artifacts/")
	}
	out.Actions = append(out.Actions, "Rebuild proof/index.json", "Append delivery history", "Raise ship gate")
	if o.DryRun {
		return out, nil
	}
	err = s.withWrite(o.Cwd, t.ID, func(t Task) error {
		// A plan revision cannot overtake the proof check and the raised gate.
		err := plan.Store{TaskDir: t.Path}.WithHash(func(string) error {
			return d.brain.WithKeeperGate(dir, func() error {
				var err error
				out.Artifacts, err = d.write(t)
				return err
			})
		})
		if errors.Is(err, plan.ErrNoPlan) {
			return errors.New("plan.md is missing; write the plan before delivering")
		}
		return err
	})
	return out, err
}

type delivery struct {
	s         Store
	o         DeliverOptions
	dir       string
	files     []string
	brain     brain.Store
	proof     proof.Store
	artifacts artifact.Store
}

// check refuses a delivery that cannot succeed. It only reads, so it serves both
// the dry run and the re-check under the locks.
func (d delivery) check(t Task) error {
	data, err := os.ReadFile(filepath.Join(d.dir, "task.md"))
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err = yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return err
	}
	legacy := ""
	if len(doc.Content) > 0 {
		legacy = scalar(doc.Content[0], "status")
	}
	if t.State == "closed" || legacy == "closed" || legacy == "cancelled" || legacy == "stopped" {
		return errors.New("task is closed or stopped; delivery cannot reopen it")
	}
	// The state table decides which states may reach waiting; delivery from
	// paused, or from a task already at a gate, is refused like a status move.
	if _, err = NextState(t.State, "status", "waiting"); err != nil {
		return err
	}
	waiting, err := d.brain.WaitingForTask(d.dir, t.ID)
	if err != nil {
		return err
	}
	if len(waiting) > 0 {
		return fmt.Errorf("Deliver blocked: %d brain proposals wait. Run: grid brain keep", len(waiting))
	}
	if _, err = (plan.Store{TaskDir: d.dir}).Read(); errors.Is(err, os.ErrNotExist) {
		return errors.New("plan.md is missing; write the plan before delivering")
	}
	idx, err := d.proof.Build()
	if err != nil {
		return err
	}
	if err = idx.Check(); err != nil {
		return err
	}
	for _, file := range d.files {
		if err = d.artifacts.CheckImport(file); err != nil {
			return err
		}
	}
	path, err := localdata.Safe(d.dir, "proof", "index.json")
	if err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("proof/index.json must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Delivery writes step: ship, which would move a task whose plan still waits
	// for a person past the plan gate. It asks the guard's own question, last,
	// after the faults the agent can mend itself.
	history, err := readOptional(filepath.Join(d.dir, "history.md"))
	if err != nil {
		return err
	}
	lookup := StepsFor(d.s.Resolver, d.dir)
	if PlanHeldWith(lookup, d.dir, string(data), string(history)) {
		return fmt.Errorf("%w: %s waits for a person to approve its plan, and delivery would move it past that gate", ErrPlanNotApproved, t.ID)
	}
	return nil
}

// write runs under the task, plan and brain locks. Artifacts and the proof index
// land first; task.md and the receipt are then committed together, so the gate
// rises only when everything before it succeeded. A failure before the commit can
// leave imported artifacts or a rebuilt index, never a half-written task record.
func (d delivery) write(t Task) ([]artifact.Record, error) {
	if err := d.check(t); err != nil {
		return nil, err
	}
	var imported []artifact.Record
	for _, file := range d.files {
		record, err := d.artifacts.Import(file, "")
		if err != nil {
			return imported, err
		}
		imported = append(imported, record)
	}
	idx, err := d.proof.Rebuild()
	if err != nil {
		return imported, err
	}
	if err = idx.Check(); err != nil {
		return imported, err
	}
	outcome := strings.TrimSpace(d.o.Outcome)
	summary := strings.TrimSpace(strings.SplitN(outcome, "\n", 2)[0])
	if len(imported) > 0 {
		outcome += "\n\n### Deliverables\n"
		for _, r := range imported {
			outcome += "\n- artifacts/" + r.File
		}
	}
	outcome += "\n\n### Proof\n\n"
	for _, item := range idx.Items {
		outcome += fmt.Sprintf("- %s: %s proof saved in proof/%s/\n", item.ID, item.Level, item.ID)
	}
	next, err := NextState(t.State, "status", "waiting")
	if err != nil {
		return imported, err
	}
	// Delivery asks for approval; it writes no approval and never closes the task.
	event := d.s.event(t, WriteOptions{Action: "deliver", Note: summary, Who: contract.Author{ID: d.o.By, Role: "agent"}}, next)
	data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
	if err != nil {
		return imported, err
	}
	updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
		setScalar(mapping, "state", next, true)
		// A legacy status field mirrors state only where the task already has one.
		setScalar(mapping, "status", next, false)
		setScalar(mapping, "step", "ship", true)
		setScalar(mapping, "gate", "ship", true)
		setScalar(mapping, "next", "Review the Outcome and approve ship", true)
		setScalar(mapping, "outcome", summary, true)
		setScalar(mapping, "updated", d.s.clock().UTC().Format(time.RFC3339), true)
		return nil
	}, func(body string) string { return replaceOutcome(body, outcome) })
	if err != nil {
		return imported, err
	}
	if _, err = Fold(t.Path, string(updated), t.Intent); err != nil {
		return imported, err
	}
	return imported, commitWrite(t.Path, event, map[string][]byte{"task.md": updated})
}

// Replace only Outcome, preserving later peer sections and all earlier notes.
func replaceOutcome(body, outcome string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	start, end := -1, len(lines)
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.TrimSpace(line) == "## Outcome" && start < 0 {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	section := "## Outcome\n\n" + strings.TrimSpace(outcome) + "\n"
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + section
	}
	before := strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
	after := strings.Join(lines[end:], "\n")
	if before != "" {
		before += "\n\n"
	}
	if after != "" {
		section += "\n"
	}
	return before + section + after
}
