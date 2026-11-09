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
	Limits      Limits    `json:"limits" yaml:"-"`
	Name        string    `json:"name" yaml:"name"`
	Every       string    `json:"every" yaml:"every"`
	Run         string    `json:"run" yaml:"run"`
	Machine     string    `json:"machine" yaml:"machine"`
	LastRun     time.Time `json:"last_run,omitzero" yaml:"last_run,omitempty"`
	Outcome     string    `json:"outcome,omitempty" yaml:"outcome,omitempty"`
	Body        string    `json:"-" yaml:"-"`
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

// Fingerprint identifies what a schedule approved: the command and the interval.
// The run value cannot hold NUL, so the pair is unambiguous.
func (e Entry) Fingerprint() string {
	sum := sha256.Sum256([]byte(e.Run + "\x00" + e.Every))
	return hex.EncodeToString(sum[:])
}

// checkScheduled refuses a timer run of a routine whose run or every differs
// from what `grid routine schedule` recorded, so a synced edit of a project
// brain cannot change what an installed timer runs.
func (s Store) checkScheduled(e Entry) error {
	p := TimerPlan{Label: timerLabel(s.GridHome, s.Project, e.Name), Project: s.Project, Name: e.Name}
	state, err := Scheduler{GridHome: s.GridHome}.readState(p)
	if err != nil {
		return err
	}
	if state.Label == "" || state.Hash == e.Fingerprint() {
		return nil
	}
	return fmt.Errorf("%w: %s; run grid routine schedule again to approve it", ErrScheduleChanged, e.Name)
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
	if _, err := ParseRun(e.Run); err != nil {
		return err
	}
	switch e.Outcome {
	case "", "running", "success", "failed":
	default:
		return fmt.Errorf("invalid routine outcome")
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
	return decode(name, b)
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
			return nil, fmt.Errorf("routine %s: %w", name, err)
		}
		entries = append(entries, e)
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
	e := Entry{Name: name, Every: strings.TrimSpace(every), Run: strings.TrimSpace(run)}
	if len(policy) > 1 {
		return e, fmt.Errorf("expected at most one routine limit policy")
	}
	if len(policy) == 1 {
		e.LimitPolicy = policy[0]
	}
	var err error
	e.Limits, err = e.EffectiveLimits()
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
	if _, err := ParseRun(e.Run); err != nil {
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
		if old != nil {
			return nil, fmt.Errorf("routine %q already exists", name)
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		return brain.EncodeConfig(e, "")
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
	result := RunResult{Entry: e}
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
	runErr := runner.Execute(ctx, s, e)
	e.Outcome = "success"
	if runErr != nil {
		e.Outcome = "failed"
	}
	result.Entry = e
	saveErr := s.save(e)
	var signalErr error
	if runErr != nil && onFailure != nil {
		signalErr = onFailure(e, runErr)
	}
	return result, errors.Join(runErr, saveErr, signalErr)
}
