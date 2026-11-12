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
	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// DeliverOptions supplies the person's outcome and the files to copy. It is
// command input; existing task records remain owned by task.Read and Fold.
type DeliverOptions struct {
	Cwd, Task, Outcome, By string
	Files                  []string
	DryRun                 bool
	// Branch and Base are the task's branch and the commit or ref it branched
	// from, written to task.md for the Code tab. Empty means the git repo holding
	// Cwd answers (see recordBranch); no answer writes no key.
	Branch, Base string
}

// Delivery describes performed or proposed actions, not a persisted task record.
type Delivery struct {
	Task      string            `json:"task"`
	DryRun    bool              `json:"dry_run"`
	Actions   []string          `json:"actions"`
	Artifacts []artifact.Record `json:"artifacts"`
	// Waiting lists the brain proposals still waiting. They never stop a delivery;
	// the orchestrator reads them to start the curator.
	Waiting []brain.WaitingProposal `json:"waiting_proposals"`
}

// Deliver checks the proof, then writes the Outcome and moves the task
// to waiting at step ship. It raises no gate: the person's answer is a Job. The task record is written only through the task writer: one
// commitWrite carries task.md and the delivery receipt, under .task.lock. The
// lock order is .task.lock, .plan.lock, the brain levels' locks, .proof.lock;
// no writer takes them the other way round.
func (s Store) Deliver(o DeliverOptions) (Delivery, error) {
	out := Delivery{DryRun: o.DryRun, Actions: []string{}, Artifacts: []artifact.Record{}, Waiting: []brain.WaitingProposal{}}
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
		waiting:   &out.Waiting,
	}
	for _, file := range o.Files {
		if !filepath.IsAbs(file) {
			file = filepath.Join(o.Cwd, file)
		}
		d.files = append(d.files, file)
	}

	// Everything before the lock is read-only, including the dry-run path.
	if out.Waiting, err = d.check(t); err != nil {
		return out, err
	}
	out.Actions = append(out.Actions, "Write Outcome in task.md")
	for _, file := range d.files {
		out.Actions = append(out.Actions, "Copy "+filepath.Base(file)+" into artifacts/")
	}
	out.Actions = append(out.Actions, "Rebuild proof/index.json", "Append delivery history")
	if o.DryRun {
		return out, nil
	}
	err = s.withWrite(o.Cwd, t.ID, func(t Task) error {
		// A plan revision cannot overtake the proof check.
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
	// waiting receives the proposals still waiting when the delivery is written.
	waiting *[]brain.WaitingProposal
}

// check refuses a delivery that cannot succeed and returns the brain proposals
// that wait. It only reads, so it serves both the dry run and the re-check under
// the locks.
func (d delivery) check(t Task) ([]brain.WaitingProposal, error) {
	data, err := os.ReadFile(filepath.Join(d.dir, "task.md"))
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err = yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return nil, err
	}
	legacy := ""
	if len(doc.Content) > 0 {
		legacy = scalar(doc.Content[0], "status")
	}
	if t.State == "closed" || legacy == "closed" || legacy == "cancelled" || legacy == "stopped" {
		return nil, errors.New("task is closed or stopped; delivery cannot reopen it")
	}
	// The state table decides which states may reach waiting; delivery from
	// paused, or from a task already at a gate, is refused like a status move,
	// except a plan gate whose approval is current (nextState).
	if _, err = d.nextState(t, data); err != nil {
		return nil, err
	}
	// Waiting proposals never stop a delivery: they are read and returned so the
	// orchestrator can start the curator. An unreadable proposal is still an error.
	waiting, err := d.brain.WaitingForTask(d.dir, t.ID)
	if err != nil {
		return nil, err
	}
	if _, err = (plan.Store{TaskDir: d.dir}).Read(); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("plan.md is missing; write the plan before delivering")
	}
	idx, err := d.proof.Build()
	if err != nil {
		return nil, err
	}
	if err = idx.Check(); err != nil {
		return nil, err
	}
	for _, file := range d.files {
		if err = d.artifacts.CheckImport(file); err != nil {
			return nil, err
		}
	}
	path, err := localdata.Safe(d.dir, "proof", "index.json")
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("proof/index.json must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return waiting, nil
}

// write runs under the task, plan and brain locks. Artifacts and the proof index
// land first; task.md and the receipt are then committed together, so the gate
// rises only when everything before it succeeded. A failure before the commit can
// leave imported artifacts or a rebuilt index, never a half-written task record.
func (d delivery) write(t Task) ([]artifact.Record, error) {
	waiting, err := d.check(t)
	if err != nil {
		return nil, err
	}
	*d.waiting = waiting
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
		outcome += tiOutcomeProofLine(item) + "\n"
	}
	branch, base := recordBranch(d.o)
	head, _ := gitx.HeadSHA(d.o.Cwd)
	if line := tiOutcomeDelivered(branch, head); line != "" {
		outcome += "\n" + line + "\n"
	}
	data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
	if err != nil {
		return imported, err
	}
	next, err := d.nextState(t, data)
	if err != nil {
		return imported, err
	}
	// Delivery asks for approval; it writes no approval and never closes the task.
	event := d.s.event(t, WriteOptions{Action: "deliver", Note: summary, Who: contract.Author{ID: d.o.By, Role: "agent"}}, next)
	updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
		setScalar(mapping, "state", next, true)
		// A legacy status field mirrors state only where the task already has one.
		setScalar(mapping, "status", next, false)
		setScalar(mapping, "step", "ship", true)
		setScalar(mapping, "next", "Review the Outcome", true)
		setScalar(mapping, "outcome", summary, true)
		setScalar(mapping, "updated", d.s.clock().UTC().Format(time.RFC3339), true)
		if branch != "" {
			setScalar(mapping, "branch", branch, true)
		}
		if base != "" {
			setScalar(mapping, "base", base, true)
		}
		if head != "" {
			setScalar(mapping, "head", head, true)
		}
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

// recordBranch is the branch and base a delivery writes to task.md. A value the
// caller gave wins; otherwise the branch is the one checked out in the git repo
// holding Cwd, and the base is where HEAD left the first of main and master that
// exists there. A missing answer is "" and writes no key.
func recordBranch(o DeliverOptions) (branch, base string) {
	branch, base = strings.TrimSpace(o.Branch), strings.TrimSpace(o.Base)
	if branch == "" {
		branch, _ = gitx.CurrentBranch(o.Cwd)
	}
	if base == "" {
		for _, def := range []string{"main", "master"} {
			if sha, ok := gitx.MergeBase(o.Cwd, "HEAD", def); ok {
				base = sha
				break
			}
		}
	}
	return branch, base
}

// tiOutcomeProofLine is one line of the Outcome's Proof list: the plan item's
// title with its id, its proof level and how many files its evidence holds. An
// item with no title keeps the id alone.
func tiOutcomeProofLine(item proof.Item) string {
	files := fmt.Sprintf("%d files", len(item.Evidence))
	if len(item.Evidence) == 1 {
		files = "1 file"
	}
	name := item.ID
	if title := strings.TrimSpace(item.Title); title != "" {
		name = title + " (" + item.ID + ")"
	}
	return fmt.Sprintf("- %s: %s, %s in proof/%s/", name, item.Level, files, item.ID)
}

// tiOutcomeDelivered is the line that names the commit a delivery was made from.
// It is "" unless both the branch and the head are known.
func tiOutcomeDelivered(branch, head string) string {
	if branch == "" || head == "" {
		return ""
	}
	if len(head) > 12 {
		head = head[:12]
	}
	return "Delivered from " + branch + " at " + head + "."
}

// nextState is the state a delivery moves t to; data is its task.md. The state
// table decides, as grid task status would, with one addition: grid approve keeps
// a task waiting at its plan gate (Approve in gates.go), so a waiting task whose
// approval receipt holds the plan's current hash moves on to waiting at step
// ship. Only the hash is compared, as this move was decided; the digest also
// binds step metadata (plan.Digest). A task already at step ship, a task with no
// approval and an approval of an older plan get the state table's refusal.
func (d delivery) nextState(t Task, data []byte) (string, error) {
	next, err := NextState(t.State, "status", "waiting")
	if err == nil || t.State != "waiting" {
		return next, err
	}
	var doc yaml.Node
	if yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc) != nil || len(doc.Content) == 0 || scalar(doc.Content[0], "step") == "ship" {
		return next, err
	}
	kept := keptApproval(t)
	if kept == nil {
		return next, err
	}
	// Read takes no lock: write already holds .plan.lock (Deliver, WithHash).
	current, readErr := plan.Store{TaskDir: d.dir}.Read()
	if readErr != nil || kept.PlanHash != plan.Hash(current) {
		return next, err
	}
	return "waiting", nil
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
