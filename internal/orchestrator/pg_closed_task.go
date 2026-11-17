package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// Does a Cancelled control end a Job's grant? Not by itself. A grant is the lease
// file lease/<job>.json written by TaskStore.GrantJob (internal/lease/task.go:360)
// and ended by TaskStore.EndJob (:317) through end (:456); nothing in
// internal/lease or internal/feed reads a Job's status. Tick ends the grant of
// each Cancelled Job through taCancelEndGrants, but this pass does not tick. So
// pgClosedCancel calls taCancelEndGrants itself, in the same lock and under the
// same task lease, after the cancels: a snapshot read again, so it holds the
// Cancelled status just written, and EndJob writes the takeover line and clears
// the holder of each Job it cancelled. A Job that was never granted has no lease
// file and is left alone, and a later pass ends nothing twice (EndJob is
// idempotent). An error from that pass is returned like a control error.
//
// What the end of the grant is for: the app worker stops a child only when
// WithLease reports that the lease file no longer names it (kernel/worker/
// supervisor.go Recover, Tick), and orchestrator/write.go refuses a post under a
// Cancelled Job's lease with ErrStaleLease. With the holder cleared the worker
// can see the lease end. That the worker then stops the child is not checked in
// this file.

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

// One exception to "a closed task gets nothing more spawned": the curator.
//
// The brain needs no attention: a curator Job files the proposals a task left
// waiting and asks nobody (owner decision, SCOPES-11nov-0455c, owner question 4).
// `grid deliver` posts that Job, but a task a person declined, stopped or closed
// does not pass through deliver, and a curator Job deliver did post would be
// cancelled by the pass above, so the proposals would wait for ever. So, in the
// same locked pass: a Job of role curator is left alone (every other open Job is
// cancelled as before); a closed task with no open curator Job that has
// proposals waiting gets one, posted under the task lease; and the pass goes on
// to the tick while a curator Job is the only open Job, so it is assigned and
// spawned like any Job. The role is read the way openCuratorJob in internal/cli
// reads it. Nothing here caps, delays or refuses: the open-curator check under
// the task lock is what keeps a second pass from posting a second Job.

// usCuratorRole is the role of the Job that keeps a task's brain proposals.
const usCuratorRole = "curator"

// usCuratorJobs is the id of every Job of the snapshot whose role is curator, read
// case-insensitively as openCuratorJob reads it.
func usCuratorJobs(snap Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), usCuratorRole) {
			out[it.ID] = true
		}
	}
	return out
}

// usOpenJobs counts the Jobs that are not Accepted, Cancelled or Rejected, and how
// many of them are curator Jobs.
func usOpenJobs(snap Snapshot) (open, curators int) {
	curator := usCuratorJobs(snap)
	for id, j := range snap.State.Jobs {
		switch j.Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
			continue
		}
		open++
		if curator[id] {
			curators++
		}
	}
	return open, curators
}

// pgClosedCancel posts a cancelled control for every Job that is not Accepted,
// Cancelled or Rejected and is not a curator Job, under the task lock and the task
// lease of s.Writer, the way `grid job cancel` writes it. The state is read again
// inside the lock: a person's unleased write (a reopen's plan Job) takes the same
// lock, so a Job posted after a reopen is never cancelled by a pass that read the
// old state. closed is false when the task is no longer closed by then; nothing
// is written.
//
// curator, when set, is asked for the body of a curator Job once the other Jobs
// are cancelled and no curator Job is open; it answers nil when no proposal waits
// for the task. A body is posted under the same lock and lease, with the next free
// Job id. An error from curator, or a body the Job schema refuses, is one
// Diagnostics line and posts nothing: the cancels stand.
func (s Store) pgClosedCancel(dir, taskID string, curator func(dir, taskID, id string) (map[string]any, error)) (closed bool, err error) {
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
		keep := usCuratorJobs(snap)
		for _, id := range ids(snap) {
			j := snap.State.Jobs[id]
			switch j.Status {
			case contract.Accepted, contract.Cancelled, contract.Rejected:
				continue
			}
			if keep[id] {
				continue
			}
			s.pcClosedExit(taskID, j)
			if e := s.control(j, contract.Cancelled, pgClosedReason, ""); e != nil {
				return e
			}
		}
		// The snapshot above was read before the cancels. Read it again, so the
		// Jobs just cancelled have their grants ended here, where Tick never runs.
		cancelled, e := Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.taCancelEndGrants(cancelled); e != nil {
			return e
		}
		if curator == nil {
			return nil
		}
		if _, curators := usOpenJobs(snap); curators > 0 {
			return nil // a curator Job is open already: one is enough
		}
		return s.usPostCurator(snap, dir, taskID, curator)
	})
	return closed, err
}

// A cancelled attempt still ends with one worker-exit on the Feed (decision
// 2026-11-17-cancelled-job-feed-exit: the Feed is the event log, worker.log is not).
//
// After a task stop two passes race: the app worker's beat stops the agent and posts
// its exit (cancelled, code stop_requested), and this pass cancels the Job and ends
// its grant. When this pass ran first, the worker's exit came under a Cancelled Job
// and was refused as a stale lease (writeLocked), so the attempt had no exit on the
// Feed. So this pass posts the exit itself, before the cancel: reason cancelled and
// code "task stopped", the body worker-exit.json allows to role cli, as a Job-side
// record of the Job's own tenure (holder, gen, fence and attempt of its grant), which
// is the tenure the fold reads an exit from. Accepting the worker's exit after the
// cancel instead would mean admitting a post past the takeover line EndJob writes,
// which the Feed refuses (feed store.go leaseLock) and the fold reads as stale.
//
// Never two: the worker's exit goes through Write, which takes the same
// .orchestrator.lock and refuses an attempt already exited or a Cancelled Job. A
// Job whose attempt has an exit (the worker came first) gets none here. A Job that
// is not Running gets none: before its ack the fold ignores any Job-side record. A
// Job granted to another holder gets none: its exit is that holder's to write.
// The exit goes before the cancel, so a pass cut off between them heals: the next
// pass sees the exit and only cancels. A failed exit is one Diagnostics line, and
// the cancel still goes out.

// pcClosedExit posts the worker-exit of the running attempt of j, which the close pass
// is about to cancel. The caller holds the task lock and lease.
func (s Store) pcClosedExit(taskID string, j *contract.Job) {
	if j.Status != contract.Running || j.Assign == nil || j.Exited != "" {
		return
	}
	if j.Assign.To != s.Writer.Worker {
		s.usDiag("grid worker next: task %s: %s is held by %s, which posts its worker-exit\n", taskID, j.ID, j.Assign.To)
		return
	}
	w := feed.Writer{Author: s.Writer.Author, Worker: j.Assign.To, Lease: j.ID, Gen: j.Generation, Fence: j.Assign.Fence}
	in := feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"reason": "cancelled", "code": pgClosedReason}}
	if _, err := writeLocked(s.Feed, w, in); err != nil {
		s.usDiag("grid worker next: task %s: no worker-exit for %s attempt %d: %s\n", taskID, j.ID, j.Attempt, oneLine(err))
	}
}

// usPostCurator asks curator for the body of the curator Job of a closed task and
// posts it. The caller holds the task lock and lease and has seen no open curator Job.
func (s Store) usPostCurator(snap Snapshot, dir, taskID string, curator func(dir, taskID, id string) (map[string]any, error)) error {
	id := gkNextJobID(snap)
	body, err := curator(dir, taskID, id)
	if err != nil {
		s.usDiag("grid worker next: task %s: no curator Job for the waiting proposals: %s\n", taskID, oneLine(err))
		return nil
	}
	if body == nil {
		return nil
	}
	data, err := json.Marshal(body)
	if err == nil {
		err = contract.Validate("job-record", data)
	}
	if err != nil {
		s.usDiag("grid worker next: task %s: the curator Job record is not valid: %s\n", taskID, oneLine(err))
		return nil
	}
	_, err = s.Feed.Post(feed.Input{Type: "job", Job: id, Body: body}, s.Writer)
	return err
}

func (s Store) usDiag(format string, args ...any) {
	if s.Diagnostics != nil {
		fmt.Fprintf(s.Diagnostics, format, args...)
	}
}

// pgClosedTask is Stream.task's step for a closed task, after the task lease is
// held and before the tick. handled is true when the task is closed: the caller
// returns err and does not tick. A task that is not closed is left alone.
func (s *Stream) pgClosedTask(dir, taskID string, ts lease.TaskStore, w feed.Writer, st Store) (handled bool, err error) {
	if !pgClosedRead(dir) {
		return false, nil
	}
	closed, cerr := st.pgClosedCancel(dir, taskID, s.ClosedCurator)
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
	after, lerr := Load(ts.Feed)
	if lerr != nil {
		return true, nil
	}
	// The only open Jobs are curator Jobs: the pass goes on to the tick, which
	// assigns and spawns them like any Job. The task still holds its lease; the end
	// of task() gives it back once the last Job has closed.
	if open, curators := usOpenJobs(after); open > 0 && open == curators {
		return false, nil
	}
	// Every Job is closed now: give the lease back, as the end of task() does. A
	// Job a person posted since keeps it until the next pass cancels that too.
	if !hasOpenJob(after) {
		if rerr := ts.ReleaseTask(w); rerr != nil && !errors.Is(rerr, lease.ErrTaskFree) && !errors.Is(rerr, lease.ErrTaskHeld) {
			s.diag("grid worker next: task %s: release: %s\n", taskID, oneLine(rerr))
		}
		delete(s.handed, fmt.Sprintf("%s %d.%d", dir, w.Gen, w.Fence))
	}
	return true, nil
}
