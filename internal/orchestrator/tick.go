package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// Tick makes one serialized pass. A second process folds the first process's
// assigns before computing Ready, even when both use the same task tenure.
func (s Store) Tick() (out Output, err error) {
	out.Rows = []Row{}
	err = s.locked(func() error {
		if s.Writer.Lease != "task" {
			return feed.ErrStaleLease
		}
		// Reuse one policy for every projection in this pass, including the
		// saved state. The value receiver keeps it local to this tick.
		if s.Feed.WithdrawAllowed == nil {
			s.Feed.WithdrawAllowed = feed.QuestionWithdrawReadPolicy(s.Feed.TaskDir, s.Feed.Warn)
		}
		snap, e := Load(s.Feed)
		if e != nil {
			return e
		}
		m, e := s.readMemory()
		if e != nil {
			return e
		}
		if e := s.publishPlan(snap, &m); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		s.taskProgress, e = s.fileProgress(&m)
		if e != nil {
			return e
		}
		s.progressMemo = map[string]string{}
		// Refuse graph errors before making any move.
		if _, e = Ready(snap, s.now()); e != nil {
			return e
		}
		if e := s.warnDuplicateGates(snap); e != nil {
			return e
		}
		if e := s.answers(snap); e != nil {
			return e
		}
		if e := s.personAnswers(snap); e != nil {
			return e
		}
		// personAnswers may just have closed a person Job that a cancel request names.
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.personCancels(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		for _, id := range ids(snap) {
			j := snap.State.Jobs[id]
			if m.Jobs[id] == nil {
				m.Jobs[id] = s.recoverMemory(snap, j)
			}
			if e := s.advance(snap, j, m.Jobs[id], &m); e != nil {
				return e
			}
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.gkReplan(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.guReplanRejected(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.guReplanAskOwner(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.guReplanPersonCancelled(snap); e != nil {
			return e
		}
		if e := s.taCancelEndGrants(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.pfMsgReplan(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.kbSecurityFound(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.abDecisionsNeeded(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		if e := s.proposals(snap); e != nil {
			return e
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		to := s.Assignee
		if to == "" {
			to = s.Writer.Worker
		}
		snap.Assignee = to
		ready, e := Ready(snap, s.now())
		if e != nil {
			return e
		}
		if e := s.noteStartedDependents(snap, ready.Notes); e != nil {
			return e
		}
		waits := map[string]string{}
		for _, w := range ready.Waiting {
			if waits[w.Job] == "" {
				waits[w.Job] = w.Reason
			}
		}
		for _, id := range ids(snap) {
			if why := waits[id]; why != "" && snap.State.Jobs[id].Reason != why {
				if e := s.control(snap.State.Jobs[id], "", why, ""); e != nil {
					return e
				}
			}
		}
		if e := s.saveMemory(m); e != nil {
			return e
		}
		for _, id := range ready.Ready {
			j := snap.State.Jobs[id]
			if m.OfflineHosts[workerHost(to)] {
				if j.Reason != "host_offline" {
					if e := s.control(j, "", "host_offline", ""); e != nil {
						return e
					}
				}
				continue
			}
			if e := s.assign(j, to); e != nil {
				return e
			}
			if m.Jobs[id] == nil {
				m.Jobs[id] = s.recoverMemory(snap, j)
			}
			m.Jobs[id].Liveness = "starting"
			out.Assigned++
		}
		snap, e = Load(s.Feed)
		if e != nil {
			return e
		}
		for _, id := range ids(snap) {
			j := snap.State.Jobs[id]
			if j.Status == contract.Assigned {
				if e := s.spawn(j); e != nil {
					return e
				}
				if j.Assign != nil {
					out.Spawns = append(out.Spawns, Spawn{j.ID, j.Assign.Attempt, j.Assign.To, j.Assign.Fence, j.Generation, j.AssignPost, j.Role, j.Repo, j.Model, j.Effort, j.Base})
				}
			} else if e := os.Remove(filepath.Join(s.Feed.TaskDir, "spawned", j.ID+".json")); e != nil && !errors.Is(e, os.ErrNotExist) {
				return e
			}
			why := j.Reason
			if why == "" {
				switch j.Status {
				case contract.Assigned:
					why = "starting"
				case contract.Running:
					why = "running"
				default:
					why = string(j.Status)
				}
			}
			out.Rows = append(out.Rows, Row{j.ID, j.Role, j.Status, why})
		}
		out.Wake = s.wake(snap, &m)
		if e := s.saveMemory(m); e != nil {
			return e
		}
		live := map[string]string{}
		for id, j := range m.Jobs {
			if j.Liveness != "" {
				live[id] = j.Liveness
			}
		}
		_, e = s.Feed.SaveStateLiveness(live)
		return e
	})
	return
}
func (s Store) assign(j *contract.Job, to string) error {
	attempt := j.Attempt + 1
	if j.ExitReason == "usage_limit" {
		attempt = j.Attempt
	}
	if j.Status != contract.Ready {
		if err := s.control(j, contract.Ready, "ready", ""); err != nil {
			return err
		}
	}
	grant, err := (lease.TaskStore{Feed: s.Feed, Now: s.Now}).GrantJob(s.Writer, j.ID, to, attempt)
	if err != nil {
		return err
	}
	a := contract.Assignment{Job: j.ID, Attempt: attempt, To: to, Fence: grant.Fence, Until: contract.FormatTime(s.now().Add(session.LeaseDuration))}
	_, err = s.Feed.Post(feed.Input{Type: "control", Body: map[string]any{"assign": a}}, s.Writer)
	return err
}
func (s Store) spawn(j *contract.Job) error {
	dir := filepath.Join(s.Feed.TaskDir, "spawned")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	request := struct {
		*contract.Assignment
		Gen   int64    `json:"gen"`
		After []string `json:"after"`
	}{j.Assign, j.Generation, []string{j.AssignPost}}
	path := filepath.Join(dir, j.ID+".json")
	b, err := os.ReadFile(path)
	want, _ := json.Marshal(request)
	var old any
	if err == nil && json.Unmarshal(b, &old) == nil {
		var expected any
		_ = json.Unmarshal(want, &expected)
		if reflect.DeepEqual(old, expected) {
			return nil
		}
	}
	return writeJSON(path, request)
}
func (s Store) advance(snap Snapshot, j *contract.Job, m *jobMemory, all *memory) error {
	// A worker stop puts the Job to Ready itself (the app worker posts the control
	// re the exit), so the cancelled arm below never sees it. The stop is not a
	// failure and clears the count, as that arm does.
	if j.Status == contract.Ready && j.ExitReason == "cancelled" && ycWorkerStopped(j.ExitCode) {
		m.Failures = nil
	}
	if j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected || j.Status == contract.Posted || j.Status == contract.Ready || j.Status == contract.Blocked {
		// Blocked waits for a person; the tick posts no control over it, whatever the exit reason.
		return nil
	}
	if j.Status == contract.Parked {
		if j.Reason == "usage_limit" && s.yaUsageReady(j) {
			return s.control(j, contract.Ready, "usage capacity restored", j.Exited)
		}
		if j.Reason == "needs_person" {
			for id, q := range snap.State.Questions {
				r := snap.Records[id]
				if r.Job == nil || *r.Job != j.ID || r.Wake == nil || *r.Wake != m.Failure+"/1" || q.Applied == nil {
					continue
				}
				if snap.Records[*q.Applied].Body["answer"] == "Retry" {
					m.Failures = nil
					return s.control(j, contract.Ready, ownerRetryReason, *q.Applied)
				}
			}
		}
		return nil
	}
	progress := s.progress(snap, j)
	if m.Progress != "" && m.Progress != progress {
		m.Failures = nil
	}
	m.Progress = progress
	// A Submitted Job whose attempt ended with no result is one an older tick moved here: it
	// takes the retry below, so a task stuck that way goes on by itself.
	if j.Status == contract.Submitted && !emptyFinish(snap, j) {
		return nil
	}
	failure := ""
	if j.Exited != "" {
		switch j.ExitReason {
		case "usage_limit":
			m.Failures = nil
			return s.control(j, contract.Parked, "usage_limit", j.Exited)
		case "cancelled":
			m.Failures = nil
			if ycWorkerStopped(j.ExitCode) {
				return s.control(j, contract.Ready, "worker stopped", j.Exited)
			}
			return s.control(j, contract.Cancelled, "worker cancelled", j.Exited)
		case "done":
			// A done exit with a result of its attempt folds Submitted itself and
			// returned above. One with no result is an empty finish: retried, then parked.
			if emptyFinish(snap, j) {
				return s.retryEmptyFinish(snap, j, m, all)
			}
			return s.control(j, contract.Submitted, "awaiting judgement", j.Exited)
		default:
			failure = j.Exited
		}
	} else if j.Assign != nil {
		state := s.liveness(j, m)
		if state == "alive" {
			delete(all.OfflineHosts, workerHost(j.Assign.To))
		}
		m.Liveness = state
		if state != "dead" && state != "host_offline" {
			return nil
		}
		failure = j.AssignPost
		if j.Status != contract.Stale {
			reason := "lease expired or process gone"
			if state == "host_offline" {
				reason = state
			}
			if err := s.control(j, contract.Stale, reason, j.AssignPost); err != nil {
				return err
			}
		}
		if state == "host_offline" {
			if all.OfflineHosts == nil {
				all.OfflineHosts = map[string]bool{}
			}
			all.OfflineHosts[workerHost(j.Assign.To)] = true
			to := s.Assignee
			if to == "" {
				to = s.Writer.Worker
			}
			if workerHost(to) == workerHost(j.Assign.To) {
				return nil
			}
		}
	}
	if failure == "" {
		return nil
	}
	if m.Failure != failure {
		m.Failures = append(m.Failures, failure)
		m.Failure = failure
	}
	if len(m.Failures) >= 3 {
		m.ParkProgress = progress
	}
	if err := s.saveMemory(*all); err != nil {
		return err
	}
	if len(m.Failures) >= 3 {
		if err := s.park(snap, j, m); err != nil {
			return err
		}
		m.ParkProgress = s.progress(snap, j)
		return nil
	}
	return s.control(j, contract.Ready, "retry after failed attempt", failure)
}
func workerHost(worker string) string {
	return worker[strings.LastIndexByte(worker, '@')+1:]
}

func (s Store) liveness(j *contract.Job, m *jobMemory) string {
	a := j.Assign
	until, _ := time.Parse(time.RFC3339Nano, a.Until)
	if m.ObservedFence != a.Fence {
		m.ObservedFence = a.Fence
		m.LastCheck = until.Add(-session.LeaseDuration)
		m.WakePending = false
	}
	if s.AfterWake {
		m.WakePending = true
	}
	unknown := func() string {
		if !m.WakePending && !until.IsZero() && !s.now().Before(until) && !m.LastCheck.IsZero() && s.now().Sub(m.LastCheck) >= 2*session.LeaseDuration {
			return "host_offline"
		}
		return "unknown"
	}
	// supervisor: this worker holds the Job and its process (the task lease pid) is alive.
	supervisor := func() bool {
		if a.To != s.Writer.Worker || s.Probe == nil {
			return false
		}
		task, e := (lease.TaskStore{Feed: s.Feed}).Read("task")
		return e == nil && task.PID != nil && task.PIDStart != nil && s.Probe(session.Live{Lease: j.ID, Holder: a.To, Fence: uint64(a.Fence), Attempt: j.Attempt, PID: *task.PID, PIDStart: *task.PIDStart}) == session.Alive
	}
	observed := func(process session.ProcessState, heartbeat bool) string {
		switch process {
		case session.Dead:
			m.LastCheck = s.now()
			m.WakePending = false
			// The worker that supervises the agent reports its end with a worker-exit, the
			// evidence of how it ended. While that worker lives and the beat is valid, a dead
			// agent is not yet a failed attempt (its end is unknown until the exit arrives):
			// retrying now would make that exit stale.
			if heartbeat && s.now().Before(until) && supervisor() {
				return "unknown"
			}
			return "dead"
		case session.Alive:
			m.LastCheck = s.now()
			if m.WakePending {
				if !heartbeat || s.now().Before(until) {
					m.WakePending = false
				}
				return "alive"
			}
			if heartbeat && !until.IsZero() && !s.now().Before(until) {
				return "dead"
			}
			return "alive"
		default:
			return unknown()
		}
	}
	path := filepath.Join(s.Feed.Dir(), ".live", fmt.Sprintf("%s.%d.json", a.To, a.Fence))
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) && j.Status == contract.Assigned {
		if m.WakePending {
			return "unknown"
		}
		if !until.IsZero() && !s.now().Before(until) {
			return "dead"
		}
		return "starting"
	}
	if os.IsNotExist(err) && j.Status == contract.Running && a.To == s.Writer.Worker && s.Probe != nil {
		task, e := (lease.TaskStore{Feed: s.Feed}).Read("task")
		if e == nil && task.PID != nil && task.PIDStart != nil {
			return observed(s.Probe(session.Live{Lease: j.ID, Holder: a.To, Fence: uint64(a.Fence), Attempt: j.Attempt, PID: *task.PID, PIDStart: *task.PIDStart}), false)
		}
	}
	if err != nil {
		return unknown()
	}
	var live session.Live
	if json.Unmarshal(b, &live) != nil || live.Lease != j.ID || live.Holder != a.To || int64(live.Fence) != a.Fence || live.Attempt != a.Attempt {
		return unknown()
	}
	if !live.Until.IsZero() {
		until = live.Until
	}
	if s.Probe != nil {
		return observed(s.Probe(live), true)
	}
	if live.Until.IsZero() || m.WakePending {
		return unknown()
	}
	if s.now().Before(live.Until) {
		return "alive"
	}
	return "dead"
}
