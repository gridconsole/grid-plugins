// Package session stores task session history and supervision observations.
// Hooks record activity; only a supervisor with process evidence updates liveness.
// Reading a record never turns an old heartbeat into a stale session.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

const (
	HeartbeatInterval = time.Minute
	LeaseDuration     = 10 * time.Minute
	UnknownLimit      = 2 * LeaseDuration
	IdleSleepAfter    = 10 * time.Minute
)

type State string

const (
	Running         State = "Running"
	Sleeping        State = "Sleeping"
	Stale           State = "Stale"
	LivenessUnknown State = "Liveness unknown"
	Finished        State = "Finished"
	Errored         State = "Errored"
	StoppedByYou    State = "Stopped by you"
)

// Key includes the machine because the same runtime session can be resumed on
// another computer. Components are validated rather than silently rewritten.
type Key struct {
	SessionID string `json:"session_id"`
	MachineID string `json:"machine_id"`
}

type Record struct {
	Key
	Runtime     string     `json:"runtime"`
	Where       string     `json:"where"`
	Cwd         string     `json:"cwd,omitempty"`
	Transcript  string     `json:"transcript,omitempty"`
	Started     time.Time  `json:"started"`
	Last        time.Time  `json:"last"`
	Ended       *time.Time `json:"ended,omitempty"`
	Slept       *time.Time `json:"slept,omitempty"`
	State       State      `json:"state"`
	PID         int        `json:"pid"`
	PIDStart    string     `json:"pid_start"`
	Lease       string     `json:"lease,omitempty"`
	Holder      string     `json:"holder,omitempty"`
	Fence       uint64     `json:"fence,omitempty"`
	Attempt     int        `json:"attempt,omitempty"`
	LastAlive   time.Time  `json:"last_alive"`
	Until       time.Time  `json:"until"`
	HostOffline bool       `json:"host_offline,omitempty"`
	WakePending bool       `json:"wake_pending,omitempty"`
}

// Live is the feed-contract.md section 8 file, kept in <task>/feed/.live/ but
// separate from the Feed records.
type Live struct {
	Lease    string    `json:"lease"`
	Holder   string    `json:"holder"`
	Fence    uint64    `json:"fence"`
	Attempt  int       `json:"attempt"`
	PID      int       `json:"pid"`
	PIDStart string    `json:"pid_start"`
	BeatAt   time.Time `json:"beat_at"`
	OutputAt time.Time `json:"output_at"`
	Until    time.Time `json:"until"`
}

// MarshalJSON writes the schema's fixed millisecond timestamps; reading stays lenient.
func (l Live) MarshalJSON() ([]byte, error) {
	type wire Live
	return json.Marshal(struct {
		wire
		BeatAt   string `json:"beat_at"`
		OutputAt string `json:"output_at"`
		Until    string `json:"until"`
	}{wire(l), contract.FormatTime(l.BeatAt), contract.FormatTime(l.OutputAt), contract.FormatTime(l.Until)})
}

type ProcessState int

const (
	Unknown ProcessState = iota
	Alive
	Dead // A successful check found the pid gone or its start time changed.
)

// Observation is supplied by the task lease holder, never a reader. AfterWake
// suppresses timeout decisions until process identities have been rechecked.
// Alive means both pid and start time matched, not merely that kill(pid, 0) worked.
type Observation struct {
	Process   ProcessState
	PID       int
	PIDStart  string
	AfterWake bool
	OutputAt  time.Time
}

type Store struct {
	TaskDir string
	Now     func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// component also names Job holders, whose member part is a git email that may
// carry a +tag.
var component = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+-]{0,159}$`)

func validateKey(k Key) error {
	if !component.MatchString(k.SessionID) || !component.MatchString(k.MachineID) {
		return errors.New("session and machine ids must be safe nonempty file components")
	}
	return nil
}

// recordPath and livePath return paths inside verified folders; only validated
// keys and tenures may reach them.
func (s Store) recordPath(k Key, create bool) (string, error) {
	dir, err := s.storageDir(create, "sessions", k.MachineID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, k.SessionID+".json"), nil
}
func (s Store) livePath(r Record, create bool) (string, error) {
	dir, err := s.storageDir(create, "feed", ".live")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%d.json", r.Holder, r.Fence)), nil
}
func validateTenure(r Record) error {
	if r.PID <= 0 || r.PIDStart == "" {
		return errors.New("session needs a pid and its start time")
	}
	if r.Lease != "" && (!component.MatchString(r.Holder) || r.Fence == 0 || r.Attempt <= 0) {
		return errors.New("Job liveness needs a holder, fence and attempt")
	}
	return nil
}

// Start creates or explicitly resumes a Claude session. Resume preserves its
// first start and transcript, clears its end/sleep marker, and requires fresh
// process evidence before showing Running. Activity alone never renews a lease.
func (s Store) Start(r Record) (Record, error) {
	if err := validateKey(r.Key); err != nil {
		return Record{}, err
	}
	if r.Runtime != "" && r.Runtime != "claude" {
		return Record{}, errors.New("only Claude session records are supported")
	}
	if err := validateTenure(r); err != nil {
		return Record{}, err
	}
	return s.change(r.Key, func() (Record, error) {
		old, err := s.Show(r.Key)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Record{}, err
		}
		now := s.now()
		r.Runtime, r.Started, r.Last = "claude", now, now
		r.Ended, r.Slept, r.HostOffline, r.WakePending = nil, nil, false, false
		r.LastAlive, r.Until = time.Time{}, time.Time{}
		r.State = LivenessUnknown
		if r.Where == "" {
			r.Where = "terminal"
		}
		if err == nil {
			// A terminal resume must not drop a Job tenure or its heartbeat.
			// Check under the session lock, including a Job started since List.
			if old.Lease != "" && r.Lease == "" {
				return Record{}, fmt.Errorf("session %s belongs to Job lease %s; use a different --session", r.SessionID, old.Lease)
			}
			r.Started = old.Started
			if r.Transcript == "" {
				r.Transcript = old.Transcript
			}
			if err := s.removeLive(old); err != nil {
				return Record{}, err
			}
		}
		// No successful check yet: use the start of this tenure as its grace period.
		if r.Lease != "" {
			r.Until = now.Add(LeaseDuration)
		}
		return r, s.save(r)
	})
}

// Touch records hook activity without changing liveness or waking a session.
func (s Store) Touch(k Key, transcript string) (Record, error) {
	return s.update(k, func(r Record) (Record, error) {
		r.Last = s.now()
		if transcript != "" {
			r.Transcript = transcript
		}
		return r, s.save(r)
	})
}

// Fold is deliberately clock-free. State changes have already been persisted
// by the task lease holder, so every reader displays the same decision.
func Fold(r Record) State { return r.State }

func (s Store) Reconcile(k Key, observation Observation) (Record, error) {
	return s.update(k, func(r Record) (Record, error) { return s.reconcile(r, observation) })
}
func (s Store) reconcile(r Record, o Observation) (Record, error) {
	if r.Ended != nil || r.State == Sleeping {
		return r, nil
	}
	if o.PID != r.PID || o.PIDStart != r.PIDStart {
		return Record{}, errors.New("process observation belongs to a different session incarnation")
	}
	now := s.now()
	if o.AfterWake {
		r.WakePending = true
	}
	switch o.Process {
	case Alive:
		r.State, r.HostOffline = Running, false
		r.WakePending = false
		r.LastAlive = now
		if r.Lease != "" {
			r.Until = now.Add(LeaseDuration)
			if o.OutputAt.IsZero() {
				previous, err := s.ReadHeartbeat(r)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return Record{}, err
				}
				o.OutputAt = previous.OutputAt
			}
			live := Live{r.Lease, r.Holder, r.Fence, r.Attempt, r.PID, r.PIDStart, now, o.OutputAt, r.Until}
			path, err := s.livePath(r, true)
			if err != nil {
				return Record{}, err
			}
			if err := atomicJSON(path, live); err != nil {
				return Record{}, err
			}
		}
	case Dead:
		r.State, r.Ended = Stale, &now
		r.WakePending = false
	case Unknown:
		r.State = LivenessUnknown
		// Plain terminal sessions use pid identity only. Unknown is never proof of
		// death for them, and laptop sleep must never consume a Job's grace period.
		if r.Lease != "" && !r.WakePending && !now.Before(r.Until) {
			last := r.LastAlive
			if last.IsZero() {
				last = r.Until.Add(-LeaseDuration)
			}
			if now.Sub(last) >= UnknownLimit {
				r.State, r.Ended, r.HostOffline = Stale, &now, true
			}
		}
	default:
		return Record{}, errors.New("invalid process observation")
	}
	if err := s.save(r); err != nil {
		return Record{}, err
	}
	if r.State == Stale {
		return r, s.removeLive(r)
	}
	return r, nil
}

// Sleep records the supervisor's intentional stop after idle time. The runtime
// id and transcript survive, and cleanup will not mistake it for a dead process.
func (s Store) Sleep(k Key) (Record, error) {
	return s.update(k, func(r Record) (Record, error) {
		if r.Ended != nil {
			return Record{}, errors.New("cannot sleep an ended session")
		}
		now := s.now()
		r.State, r.Slept = Sleeping, &now
		if err := s.save(r); err != nil {
			return Record{}, err
		}
		return r, s.removeLive(r)
	})
}
func (s Store) End(k Key, state State) (Record, error) {
	if state != Finished && state != Errored && state != StoppedByYou {
		return Record{}, errors.New("invalid session end state")
	}
	return s.update(k, func(r Record) (Record, error) {
		now := s.now()
		r.State, r.Ended, r.Slept = state, &now, nil
		if err := s.save(r); err != nil {
			return Record{}, err
		}
		return r, s.removeLive(r)
	})
}

// Cleanup checks current records while serialized with writers. It removes
// heartbeat files of confirmed dead sessions, marks them Stale, and keeps the
// history for resume and transcript inspection. A failed probe means unknown.
// The probe must be bounded and must not call back into this Store's writers.
func (s Store) Cleanup(probe func(Record) (Observation, error)) ([]Key, error) {
	if probe == nil {
		return nil, errors.New("process probe is required")
	}
	var cleaned []Key
	err := s.locked(func() error {
		records, err := s.List()
		if err != nil {
			return err
		}
		for _, r := range records {
			if r.State == Sleeping {
				continue
			}
			if r.Ended != nil {
				if err := s.removeLive(r); err != nil {
					return err
				}
				continue
			}
			obs, err := probe(r)
			if err != nil {
				obs.Process = Unknown
				obs.PID, obs.PIDStart = r.PID, r.PIDStart
			} else if obs.PID != r.PID || obs.PIDStart != r.PIDStart {
				// Evidence about another incarnation proves nothing about this
				// one; it must not abort the sweep over the other records.
				obs = Observation{Process: Unknown, PID: r.PID, PIDStart: r.PIDStart}
			}
			next, err := s.reconcile(r, obs)
			if err != nil {
				return err
			}
			if next.State == Stale {
				cleaned = append(cleaned, r.Key)
			}
		}
		return nil
	})
	return cleaned, err
}

// Show and List are read-only and never infer death from wall-clock age.
func (s Store) Show(k Key) (Record, error) {
	if err := validateKey(k); err != nil {
		return Record{}, err
	}
	path, err := s.recordPath(k, false)
	if err != nil {
		return Record{}, err
	}
	b, err := readFile(path)
	if err != nil {
		return Record{}, err
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return Record{}, fmt.Errorf("read session %s: %w", k.SessionID, err)
	}
	if r.Key != k {
		return Record{}, errors.New("session record identity does not match its path")
	}
	if err := validateTenure(r); err != nil {
		return Record{}, err
	}
	switch r.State {
	case Running, Sleeping, Stale, LivenessUnknown, Finished, Errored, StoppedByYou:
	default:
		return Record{}, errors.New("invalid session state")
	}
	return r, nil
}
func (s Store) List() ([]Record, error) {
	records := []Record{}
	dir, err := s.storageDir(false, "sessions")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return records, nil
		}
		return nil, err
	}
	machines, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if !machine.IsDir() || machine.Name() == "hooks" || !component.MatchString(machine.Name()) {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(dir, machine.Name()))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			r, err := s.Show(Key{strings.TrimSuffix(entry.Name(), ".json"), machine.Name()})
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
	}
	return records, nil
}
func (s Store) ReadHeartbeat(r Record) (Live, error) {
	if err := validateKey(r.Key); err != nil {
		return Live{}, err
	}
	if err := validateTenure(r); err != nil {
		return Live{}, err
	}
	if r.Lease == "" {
		return Live{}, errors.New("session has no Job heartbeat")
	}
	path, err := s.livePath(r, false)
	if err != nil {
		return Live{}, err
	}
	b, err := readFile(path)
	if err != nil {
		return Live{}, err
	}
	var live Live
	if err := json.Unmarshal(b, &live); err != nil {
		return Live{}, err
	}
	if live.Lease != r.Lease || live.Holder != r.Holder || live.Fence != r.Fence || live.Attempt != r.Attempt || live.PID != r.PID || live.PIDStart != r.PIDStart {
		return Live{}, errors.New("heartbeat does not match the session tenure")
	}
	return live, nil
}
func (s Store) save(r Record) error {
	path, err := s.recordPath(r.Key, true)
	if err != nil {
		return err
	}
	return atomicJSON(path, r)
}
func (s Store) removeLive(r Record) error {
	if r.Lease == "" {
		return nil
	}
	if err := validateTenure(r); err != nil {
		return err
	}
	path, err := s.livePath(r, false)
	if err == nil {
		err = os.Remove(path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (s Store) update(k Key, fn func(Record) (Record, error)) (Record, error) {
	return s.change(k, func() (Record, error) {
		r, err := s.Show(k)
		if err != nil {
			return Record{}, err
		}
		return fn(r)
	})
}
func (s Store) change(k Key, fn func() (Record, error)) (Record, error) {
	if err := validateKey(k); err != nil {
		return Record{}, err
	}
	var result Record
	err := s.locked(func() error {
		var err error
		result, err = fn()
		return err
	})
	return result, err
}
