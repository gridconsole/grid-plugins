// Package routine owns the executable schema of the routines config topic.
package routine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

type Entry struct {
	LimitPolicy `yaml:",inline"`
	Limits      Limits `json:"limits" yaml:"-"`
	Name        string `json:"name" yaml:"name"`
	Every       string `json:"every" yaml:"every"`
	Run         string `json:"run" yaml:"run"`
	// Workflow, when set, takes the place of Run: each run starts one new task
	// named after the routine with this workflow, and LastTask records its id.
	Workflow string    `json:"workflow,omitempty" yaml:"workflow,omitempty"`
	Machine  string    `json:"machine" yaml:"machine"`
	LastRun  time.Time `json:"last_run,omitzero" yaml:"last_run,omitempty"`
	Outcome  string    `json:"outcome,omitempty" yaml:"outcome,omitempty"`
	LastTask string    `json:"last_task,omitempty" yaml:"last_task,omitempty"`
	// Scheduled and Next are read from this machine's timer ownership record when
	// an entry is listed or run; they are never stored in the routine's config.
	Scheduled bool       `json:"scheduled" yaml:"-"`
	Next      *time.Time `json:"next" yaml:"-"`
	Body      string     `json:"-" yaml:"-"`

	workflowPolicy LimitPolicy // snapshot read with the entry; never persisted
}

type Store struct {
	GridHome string
	Project  string // empty selects home; no implicit project fallback
	Now      func() time.Time
}

var ErrScheduleChanged = errors.New("routine changed since it was scheduled")

func (s Store) brain() brain.Store { return brain.Store{GridHome: s.GridHome, Project: s.Project} }
func (s Store) level() string {
	if s.Project != "" {
		return "project"
	}
	return "home"
}
func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Fingerprint identifies the command, interval and effective execution limits.
// Legacy entries with default workflow policy retain exactly run + NUL + every.
// Otherwise append NUL + "limits-v1" + NUL, then timeout nanoseconds, max turns
// and budget (canonical decimals), separated by NUL, with no trailing separator.
// A routine with a workflow then appends NUL + "workflow-v1" + NUL + workflow;
// an entry without one keeps the fingerprint it had before workflows existed.
func (e Entry) Fingerprint() (string, error) {
	limits, err := e.EffectiveLimits()
	if err != nil {
		return "", err
	}
	defaults, err := (LimitPolicy{}).EffectiveLimits()
	if err != nil {
		return "", err
	}
	workflow, err := e.workflowPolicy.EffectiveLimits()
	if err != nil {
		return "", err
	}
	input := e.Run + "\x00" + e.Every
	if e.LimitPolicy.explicit() || limits.fingerprint() != defaults.fingerprint() || workflow.fingerprint() != defaults.fingerprint() {
		input += "\x00limits-v1\x00" + limits.fingerprint()
	}
	if e.Workflow != "" {
		input += "\x00workflow-v1\x00" + e.Workflow
	}
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:]), nil
}

// checkScheduled refuses a timer run whose command, interval or limits differ
// from what `grid routine schedule` recorded, so a synced edit of a project
// brain cannot change what an installed timer runs.
func (s Store) checkScheduled(e Entry) error {
	p := TimerPlan{Label: timerLabel(s.GridHome, s.Project, e.Name), Project: s.Project, Name: e.Name}
	state, err := Scheduler{GridHome: s.GridHome}.readState(p)
	if err != nil {
		return err
	}
	hash, err := e.Fingerprint()
	if err != nil {
		return err
	}
	if state.Label == "" || state.Hash == hash {
		return nil
	}
	return fmt.Errorf("%w: %s; run grid routine schedule again to approve it", ErrScheduleChanged, e.Name)
}

// annotate fills Scheduled and Next. A routine is scheduled here only when this
// machine's ownership record says installed and holds the entry's fingerprint, the
// same comparison checkScheduled makes for a timer run; a routine of another
// machine, a missing or unreadable record and a changed routine all read false.
func (s Store) annotate(e Entry) Entry {
	e.Scheduled, e.Next = false, nil
	if e.Machine != gridhome.Machine(s.GridHome) {
		return e
	}
	p := TimerPlan{Label: timerLabel(s.GridHome, s.Project, e.Name), Project: s.Project, Name: e.Name}
	state, err := Scheduler{GridHome: s.GridHome}.readState(p)
	if err != nil || state.Label == "" || !state.Installed {
		return e
	}
	if hash, err := e.Fingerprint(); err != nil || state.Hash != hash {
		return e
	}
	e.Scheduled = true
	if interval, err := ParseInterval(e.Every); err == nil {
		if next, ok := interval.Next(s.now(), e.LastRun); ok {
			e.Next = &next
		}
	}
	return e
}

func (e Entry) Validate() error {
	if _, err := e.EffectiveLimits(); err != nil {
		return err
	}
	if !brain.ValidName(e.Name) || e.Name == "TOPIC" {
		return fmt.Errorf("routine name must be a plain name")
	}
	if !gridhome.ValidID(e.Machine) {
		return fmt.Errorf("routine needs an owner machine ULID")
	}
	if _, err := ParseInterval(e.Every); err != nil {
		return err
	}
	if err := e.validCommand(); err != nil {
		return err
	}
	switch e.Outcome {
	case "", "running", "success", "failed":
	default:
		return fmt.Errorf("invalid routine outcome")
	}
	return nil
}

// validCommand accepts a routine with a run command or a workflow, never both.
// A workflow is a plain name, as the brain names workflow files.
func (e Entry) validCommand() error {
	if e.Workflow == "" {
		_, err := ParseRun(e.Run)
		return err
	}
	if e.Run != "" {
		return fmt.Errorf("a routine has run or workflow, not both")
	}
	if !brain.ValidName(e.Workflow) {
		return fmt.Errorf("workflow must be a plain name")
	}
	return nil
}

func decode(name string, b []byte) (Entry, error) {
	var e Entry
	if b == nil {
		return e, fmt.Errorf("routine %q does not exist", name)
	}
	body, err := brain.DecodeConfig(b, &e)
	e.Body = body
	if err != nil {
		return e, err
	}
	if e.Name != name {
		return e, fmt.Errorf("routine name does not match its file")
	}
	if err := e.Validate(); err != nil {
		return e, err
	}
	e.Limits, err = e.EffectiveLimits()
	return e, err
}
func (s Store) Get(name string) (Entry, error) {
	b, err := s.brain().ReadConfig(s.level(), "routines", name)
	if err != nil {
		return Entry{}, err
	}
	e, err := decode(name, b)
	if err != nil {
		return e, err
	}
	return s.resolveLimits(e)
}
func (s Store) List() ([]Entry, error) {
	entries := []Entry{}
	names, err := s.brain().ConfigNames(s.level(), "routines")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		e, err := s.Get(name)
		if err != nil {
			var policyErr *WorkflowPolicyError
			if errors.As(err, &policyErr) {
				return nil, err
			}
			var invalid *LimitError
			if errors.As(err, &invalid) {
				return nil, &LimitError{Key: invalid.Key, Routine: name}
			}
			return nil, fmt.Errorf("routine %s: %w", name, err)
		}
		entries = append(entries, s.annotate(e))
	}
	return entries, nil
}
func (s Store) lock(name string) (func(), error) {
	if !brain.ValidName(name) || (s.Project != "" && !brain.ValidName(s.Project)) {
		return nil, fmt.Errorf("invalid routine or project name")
	}
	// Name is hashed with the scope so one home's locks cannot collide with a project.
	return localdata.Lock(s.GridHome, ".routine-"+identity(s.GridHome, s.Project, name)+".lock")
}
func (s Store) Add(name, every, run string, policy ...LimitPolicy) (Entry, error) {
	if len(policy) > 1 {
		return Entry{Name: name, Every: strings.TrimSpace(every), Run: strings.TrimSpace(run)}, fmt.Errorf("expected at most one routine limit policy")
	}
	var o AddOptions
	if len(policy) == 1 {
		o.Policy = policy[0]
	}
	return s.AddWith(name, every, run, o)
}

// AddOptions holds what Add's short form leaves out.
type AddOptions struct {
	// Workflow replaces run: each run starts a new task with this workflow.
	Workflow string
	// Replace rewrites an existing routine of the same name, keeping its
	// last_run, outcome, last_task and body. Its fingerprint changes with what
	// was edited, so an installed timer refuses it until it is scheduled again.
	Replace bool
	Policy  LimitPolicy
}

// AddWith adds a routine with a run command or a workflow.
func (s Store) AddWith(name, every, run string, o AddOptions) (Entry, error) {
	e := Entry{Name: name, Every: strings.TrimSpace(every), Run: strings.TrimSpace(run), Workflow: strings.TrimSpace(o.Workflow), LimitPolicy: o.Policy}
	var err error
	e, err = s.resolveLimits(e)
	if err != nil {
		return e, err
	}
	// Validate the command and interval before creating identity or config files.
	if !brain.ValidName(name) {
		return e, fmt.Errorf("routine name must be a plain name")
	}
	if _, err := ParseInterval(e.Every); err != nil {
		return e, err
	}
	if err := e.validCommand(); err != nil {
		return e, err
	}
	machine, err := localdata.Machine(s.GridHome)
	if err != nil {
		return e, err
	}
	e.Machine = machine
	unlock, err := s.lock(name)
	if err != nil {
		return e, err
	}
	defer unlock()
	err = s.brain().EditConfig(s.level(), "routines", name, func(old []byte) ([]byte, error) {
		body := ""
		if old != nil {
			if !o.Replace {
				return nil, fmt.Errorf("routine %q already exists", name)
			}
			// What the old file holds is kept only where it reads; an unreadable
			// routine is rewritten whole, which is what an edit of it is for.
			var prev Entry
			if text, err := brain.DecodeConfig(old, &prev); err == nil {
				e.LastRun, e.Outcome, e.LastTask, body = prev.LastRun, prev.Outcome, prev.LastTask, text
			}
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		return brain.EncodeConfig(e, body)
	})
	return e, err
}

// Remove is idempotent. Installed schedules must be removed by the caller first.
func (s Store) Remove(name string) error {
	unlock, err := s.lock(name)
	if err != nil {
		return err
	}
	defer unlock()
	return s.remove(name)
}

func (s Store) remove(name string) error {
	return s.brain().EditConfig(s.level(), "routines", name, func([]byte) ([]byte, error) { return nil, nil })
}
func (s Store) save(e Entry) error {
	return s.brain().EditConfig(s.level(), "routines", e.Name, func(old []byte) ([]byte, error) {
		if old == nil {
			return nil, fmt.Errorf("routine disappeared during run")
		}
		return brain.EncodeConfig(e, e.Body)
	})
}

type Executor interface {
	Execute(context.Context, Store, Entry) error
}

// TaskExecutor is an Executor that also starts a workflow routine's task and
// returns its id, which Execute records as last_task. A runner without it runs a
// workflow routine through Execute and records no task.
type TaskExecutor interface {
	ExecuteTask(context.Context, Store, Entry) (taskID string, err error)
}
type FailureSignal func(Entry, error) error

type RunResult struct {
	Entry
	Skipped bool `json:"skipped"`
}

// Run executes manual requests once. OS invocations set due to apply the interval
// guard. Both paths hold the same per-entry lock across execution and recording.
func (s Store) Execute(ctx context.Context, name string, due bool, runner Executor, onFailure FailureSignal) (RunResult, error) {
	unlock, err := s.lock(name)
	if err != nil {
		return RunResult{}, err
	}
	defer unlock()
	e, err := s.Get(name)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{Entry: s.annotate(e)}
	if e.Machine != gridhome.Machine(s.GridHome) {
		return result, fmt.Errorf("routine belongs to another machine")
	}
	if due {
		if err := s.checkScheduled(e); err != nil {
			// Keep last_run unchanged so repeated refusals share the failure
			// signal's SourceID and produce one Personal note.
			if onFailure != nil {
				return result, errors.Join(err, onFailure(e, err))
			}
			return result, err
		}
	}
	schedule, _ := ParseInterval(e.Every)
	if due && !schedule.Due(s.now(), e.LastRun) {
		result.Skipped = true
		return result, nil
	}
	if runner == nil {
		return result, fmt.Errorf("routine executor is required")
	}
	e.LastRun, e.Outcome = s.now(), "running"
	if err := s.save(e); err != nil {
		return result, err
	}
	var runErr error
	var taskID string
	if tasks, ok := runner.(TaskExecutor); ok && e.Workflow != "" {
		taskID, runErr = tasks.ExecuteTask(ctx, s, e)
	} else {
		runErr = runner.Execute(ctx, s, e)
	}
	e.Outcome = "success"
	if runErr != nil {
		// A failed run keeps the task of the last run that started one.
		e.Outcome = "failed"
	} else if taskID != "" {
		e.LastTask = taskID
	}
	result.Entry = s.annotate(e)
	saveErr := s.save(e)
	var signalErr error
	if runErr != nil && onFailure != nil {
		signalErr = onFailure(e, runErr)
	}
	return result, errors.Join(runErr, saveErr, signalErr)
}
