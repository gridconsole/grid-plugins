package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
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
	// Code is the worker-exit's code as text; ReasonMissing says the worker-exit
	// gave no reason, so Exit reads crash only by default.
	Code          string `json:"code,omitempty"`
	ReasonMissing bool   `json:"reason_missing,omitempty"`
	// NoResult says the agent ended with exit done and no result record came for
	// this attempt: an empty finish. It is shown, never acted on.
	NoResult bool `json:"no_result,omitempty"`
	// The record times of the assign, the ack and the worker-exit. They come from
	// the Feed records, never from the clock.
	AssignedAt string `json:"assigned_at,omitempty"`
	AckedAt    string `json:"acked_at,omitempty"`
	ExitedAt   string `json:"exited_at,omitempty"`
}

// recordTime reads a record's At as the time layout every other tree time uses.
func recordTime(at string) string {
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return at
	}
	return contract.FormatTime(t)
}

// exitCodeText reads a worker-exit body's code the way the fold keeps it: a
// non-empty string, or a whole number written without its fraction.
func exitCodeText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case float64:
		if c == math.Trunc(c) && math.Abs(c) < 1<<53 {
			return strconv.FormatInt(int64(c), 10)
		}
	}
	return ""
}

// Orphan kinds: a Job that nobody is working and nothing will move by itself.
const (
	OrphanNeverPickedUp = "never_picked_up"
	OrphanNeverAcked    = "never_acked"
	OrphanNoExit        = "no_exit"
	// OrphanDeadWait is a Posted or Ready Job that waits through a done edge on a
	// Job that was rejected or cancelled: it is never released by itself.
	OrphanDeadWait = "dead_wait"
	// OrphanNeverJudged is a Submitted Job past its lease that no Judge has taken:
	// the wake of its latest result is not acknowledged and no review or person
	// Job stands from it. The review and the rework come from the app's Judge, so
	// where the app's worker does not run a done Job stops here.
	OrphanNeverJudged = "never_judged"
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
	// ReadyAt is when the Job became ready: its post, or the record that closed
	// the last Job it waited on. A Job still waiting has none. PickupSeconds is
	// the latest attempt's ack minus ReadyAt, and is absent until an ack exists.
	ReadyAt       string   `json:"ready_at,omitempty"`
	PickupSeconds *float64 `json:"pickup_seconds,omitempty"`
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
	// open: no unmet after, so the Job has a ready time. A person's Job is held
	// back from Ready by a wait with no target; a started Job was ready to start.
	open := map[string]bool{}
	// dead: the first wait of a Job through a done edge on a Rejected or
	// Cancelled Job, which nothing will release by itself.
	dead := map[string]jobgraph.Wait{}
	if err == nil {
		for _, id := range readiness.Ready {
			ready[id], open[id] = true, true
		}
		for _, w := range readiness.Waiting {
			if w.Target == "" && w.Reason != "usage_limit" && w.Reason != "review_capacity" {
				open[w.Job] = true
			}
			if _, named := dead[w.Job]; !named && deadDoneWait(s, w) {
				dead[w.Job] = w
			}
		}
	}
	acked := ackedEvents(s)
	for _, id := range ids(s) {
		j := s.State.Jobs[id]
		row := TreeJob{Job: j, Attempts: []Attempt{}}
		var until time.Time
		var to string
		hasResult := false
		for _, event := range j.History {
			r := s.Records[event]
			if a, ok := r.Body["assign"].(map[string]any); ok {
				row.Attempts = append(row.Attempts, Attempt{Number: int(a["attempt"].(float64)), Fence: int64(a["fence"].(float64)), State: "assigned", AssignedAt: recordTime(r.At)})
				until, _ = time.Parse(time.RFC3339Nano, stringValue(a["until"]))
				to, hasResult = stringValue(a["to"]), false
			} else if len(row.Attempts) > 0 {
				a := &row.Attempts[len(row.Attempts)-1]
				switch r.Type {
				case "result":
					hasResult = true
					a.NoResult = false
				case "ack":
					a.State, a.AckedAt = "running", recordTime(r.At)
				case "worker-exit":
					a.ExitedAt = recordTime(r.At)
					a.Exit = stringValue(r.Body["reason"])
					if a.Exit == "" {
						a.Exit, a.ReasonMissing = "crash", true
					}
					a.State, a.Code = "exited", exitCodeText(r.Body["code"])
					// An empty finish: the agent ended done and this attempt posted no result.
					a.NoResult = a.Exit == "done" && !a.ReasonMissing && !hasResult
				case "control":
					if status := stringValue(r.Body["status"]); status != "" {
						a.State = status
					}
				}
			}
		}
		if (open[id] || len(row.Attempts) > 0) && !since[id].IsZero() {
			row.ReadyAt = contract.FormatTime(since[id])
			if n := len(row.Attempts); n > 0 && row.Attempts[n-1].AckedAt != "" {
				acked, _ := time.Parse(time.RFC3339Nano, row.Attempts[n-1].AckedAt)
				secs := acked.Sub(since[id]).Seconds()
				row.PickupSeconds = &secs
			}
		}
		row.Orphan = orphanOf(j, row.Attempts, ready[id], since[id], until, to, hasResult, now)
		if w, isDead := dead[id]; isDead && row.Orphan == nil && (j.Status == contract.Posted || j.Status == contract.Ready) {
			row.Orphan = &Orphan{Kind: OrphanDeadWait, Since: contract.FormatTime(closedAt(s, w.Target)), Detail: w.Reason}
		}
		if row.Orphan == nil {
			row.Orphan = neverJudged(s, j, acked, now)
		}
		out = append(out, row)
	}
	return out
}

// ackedEvents is the set of events some record answers with its wake ("<event>/<n>").
func ackedEvents(s Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, r := range s.Records {
		if r.Wake != nil {
			event, _, _ := strings.Cut(*r.Wake, "/")
			out[event] = true
		}
	}
	return out
}

// neverJudged names a Submitted Job whose lease ran out and that no Judge has
// taken: the wake of its latest result is not acknowledged, and no review, check
// or person Job was raised from it. The plugin only prints the wake; the review,
// the accept and the rework after a rejection come from the app's Judge, so a
// done Job waits here when the app's worker does not run. It is a report; nothing
// acts on it.
func neverJudged(s Snapshot, j *contract.Job, acked map[string]bool, now time.Time) *Orphan {
	if j.Status != contract.Submitted {
		return nil
	}
	lease, _ := time.Parse(time.RFC3339Nano, j.LeaseExpires)
	if lease.IsZero() || !lease.Before(now) {
		return nil
	}
	if n := len(j.Results); n > 0 && acked[j.Results[n-1]] {
		return nil
	}
	for _, other := range s.State.Jobs {
		if other.DiscoveredFrom == nil || *other.DiscoveredFrom != j.ID {
			continue
		}
		switch contract.NormalizeRole(other.Role) {
		case "reviewer", "checker", "person":
			return nil
		}
	}
	return &Orphan{Kind: OrphanNeverJudged, Since: contract.FormatTime(lease), Detail: "submitted, and no wake of its result acknowledged after its lease ran out; the review and any rework are posted by the app's Judge"}
}

// deadDoneWait reports whether w is a wait through a done edge on a Job that
// was rejected or cancelled. Such a Job is never released by itself, so the
// wait never ends. A gate, a failed edge and a live target are not dead.
func deadDoneWait(s Snapshot, w jobgraph.Wait) bool {
	if w.Target == "" || strings.HasPrefix(w.Target, "gate:") {
		return false
	}
	target := s.State.Jobs[w.Target]
	if target == nil || target.Status != contract.Rejected && target.Status != contract.Cancelled {
		return false
	}
	return w.Reason == jobgraph.DoneReason(w.Target, target.Status)
}

// closedAt is when a Job reached its present status: the latest control in its
// History whose status is that status, read as readyAt reads it.
func closedAt(s Snapshot, id string) time.Time {
	var at time.Time
	j := s.State.Jobs[id]
	if j == nil {
		return at
	}
	for _, rid := range j.History {
		if r := s.Records[rid]; r.Type == "control" && r.Body["status"] == string(j.Status) {
			at, _ = time.Parse(time.RFC3339Nano, r.At)
		}
	}
	return at
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
