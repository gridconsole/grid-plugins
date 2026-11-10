package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// Skip reasons are fixed codes: a reader branches on them, so they never carry
// free text. The readable cause goes to Diag.
const (
	SkipTaskHeld     = "task_held"
	SkipFeedInvalid  = "feed_invalid"
	SkipGraphInvalid = "graph_invalid"
	SkipLeaseFailed  = "lease_failed"
	SkipTickFailed   = "tick_failed"
)

// Stream is one long-lived supervisor for every task in a Grid home. Each pass
// takes (or reuses, or adopts) the task lease for the supervisor's process and
// runs a Tick as that holder, so every Job a pass assigns is assigned to the
// holder itself. It prints one JSON object per line on Out:
//
//	{"spawn":{...}}  a Job in Assigned, once per (task, Job, Job fence)
//	{"wake":{...}}   the wake a Tick raised
//	{"skip":{...}}   a task the pass could not work on, once per change of reason
//
// The tenure names PID and PIDStart, the supervisor's own process, not the
// process that runs the stream. A restarted stream therefore reuses the
// tenure, and only a dead supervisor can be adopted. The stream adds no cap,
// delay or rate hold on Jobs.
type Stream struct {
	Member   string // the holder's member id, without @machine
	Machine  string // this machine's id
	Role     string // the role recorded on the task lease writer
	PID      int
	PIDStart string // RFC3339 as contract.FormatTime writes it

	// Tasks lists the task folders of the home, each an absolute path.
	Tasks func() ([]string, error)
	// Configure sets the per-task Store fields the caller owns (worktrees,
	// liveness probe, incident sink, owner); st.Feed, st.Writer and st.Assignee are set.
	Configure func(st *Store, taskID string)
	// Probe judges whether a recorded holder process is gone; nil uses session.ProbeLocal.
	Probe func(session.Live) session.ProcessState

	Out  io.Writer // JSON lines
	Diag io.Writer // one readable line per skip; nil discards
	Now  func() time.Time
	// Every is the pause between passes; zero is one second, as job next waits.
	Every time.Duration

	sent    map[string]bool   // task folder, Job and Job fence already printed
	skipped map[string]string // task folder to the last reason printed
	handed  map[string]bool   // task folder and task epoch whose handover is known posted
	listErr string
}

type spawnLine struct {
	Task      string `json:"task"`
	TaskDir   string `json:"task_dir"`
	ID        string `json:"id"`
	Holder    string `json:"holder"`
	Fence     int64  `json:"fence"`
	Gen       int64  `json:"gen"`
	Attempt   int    `json:"attempt"`
	Assign    string `json:"assign"`
	TaskEpoch string `json:"task_epoch"`
}
type wakeLine struct {
	Task   string   `json:"task"`
	Job    string   `json:"job"`
	Event  string   `json:"event"`
	N      int      `json:"n"`
	Events []string `json:"events,omitempty"`
}
type skipLine struct {
	Task   string `json:"task"`
	Reason string `json:"reason"`
}

// Holder is the worker id the stream holds task leases and assigns Jobs as.
func (s *Stream) Holder() string { return s.Member + "@" + s.Machine }

func (s *Stream) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Run passes over the home until ctx ends, then returns ctx.Err(). It returns
// another error only when Out cannot be written. It does not release a task
// lease when it ends: the tenure belongs to the supervisor's process, and the
// next stream of that process reuses it.
func (s *Stream) Run(ctx context.Context) error {
	every := s.Every
	if every <= 0 {
		every = time.Second
	}
	for {
		if err := s.Pass(ctx); err != nil {
			return err
		}
		timer := time.NewTimer(every)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Pass works through every task once. An error in one task is a skip line and
// the pass goes on; Pass fails only when ctx ends or Out cannot be written.
func (s *Stream) Pass(ctx context.Context) error {
	dirs, err := s.Tasks()
	if err != nil {
		// The listing may be back next pass; say it once per change.
		if msg := oneLine(err); msg != s.listErr {
			s.listErr = msg
			s.diag("grid worker next: cannot list tasks: %s\n", msg)
		}
		return nil
	}
	s.listErr = ""
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.task(dir); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stream) task(dir string) error {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if s.skipped == nil {
		s.skipped, s.sent, s.handed = map[string]string{}, map[string]bool{}, map[string]bool{}
	}
	name := filepath.Base(dir)
	taskID, err := feed.TaskID(dir)
	if errors.Is(err, feed.ErrNoTaskID) {
		delete(s.skipped, dir) // no Feed identity yet: nothing to schedule
		return nil
	}
	if err != nil {
		return s.skip(dir, name, SkipFeedInvalid, err)
	}
	fs := feed.Store{TaskDir: dir, TaskID: taskID}
	snap, err := Load(fs)
	if err != nil {
		return s.skip(dir, taskID, SkipFeedInvalid, err)
	}
	ts := lease.TaskStore{Feed: fs, Now: s.Now}
	// A task whose Jobs are all closed is left alone unless this stream still
	// holds its lease: then one more pass delivers the last wake and releases it.
	if !hasOpenJob(snap) && !s.holds(ts) {
		delete(s.skipped, dir)
		return nil
	}
	if _, err := Ready(snap, s.now()); err != nil {
		return s.skip(dir, taskID, SkipGraphInvalid, err)
	}
	w := feed.Writer{Author: feed.Author{ID: s.Member, Role: s.Role}, Worker: s.Holder(), Lease: "task"}
	got, err := ts.AcquireTask(w, s.PID, s.PIDStart, s.probe())
	if err != nil {
		if errors.Is(err, lease.ErrTaskHeld) {
			return s.skip(dir, taskID, SkipTaskHeld, err)
		}
		return s.skip(dir, taskID, SkipLeaseFailed, err)
	}
	w.Gen, w.Fence = got.Tenure.Gen, got.Tenure.Fence
	if err := s.handover(fs, w, got); err != nil {
		return s.skip(dir, taskID, SkipLeaseFailed, err)
	}
	st := Store{Feed: fs, Writer: w, Assignee: s.Holder(), Now: s.Now, Diagnostics: s.Diag}
	if s.Configure != nil {
		s.Configure(&st, taskID)
	}
	out, err := st.Tick()
	if err != nil {
		if errors.Is(err, feed.ErrStaleLease) {
			return s.skip(dir, taskID, SkipTaskHeld, err)
		}
		return s.skip(dir, taskID, SkipTickFailed, err)
	}
	delete(s.skipped, dir)
	epoch := fmt.Sprintf("%d.%d", w.Gen, w.Fence)
	for _, sp := range out.Spawns {
		if sp.To != w.Worker {
			continue // assigned to another worker: its own supervisor launches it
		}
		key := fmt.Sprintf("%s\x00%s\x00%d", dir, sp.Job, sp.Fence)
		if s.sent[key] {
			continue
		}
		if err := s.emit("spawn", spawnLine{taskID, dir, sp.Job, sp.To, sp.Fence, sp.Gen, sp.Attempt, sp.Assign, epoch}); err != nil {
			return err
		}
		s.sent[key] = true
	}
	if out.Wake != nil {
		if err := s.emit("wake", wakeLine{taskID, out.Wake.Job, out.Wake.Event, out.Wake.N, out.Wake.Events}); err != nil {
			return err
		}
		return nil
	}
	// The last Job closed and its wake is out: give the lease back, so a person
	// can post the next Job or take the task. The next open Job takes it again.
	if after, err := Load(fs); err == nil && !hasOpenJob(after) {
		if err := ts.ReleaseTask(w); err != nil && !errors.Is(err, lease.ErrTaskFree) && !errors.Is(err, lease.ErrTaskHeld) {
			s.diag("grid worker next: task %s: release: %s\n", taskID, oneLine(err))
		}
		delete(s.handed, fmt.Sprintf("%s %s", dir, epoch))
	}
	return nil
}

func (s *Stream) probe() func(session.Live) session.ProcessState {
	if s.Probe != nil {
		return s.Probe
	}
	return session.ProbeLocal
}

// holds reports whether the lease file names this holder and supervisor process.
func (s *Stream) holds(ts lease.TaskStore) bool {
	t, err := ts.Read("task")
	return err == nil && t.Holder != nil && *t.Holder == s.Holder() && t.PID != nil && *t.PID == s.PID && t.PIDStart != nil && *t.PIDStart == s.PIDStart
}

// handover posts the handover record of a tenure this pass just got, and of a
// reused tenure whose record is missing (a crash between grant and post).
func (s *Stream) handover(fs feed.Store, w feed.Writer, got lease.Acquisition) error {
	key := fmt.Sprintf("%s %d.%d", fs.TaskDir, w.Gen, w.Fence)
	if got.How == lease.AcquiredReused {
		if s.handed[key] {
			return nil
		}
		read, err := fs.Read(feed.Filter{Type: "handover"})
		if err != nil {
			return err
		}
		for _, r := range read.Records {
			if r.Worker == w.Worker && r.Gen == w.Gen && r.Fence == w.Fence && r.Verdict == "counted" {
				s.handed[key] = true
				return nil
			}
		}
	}
	body := map[string]any{}
	if got.How == lease.AcquiredAdopted && got.From.Holder != nil {
		body = map[string]any{"adopted": "session-gone", "from": *got.From.Holder}
	}
	if _, err := fs.Handover(feed.Input{Body: body}, w); err != nil {
		return err
	}
	s.handed[key] = true
	return nil
}

func (s *Stream) skip(dir, taskID, reason string, cause error) error {
	if s.skipped[dir] == reason {
		return nil
	}
	s.skipped[dir] = reason
	s.diag("grid worker next: task %s skipped (%s): %s\n", taskID, reason, oneLine(cause))
	return s.emit("skip", skipLine{taskID, reason})
}

// emit writes one line with a single Write, so lines never interleave.
func (s *Stream) emit(kind string, v any) error {
	b, err := json.Marshal(map[string]any{kind: v})
	if err != nil {
		return err
	}
	_, err = s.Out.Write(append(b, '\n'))
	return err
}

func (s *Stream) diag(format string, args ...any) {
	if s.Diag != nil {
		fmt.Fprintf(s.Diag, format, args...)
	}
}

func oneLine(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }

func hasOpenJob(snap Snapshot) bool {
	for _, j := range snap.State.Jobs {
		switch j.Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
		default:
			return true
		}
	}
	return false
}
