package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// A stopped or closed task gets its open Jobs cancelled and nothing more spawned.
//
// `grid task stop` and close only rewrite task.md (internal/task/write.go: "it
// records intent, not a claim that a worker was killed"), and the stream used
// to work every task with an open Job whatever task.md said. Now Stream.task
// calls pgClosedTask after it holds the task lease: when the state is closed it
// posts one cancelled control per open Job, under the task lease, gives the
// lease back and does not tick, so nothing is spawned and nothing is re-planned.
// A reopen writes another state and posts a plan Job for a person (internal/cli
// task_write.go, reopenPlanJob); that is the path of today, and the cancelled
// Jobs stay cancelled.
//
// Does a Cancelled control end a Job's grant? No. In the plugin a grant is the
// lease file lease/<job>.json written by TaskStore.GrantJob (internal/lease/task.go:310)
// and ended only by TaskStore.end (:406), which has two callers: ReleaseTask
// (:295, the task lease of a stream, not a Job lease) and grant (:350, when the
// same lease name is granted again). Nothing in internal/lease or internal/feed
// reads a Job's status (a grep for the Cancelled status finds only
// feed/questions.go:155, a question about a cancelled Job). What a Cancelled control does change is the
// Job's writer: orchestrator/write.go:79 refuses a post under that Job's lease
// with ErrStaleLease once the Job is Cancelled, so a running child cannot post
// its ack, result or exit. The app worker stops a child only when WithLease
// reports that the lease file no longer names it (kernel/worker/supervisor.go
// Recover :662-679, Tick :723-747), and that stays true here: lease/<job>.json
// still names the holder. So a child that was running when the task was closed
// runs on until it ends by itself, and its posts are refused. That is an open
// item for the lease code or the app (end the Job grant on a Cancelled control,
// or have the worker stop a child whose Job is Cancelled), not something this
// file does: it changes neither.

// pgClosedReason is the close reason written on every Job a closed task loses.
const pgClosedReason = "task stopped"

// pgClosedRead reports whether task.md in dir has state: closed. A missing or
// unreadable file, no frontmatter, no state or any other state is false: the
// task is worked as it is today.
func pgClosedRead(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return false
	}
	fm := taskfile.ScalarFrontmatter(string(b))
	if fm == nil {
		return false
	}
	state, _ := fm.Data["state"].(string)
	return state == "closed"
}

// pgClosedCancel posts a cancelled control for every Job that is not Accepted,
// Cancelled or Rejected, under the task lock and the task lease of s.Writer, the
// way `grid job cancel` writes it. The state is read again inside the lock: a
// person's unleased write (a reopen's plan Job) takes the same lock, so a Job
// posted after a reopen is never cancelled by a pass that read the old state.
// closed is false when the task is no longer closed by then; nothing is written.
func (s Store) pgClosedCancel(dir string) (closed bool, err error) {
	err = s.locked(func() error {
		if s.Writer.Lease != "task" {
			return feed.ErrStaleLease
		}
		if !pgClosedRead(dir) {
			return nil
		}
		closed = true
		snap, e := Load(s.Feed)
		if e != nil {
			return e
		}
		for _, id := range ids(snap) {
			j := snap.State.Jobs[id]
			switch j.Status {
			case contract.Accepted, contract.Cancelled, contract.Rejected:
				continue
			}
			if e := s.control(j, contract.Cancelled, pgClosedReason, ""); e != nil {
				return e
			}
		}
		return nil
	})
	return closed, err
}

// pgClosedTask is Stream.task's step for a closed task, after the task lease is
// held and before the tick. handled is true when the task is closed: the caller
// returns err and does not tick. A task that is not closed is left alone.
func (s *Stream) pgClosedTask(dir, taskID string, ts lease.TaskStore, w feed.Writer, st Store) (handled bool, err error) {
	if !pgClosedRead(dir) {
		return false, nil
	}
	closed, cerr := st.pgClosedCancel(dir)
	if cerr != nil {
		if errors.Is(cerr, feed.ErrStaleLease) {
			return true, s.skip(dir, taskID, SkipTaskHeld, cerr)
		}
		return true, s.skip(dir, taskID, SkipTickFailed, cerr)
	}
	if !closed {
		return false, nil // reopened since the read: the path of today
	}
	delete(s.skipped, dir)
	// Every Job is closed now: give the lease back, as the end of task() does. A
	// Job a person posted since keeps it until the next pass cancels that too.
	if after, lerr := Load(ts.Feed); lerr == nil && !hasOpenJob(after) {
		if rerr := ts.ReleaseTask(w); rerr != nil && !errors.Is(rerr, lease.ErrTaskFree) && !errors.Is(rerr, lease.ErrTaskHeld) {
			s.diag("grid worker next: task %s: release: %s\n", taskID, oneLine(rerr))
		}
		delete(s.handed, fmt.Sprintf("%s %d.%d", dir, w.Gen, w.Fence))
	}
	return true, nil
}
