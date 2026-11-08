package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
		snap, e := Load(s.Feed)
		if e != nil {
			return e
		}
		m, e := s.readMemory()
		if e != nil {
			return e
		}
		// Refuse graph errors before making any move.
		if _, e = Ready(snap, s.now()); e != nil {
			return e
		}
		if e := s.answers(snap); e != nil {
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
	if j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected || j.Status == contract.Posted || j.Status == contract.Ready {
		return nil
	}
	if j.Status == contract.Parked {
		if j.Reason == "usage_limit" && s.UsageReady != nil && s.UsageReady(j.Assign.To) {
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
					return s.control(j, contract.Ready, "owner requested retry", *q.Applied)
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
	if j.Status == contract.Submitted {
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
			return s.control(j, contract.Cancelled, "worker cancelled", j.Exited)
		case "done":
			return s.control(j, contract.Submitted, "awaiting judgement", j.Exited)
		default:
			failure = j.Exited
		}
	} else if j.Assign != nil {
		state := s.liveness(j)
		m.Liveness = state
		if state != "dead" {
			return nil
		}
		failure = j.AssignPost
		if j.Status != contract.Stale {
			if err := s.control(j, contract.Stale, "lease expired or process gone", j.AssignPost); err != nil {
				return err
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
	// max_rounds counts rework after the first attempt. Crash parking takes
	// precedence when the third consecutive failure hits the same boundary.
	if j.Attempt >= j.MaxRounds+1 {
		return s.control(j, contract.Rejected, "max_rounds exhausted", failure)
	}
	return s.control(j, contract.Ready, "retry after failed attempt", failure)
}
func (s Store) liveness(j *contract.Job) string {
	a := j.Assign
	path := filepath.Join(s.Feed.Dir(), ".live", fmt.Sprintf("%s.%d.json", a.To, a.Fence))
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) && j.Status == contract.Assigned {
		until, e := time.Parse(time.RFC3339Nano, a.Until)
		if e == nil && !s.now().Before(until) {
			return "dead"
		}
		return "starting"
	}

	if os.IsNotExist(err) && j.Status == contract.Running && a.To == s.Writer.Worker && s.Probe != nil {
		task, e := (lease.TaskStore{Feed: s.Feed}).Read("task")
		if e == nil && task.PID != nil && task.PIDStart != nil {
			process := s.Probe(session.Live{Lease: j.ID, Holder: a.To, Fence: uint64(a.Fence), Attempt: j.Attempt, PID: *task.PID, PIDStart: *task.PIDStart})
			if process == session.Dead {
				return "dead"
			}
			if process == session.Alive {
				return "alive"
			}
		}
	}
	if err != nil {
		return "unknown"
	}
	var live session.Live
	if json.Unmarshal(b, &live) != nil || live.Lease != j.ID || live.Holder != a.To || int64(live.Fence) != a.Fence || live.Attempt != a.Attempt {
		return "unknown"
	}
	if s.Probe != nil {
		switch s.Probe(live) {
		case session.Dead:
			return "dead"
		case session.Unknown:
			return "unknown"
		case session.Alive:
			return "alive"
		}
	}
	// A failed explicit probe above never reaches the clock fallback. After a
	// sleep, supervision must recheck identities before allowing expiry again.
	if live.Until.IsZero() || s.AfterWake {
		return "unknown"
	}
	if s.now().Before(live.Until) {
		return "alive"
	}
	return "dead"
}
