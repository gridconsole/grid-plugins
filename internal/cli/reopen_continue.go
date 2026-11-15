package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/session"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// A task whose Jobs a person stopped can be continued (eval round 13, T-02).
//
// Two stops end an agent, and the app worker (since app c8bec32d, end review 1012Z
// ruling 1) treats them differently:
//
//   - A worker stop (grid-worker stop, a signal, an update) ends each running agent
//     with a worker-exit `cancelled` whose code is worker_stopped, and the worker
//     posts the Job's Ready control itself. The Jobs stay Ready and the task stays
//     running; nothing is stopped and a reopen of it is refused, as it was.
//   - A task stop (the app's stop, grid task stop) ends each running agent with the
//     code stop_requested and the worker cancels the Job ("task stopped"). The task
//     is closed with only cancelled Jobs, and `grid task reopen` gives queued with no
//     open Job: the worker ticks a task only while a Job is open, so nothing started.
//
// reopenPlanJob posts a plan Job only when the task has an accepted approve step to
// answer again. The reopen here posts what the worker can start: an orchestrator Job
// like J-1, which reads the request and the Jobs on the Feed and posts what the task
// still needs.
//
// A running task whose Job carries stop_requested is the shape that a worker stop
// left before app c8bec32d; on a Feed the new worker wrote it is not reached, and the
// branch only serves a Feed written before.
//
// It posts a Job and refuses nothing new: a running task that has an open Job, or no
// Job a stop ended, gets the refusal it got before. A closed task gets the Job only
// when its last move to closed was a stop: a delivered task that the land approval
// closed, and once had a cancelled Job, is reopened as before with nothing posted.

// reopenContinueTitle is the title of the orchestrator Job a reopen posts after a stop.
const reopenContinueTitle = "Continue the task after it was stopped"

// reopenContinueText is the acceptance text of that Job.
const reopenContinueText = "The task was stopped and its Jobs were cancelled. Read the Jobs on the Feed with `grid job tree`, post again with `grid job post` the Jobs the task still needs, each with its `after` list, store plan.md with `grid plan write` when the plan changes, and post a result with `grid job result`."

// reopenContinuation is the orchestrator Job a reopen posts for a task that has no
// open Job. It is built before the reopen, so a failure to post leaves the task as it
// was, and posted after it, holding no task or plan lock.
type reopenContinuation struct {
	// fromRunning is true when the task is running: the reopen then goes through
	// closed (stop, then reopen), as a person's grid task stop and grid task reopen do.
	fromRunning bool
	post        func() error
	drop        func()
}

// reopenStopCode is the worker-exit code of an agent that a task stop ended (and, on a
// Feed written before app c8bec32d, one that a worker stop ended).
const reopenStopCode = "stop_requested"

// reopenClosedByStop reports whether the last receipt in history.md that moved the
// task to closed was a stop. Receipts are the "- {json}" lines of task.Event; a
// history with no such receipt, or one that cannot be read, is not a stop.
func reopenClosedByStop(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		return false
	}
	what := ""
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scan.Scan() {
		raw, ok := strings.CutPrefix(scan.Text(), "- {")
		if !ok {
			continue
		}
		var e taskstore.Event
		if json.Unmarshal([]byte("{"+raw), &e) == nil && e.ToState == "closed" {
			what = e.What
		}
	}
	return what == "stop"
}

// reopenCodeIs reports whether a folded worker-exit code is the JSON string want. A
// number, no code or any other string is not it.
func reopenCodeIs(raw json.RawMessage, want string) bool {
	var s string
	return len(raw) > 0 && json.Unmarshal(raw, &s) == nil && s == want
}

// reopenContinueJob prepares the orchestrator Job of a reopen, or returns nil when the
// reopen has none to post: the task is not running or closed, the Feed holds no Job
// (the worker posts J-1 itself), a Job is open, or no Job was cancelled by a stop. A
// running task needs a Job whose agent a stop ended (code stop_requested, a Feed
// written before app c8bec32d); a closed task needs a cancelled Job and a last move
// to closed that was a stop (reopenClosedByStop), not a close.
func reopenContinueJob(env Env, dir string) (*reopenContinuation, error) {
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return nil, nil
	}
	fm := taskfile.ScalarFrontmatter(string(b))
	if fm == nil {
		return nil, nil
	}
	state, _ := fm.Data["state"].(string)
	if state != "running" && state != "closed" {
		return nil, nil
	}
	kind, _ := fm.Data["kind"].(string)
	id, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return nil, nil
	}
	store := feed.Store{TaskDir: dir, TaskID: id, Warn: env.Stderr}
	snap, err := orchestrator.Load(store)
	if err != nil || len(snap.State.Jobs) == 0 {
		return nil, nil
	}
	cancelled, stopped := 0, 0
	for _, j := range snap.State.Jobs {
		switch j.Status {
		case contract.Accepted, contract.Rejected:
		case contract.Cancelled:
			cancelled++
			if reopenCodeIs(j.ExitCode, reopenStopCode) {
				stopped++
			}
		default:
			return nil, nil
		}
	}
	if cancelled == 0 || (state == "running" && stopped == 0) {
		return nil, nil
	}
	if state == "closed" && !reopenClosedByStop(dir) {
		return nil, nil
	}
	w, release, begin, err := reopenTaskWriter(env, store)
	if err != nil {
		return nil, fmt.Errorf("the orchestrator Job cannot be posted for the reopened task: %w", err)
	}
	record := func(id string) ([]byte, error) {
		body := firstJobOf(kind)
		body["id"] = id
		body["title"] = reopenContinueTitle
		body["acceptance"] = []map[string]any{{"id": "A1", "text": reopenContinueText, "traces": "owner"}}
		return json.Marshal(body)
	}
	return &reopenContinuation{
		fromRunning: state == "running",
		post: func() error {
			defer release()
			// The Handover record is written here, after the state moves held: a
			// refused move leaves nothing on the Feed.
			if err := begin(); err != nil {
				return err
			}
			return reopenPostJob(store, w, 0, record)
		},
		drop: release,
	}, nil
}

// reopenTaskWriter is the writer of a task write that a reopen makes: the task epoch
// the caller holds (GRID_TASK_EPOCH, or GRID_LEASE_EPOCH unless it was typed as a
// global), as reopenPlanJob takes it. A person at a terminal holds none, so the person
// acquires the task lease for this one post (free, or left by a worker process that is
// gone), as grid task new does for J-1. release gives it back; it is safe to call twice.
// begin writes the Handover record that a lease taken anew must open its tenure with;
// it does nothing for a lease the caller already held. It is not called here: the
// caller calls it after the state moves, so a refused move writes no record.
func reopenTaskWriter(env Env, store feed.Store) (w feed.Writer, release func(), begin func() error, err error) {
	none := func() {}
	noBegin := func() error { return nil }
	epoch := env.getenv("GRID_LEASE_EPOCH")
	if t := env.getenv("GRID_TASK_EPOCH"); t != "" && !env.typedGlobal["GRID_LEASE_EPOCH"] {
		epoch = t
	}
	if epoch != "" {
		w, err := orchestratorWriter(env, "task", epoch)
		return w, none, noBegin, err
	}
	w, err = orchestratorWriter(env, "task", "0.0")
	if err != nil {
		return w, none, noBegin, err
	}
	if !contract.ValidWorker(w.Worker) {
		return w, none, noBegin, fmt.Errorf("%q is not a valid <member>@<machine ULID> writer id", w.Worker)
	}
	pid, began := os.Getpid(), time.Time{}
	if env.OwnerPID != 0 {
		pid, began = env.OwnerPID, env.OwnerStart
	}
	if began.IsZero() {
		began, err = time.Parse(time.RFC3339Nano, migrate.ProcessStart(pid))
		if err != nil {
			began = time.Now()
		}
	}
	// A worker that was stopped never gave the lease back: the lease file still names
	// its process, which is gone. AcquireTask takes a free lease and adopts one whose
	// holder is provably gone on this host, as the worker's own pass does
	// (internal/lease/task.go AcquireTask); a live holder keeps it and the reopen is
	// refused before any state moves.
	ts := lease.TaskStore{Feed: store}
	got, err := ts.AcquireTask(w, pid, contract.FormatTime(began), session.ProbeLocal)
	if err != nil {
		return w, none, noBegin, fmt.Errorf("the task lease cannot be taken: %w", err)
	}
	w.Gen, w.Fence = got.Tenure.Gen, got.Tenure.Fence
	released := false
	release = func() {
		if !released {
			released = true
			_ = ts.ReleaseTask(w)
		}
	}
	begin = noBegin
	if got.How != lease.AcquiredReused {
		begin = func() error {
			_, err := store.Handover(feed.Input{Body: map[string]any{}}, w)
			return err
		}
	}
	return w, release, begin, nil
}
