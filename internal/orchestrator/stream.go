package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
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
	// Assigned, when set, is called with the task folder and the Job id of a spawn
	// the stream prints for the first time, just before it prints the spawn line.
	// An error is one Diag line: the spawn line is still printed and the pass goes
	// on, so a failed callback neither undoes nor blocks the assign.
	Assigned func(taskDir, job string) error
	// Model, when set, is called with the task folder and the role of a spawn the
	// stream prints for the first time; what it answers goes on the spawn line as
	// model and effort. Empty strings leave the keys out. An error is one Diag line:
	// the spawn line is still printed without the two keys, never a skipped Job.
	Model func(taskDir, role string) (model, effort string, err error)
	// Repo, when set, is called with the task folder and the repo the Job of a spawn
	// names ("" when it names none) the first time the stream prints that spawn; the
	// folder it answers goes on the spawn line as repo, and empty leaves the key out.
	// An error is one Diag line: the spawn line is still printed without the key.
	Repo func(taskDir, repo string) (string, error)
	// Probe judges whether a recorded holder process is gone; nil uses session.ProbeLocal.
	Probe func(session.Live) session.ProcessState
	// ReceiptPending and ReceiptMove move an old approval receipt of a task folder
	// into its Feed, once, so a task that is only read still gets its approve
	// person Job. The pass takes the task lease for it, so the move is a leased
	// write and the approval takes effect at once. migrate imports this package, so
	// the caller supplies them (migrate.ReceiptPending and migrate.ReceiptToJob).
	// Both nil leaves receipts to grid approve.
	ReceiptPending func(dir string) bool
	ReceiptMove    func(dir string, w feed.Writer) error
	// FirstJob, when set, makes the pass post J-1 on an open task whose Feed holds no
	// Job record at all, so a task whose first Job was never posted (a killed or failed
	// `grid task new`) still starts. It answers the J-1 record for a task of the given
	// kind, the kind written in task.md ("" when none). A closed task, a task waiting
	// on a person and a task with any Job record are left alone. Nil posts nothing.
	FirstJob func(kind string) map[string]any

	Out  io.Writer // JSON lines
	Diag io.Writer // one readable line per skip; nil discards
	Now  func() time.Time
	// Every is the pause between passes; zero is one second, as job next waits.
	Every time.Duration

	sent    map[string]bool   // task folder, Job and Job fence already printed
	skipped map[string]string // task folder to the last reason printed
	handed  map[string]bool   // task folder and task epoch whose handover is known posted
	listErr string

	ohSqMemo map[string]ohSqKey // task folder to the Feed key of the pass that dropped it as quiet
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
	// Model and Effort are the keys the worker adds to the claude command; empty
	// leaves them out and claude keeps its own default.
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	// Repo is the folder of the repo the Job works in, which the worker runs it in;
	// empty leaves it out and the worker finds the repo itself.
	Repo string `json:"repo,omitempty"`
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
	pending := s.ReceiptPending != nil && s.ReceiptMove != nil && s.ReceiptPending(dir)
	taskID, err := feed.TaskID(dir)
	if errors.Is(err, feed.ErrNoTaskID) && pending {
		// An old task with a receipt gets its Feed here, as the move needs one.
		taskID, err = feed.EnsureTaskID(dir)
	}
	if errors.Is(err, feed.ErrNoTaskID) {
		delete(s.skipped, dir) // no Feed identity yet: nothing to schedule
		return nil
	}
	if err != nil {
		return s.skip(dir, name, SkipFeedInvalid, err)
	}
	fs := feed.Store{TaskDir: dir, TaskID: taskID}
	// A task dropped as quiet on a pass whose Feed folder looked the same is dropped
	// again without a Load. The key is taken here, before Load, so a write that lands
	// after it shows in the next key.
	key, quiet := s.ohSqCheck(dir, fs, pending)
	if quiet {
		delete(s.skipped, dir)
		return nil
	}
	snap, err := Load(fs)
	if err != nil {
		return s.skip(dir, taskID, SkipFeedInvalid, err)
	}
	ts := lease.TaskStore{Feed: fs, Now: s.Now}
	if !pending {
		var done bool
		if snap, done, err = s.wlFirstJob(dir, taskID, fs, ts, snap); done || err != nil {
			return err
		}
	}
	// A task whose Jobs are all closed is left alone unless this stream still
	// holds its lease: then one more pass delivers the last wake and releases it.
	if !pending && !hasOpenJob(snap) && !s.holds(ts) {
		s.ohSqStore(dir, key, snap)
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
	if pending {
		if err := s.ReceiptMove(dir, w); err != nil {
			return s.skip(dir, taskID, SkipTickFailed, err)
		}
	}
	// Capacity is confirmed for the holder, as job next --usage-ready does for its assignee, so a Job
	// parked on usage_limit is offered again.
	st := Store{Feed: fs, Writer: w, Assignee: s.Holder(), Now: s.Now, Diagnostics: s.Diag,
		UsageReady: func(worker string) bool { return worker == w.Worker }}
	if s.Configure != nil {
		s.Configure(&st, taskID)
	}
	if closed, err := s.pgClosedTask(dir, taskID, ts, w, st); closed {
		return err
	}
	// A question that is due gets its default now, under the task lease and before
	// Store.Tick takes .orchestrator.lock, so the Jobs behind its person Job are
	// ready in this same Tick (rnQuestionDefaults says why this is the place).
	s.rnQuestionDefaults(dir, taskID, snap, w)
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
		if s.Assigned != nil {
			if err := s.Assigned(dir, sp.Job); err != nil {
				s.diag("grid worker next: task %s: job %s assigned, the Assigned callback failed: %s\n", taskID, sp.Job, oneLine(err))
			}
		}
		line := spawnLine{Task: taskID, TaskDir: dir, ID: sp.Job, Holder: sp.To, Fence: sp.Fence, Gen: sp.Gen, Attempt: sp.Attempt, Assign: sp.Assign, TaskEpoch: epoch}
		if s.Model != nil {
			var err error
			line.Model, line.Effort, err = s.Model(dir, sp.Role)
			if err != nil {
				line.Model, line.Effort = "", ""
				s.diag("grid worker next: task %s: job %s assigned, no model or effort for role %q: %s\n", taskID, sp.Job, sp.Role, oneLine(err))
			}
		}
		if s.Repo != nil {
			var err error
			line.Repo, err = s.Repo(dir, sp.Repo)
			if err != nil {
				line.Repo = ""
				s.diag("grid worker next: task %s: job %s assigned, no folder for repo %q: %s\n", taskID, sp.Job, sp.Repo, oneLine(err))
			}
		}
		if err := s.emit("spawn", line); err != nil {
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

// wlFirstJobID is the id of the Job a task starts with, as `grid task new` posts it.
const wlFirstJobID = "J-1"

// wlFirstJobKind reads task.md the way pgClosedRead does and answers the task's kind
// and whether it may get a first Job: a task whose state is closed or waiting, and a
// task.md that cannot be read, are left alone.
func wlFirstJobKind(dir string) (kind string, ok bool) {
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return "", false
	}
	fm := taskfile.ScalarFrontmatter(string(b))
	if fm == nil {
		return "", false
	}
	if state, _ := fm.Data["state"].(string); state == "closed" || state == "waiting" {
		return "", false
	}
	kind, _ = fm.Data["kind"].(string)
	return kind, true
}

// wlFirstJob posts J-1 on an open task whose Feed holds no Job record, under the task
// lease, so a task whose first Job was never posted (`grid task new` killed or failed
// between its two writes) still starts: the same pass goes on and assigns it. It
// returns the snapshot to go on with. done is true when the pass is over for the task:
// the lease is held by someone else (the task has no Job, so there is nothing to
// schedule and the next pass looks again), a skip line was printed, or the Feed holds
// Jobs and none is open. Everything else, a task with any Job record included, is
// left to the pass as it is today.
func (s *Stream) wlFirstJob(dir, taskID string, fs feed.Store, ts lease.TaskStore, snap Snapshot) (_ Snapshot, done bool, err error) {
	if s.FirstJob == nil || len(snap.State.Jobs) != 0 {
		return snap, false, nil
	}
	kind, ok := wlFirstJobKind(dir)
	if !ok {
		return snap, false, nil
	}
	body := s.FirstJob(kind)
	data, err := json.Marshal(body)
	if err == nil {
		err = contract.Validate("job-record", data)
	}
	if err != nil {
		return snap, true, s.skip(dir, taskID, SkipTickFailed, fmt.Errorf("first Job record: %w", err))
	}
	w := feed.Writer{Author: feed.Author{ID: s.Member, Role: s.Role}, Worker: s.Holder(), Lease: "task"}
	got, err := ts.AcquireTask(w, s.PID, s.PIDStart, s.probe())
	if err != nil {
		if errors.Is(err, lease.ErrTaskHeld) {
			delete(s.skipped, dir)
			return snap, true, nil
		}
		return snap, true, s.skip(dir, taskID, SkipLeaseFailed, err)
	}
	w.Gen, w.Fence = got.Tenure.Gen, got.Tenure.Fence
	release := func() {
		if rerr := ts.ReleaseTask(w); rerr != nil && !errors.Is(rerr, lease.ErrTaskFree) && !errors.Is(rerr, lease.ErrTaskHeld) {
			s.diag("grid worker next: task %s: release: %s\n", taskID, oneLine(rerr))
		}
		delete(s.handed, fmt.Sprintf("%s %d.%d", dir, w.Gen, w.Fence))
	}
	if err := s.handover(fs, w, got); err != nil {
		return snap, true, s.skip(dir, taskID, SkipLeaseFailed, err)
	}
	// Read again under the lease: another pass, or an owner's post, may have been first.
	again, err := Load(fs)
	if err != nil {
		release()
		return snap, true, s.skip(dir, taskID, SkipFeedInvalid, err)
	}
	if len(again.State.Jobs) == 0 {
		_, err := Write(fs, w, feed.Input{Type: "job", Job: wlFirstJobID, Body: body})
		switch {
		case err == nil, errors.Is(err, feed.ErrJobExists):
		case errors.Is(err, feed.ErrStaleLease):
			release()
			return snap, true, s.skip(dir, taskID, SkipTaskHeld, err)
		default:
			release()
			return snap, true, s.skip(dir, taskID, SkipTickFailed, err)
		}
		if again, err = Load(fs); err != nil {
			return snap, true, s.skip(dir, taskID, SkipFeedInvalid, err)
		}
	}
	if !hasOpenJob(again) {
		release()
		delete(s.skipped, dir)
		return again, true, nil
	}
	return again, false, nil
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
