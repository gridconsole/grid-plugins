package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

var ErrDuplicateWake = errors.New("wake write already exists")

// Write serializes command-side checks with assignments, then delegates the
// actual append, fence check and wake deduplication to Feed. The returned record
// on ErrDuplicateWake is the first write, which the CLI prints before refusing.
func Write(store feed.Store, w feed.Writer, in feed.Input) (record feed.Record, err error) {
	return WriteUnless(store, w, in, nil)
}

// WriteUnless is Write with a check made under .orchestrator.lock, in the same
// pass as the write: when done returns true nothing is written and the record is
// empty. A check made before the lock could be overtaken by a write from another
// process, so a caller that must not write twice asks here.
func WriteUnless(store feed.Store, w feed.Writer, in feed.Input, done func() bool) (record feed.Record, err error) {
	if err := store.CheckWithdrawalPolicy(in); err != nil {
		return record, err
	}
	if w.Lease == "" {
		return record, feed.ErrStaleLease
	}
	guard := w
	if w.Lease != "task" {
		task, e := (lease.TaskStore{Feed: store}).Read("task")
		if e != nil {
			return record, e
		}
		if task.Holder == nil {
			return record, feed.ErrStaleLease
		}
		guard.Lease, guard.Worker, guard.Gen, guard.Fence = "task", *task.Holder, task.Gen, task.Fence
	}
	err = (Store{Feed: store, Writer: guard}).locked(func() error {
		if done != nil && done() {
			return nil
		}
		var e error
		record, e = writeLocked(store, w, in)
		return e
	})
	return
}

func writeLocked(store feed.Store, w feed.Writer, in feed.Input) (record feed.Record, err error) {
	snap, err := Load(store)
	if err != nil {
		return record, err
	}
	if in.Type == "control" {
		target := stringValue(in.Body["target"])
		if strings.HasPrefix(target, "J-") && snap.State.Jobs[target] == nil {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("unknown Job %s", target)}
		}
	}
	if in.Type == "job" {
		if err := checkAfterTargets(snap, in); err != nil {
			return record, err
		}
	}
	if w.Lease != "task" {
		j := snap.State.Jobs[in.Job]
		if j == nil || j.Assign == nil || j.Attempt != in.Attempt || j.Assign.Fence != w.Fence || j.Assign.To != w.Worker || j.Generation != w.Gen || j.Status == contract.Cancelled || j.Status == contract.Rejected || j.Status == contract.Accepted {
			return record, feed.ErrStaleLease
		}
		if in.Type == "ack" {
			in.After = []string{j.AssignPost}
		} else if j.Status != contract.Running {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("Job tenure must start with ack")}
		}
		if j.Exited != "" {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("attempt already exited")}
		}
	}
	if in.ID == "" {
		in.ID, err = feed.NewID()
		if err != nil {
			return record, err
		}
	}
	record, err = store.Post(in, w)
	if err == nil && in.Wake != "" && record.ID != in.ID {
		err = ErrDuplicateWake
	}
	return
}

// checkAfterTargets refuses a Job that waits on a Job the task does not have, so
// the typo is found when the Job is posted. A gate is checked by its own path.
func checkAfterTargets(snap Snapshot, in feed.Input) error {
	data, err := json.Marshal(in.Body)
	if err != nil {
		return nil
	}
	var item job.Job
	if json.Unmarshal(data, &item) != nil {
		return nil // the schema reports a malformed body
	}
	for _, e := range item.After {
		if strings.HasPrefix(e.Job, "gate:") || snap.State.Jobs[e.Job] != nil {
			continue
		}
		return &feed.InvalidRecordError{Err: fmt.Errorf("%s: after target %q does not exist; post %s first", item.ID, e.Job, e.Job)}
	}
	return nil
}

type Attempt struct {
	Number int    `json:"attempt"`
	Fence  int64  `json:"fence"`
	State  string `json:"state"`
	Exit   string `json:"exit,omitempty"`
}
// Orphan kinds: a Job that nobody is working and nothing will move by itself.
const (
	OrphanNeverPickedUp = "never_picked_up"
	OrphanNeverAcked    = "never_acked"
	OrphanNoExit        = "no_exit"
)

// Orphan names a Job left behind: ready and never assigned, assigned and never
// acked past its lease, or acked with no result and no worker-exit past its
// lease. It is a report; nothing acts on it.
type Orphan struct {
	Kind   string `json:"kind"`
	Since  string `json:"since"`
	Detail string `json:"detail"`
}

type TreeJob struct {
	*contract.Job
	Attempts []Attempt `json:"attempts"`
	Orphan   *Orphan   `json:"orphan,omitempty"`
}

func Tree(s Snapshot) []TreeJob { return TreeAt(s, time.Now()) }

// TreeAt is Tree judged at now: a Job whose clock ran out reads as an orphan.
func TreeAt(s Snapshot, now time.Time) []TreeJob {
	out := []TreeJob{}
	// Dependency readiness only; a person's Job and a Job behind an open after
	// are not ready, so they never read as never picked up.
	plain := s
	plain.Assignee = ""
	readiness, since, err := readyAt(plain, now)
	ready := map[string]bool{}
	if err == nil {
		for _, id := range readiness.Ready {
			ready[id] = true
		}
	}
	for _, id := range ids(s) {
		j := s.State.Jobs[id]
		row := TreeJob{Job: j, Attempts: []Attempt{}}
		var until time.Time
		var to string
		hasResult := false
		for _, event := range j.History {
			r := s.Records[event]
			if a, ok := r.Body["assign"].(map[string]any); ok {
				row.Attempts = append(row.Attempts, Attempt{Number: int(a["attempt"].(float64)), Fence: int64(a["fence"].(float64)), State: "assigned"})
				until, _ = time.Parse(time.RFC3339Nano, stringValue(a["until"]))
				to, hasResult = stringValue(a["to"]), false
			} else if len(row.Attempts) > 0 {
				a := &row.Attempts[len(row.Attempts)-1]
				switch r.Type {
				case "result":
					hasResult = true
				case "ack":
					a.State = "running"
				case "worker-exit":
					a.Exit = stringValue(r.Body["reason"])
					if a.Exit == "" {
						a.Exit = "crash"
					}
					a.State = "exited"
				case "control":
					if status := stringValue(r.Body["status"]); status != "" {
						a.State = status
					}
				}
			}
		}
		row.Orphan = orphanOf(j, row.Attempts, ready[id], since[id], until, to, hasResult, now)
		out = append(out, row)
	}
	return out
}

// orphanOf judges one Job on its latest attempt only. A closed Job has none.
func orphanOf(j *contract.Job, attempts []Attempt, ready bool, readySince, until time.Time, to string, hasResult bool, now time.Time) *Orphan {
	if j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
		return nil
	}
	if len(attempts) == 0 {
		if ready && (j.Status == contract.Posted || j.Status == contract.Ready) && !readySince.IsZero() && now.After(readySince) {
			return &Orphan{Kind: OrphanNeverPickedUp, Since: contract.FormatTime(readySince), Detail: "ready and not assigned to any worker"}
		}
		return nil
	}
	last := attempts[len(attempts)-1]
	switch last.State {
	case "assigned":
		if !until.IsZero() && until.Before(now) {
			return &Orphan{Kind: OrphanNeverAcked, Since: contract.FormatTime(until), Detail: fmt.Sprintf("attempt %d assigned to %s and not acked before its lease ran out", last.Number, to)}
		}
	case "running":
		lease, _ := time.Parse(time.RFC3339Nano, j.LeaseExpires)
		if !hasResult && !lease.IsZero() && lease.Before(now) {
			return &Orphan{Kind: OrphanNoExit, Since: contract.FormatTime(lease), Detail: fmt.Sprintf("attempt %d acked by %s, no result and no worker-exit after its lease ran out", last.Number, to)}
		}
	}
	return nil
}
