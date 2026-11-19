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
	// Tools, when set, is called the same way as Model; the tool names it answers go on
	// the spawn line as tools, in order. Nothing is answered for a role whose agent names
	// none, and the key is left out. The stream reports what the callback answers and does
	// not filter, add or cap it. An error is one Diag line: the spawn line is still
	// printed without the key, never a skipped Job.
	Tools func(taskDir, role string) ([]string, error)
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
	// ClosedCurator, when set, is asked for the curator Job of a task whose task.md
	// says state: closed, inside the pass that cancels that task's open Jobs, when no
	// curator Job of the task is open. dir is the task folder, taskID the task's Feed
	// id, id the Job id the record will be posted with. It answers the job-record body
	// of a curator Job for the brain proposals waiting for the task, or nil when none
	// waits. A body is posted under the task lease, the task is not left alone while
	// that Job is open, and the Job is assigned and spawned like any Job; every other
	// Job of the closed task is cancelled as before. An error is one Diag line and
	// posts nothing. Nil posts nothing, and a curator Job already open is still left
	// alone.
	ClosedCurator func(dir, taskID, id string) (map[string]any, error)
	// ProposalDirs, when set, answers the brain-proposals folders ClosedCurator reads for
	// the task in dir besides the task folder's own (the project's and the home's). After
	// an accepted Job of an open task finds nothing waiting, ClosedCurator is not asked
	// again until a proposal file in one of these folders, or in the task folder, changes
	// or a newer Job is accepted. It runs on every pass: it only joins paths. Nil watches
	// the task folder alone.
	ProposalDirs func(dir string) []string
	// Ticked is called with the task folder after each tick of the task that did
	// not fail: the tick is where an unleased answer to a person Job is accepted, so
	// this is what runs the step that follows it (grid land after the ship person
	// Job). w is the pass's task-lease writer, which puts the ship Job back with its
	// person when that land fails. It reports nothing back; nil does nothing.
	Ticked func(dir string, w feed.Writer)
	// LandPending, when set, reports whether the task in dir has a land to run though
	// no Job of it is open: an approved delivery whose land stopped on a conflict or
	// has not run (the step Ticked runs). Such a task is not left alone when its Jobs
	// are all closed, and the pass visits it before the other tasks, so a land that
	// waited for a rebase Job runs before another task's land moves the target again.
	// Nil leaves a task with no open Job alone, as before.
	LandPending func(dir string) bool
	// PersonWait, when set, is called with the task folder after each tick, with whether
	// every open Job of the task waits on a person (a person Job or an open question), so
	// task.md can show waiting instead of running. An error is one Diag line.
	PersonWait func(dir string, onPerson bool) error

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
	landSeen map[string]bool    // task folder to what LandPending said of it at the start of this pass

	ocCuratorMemo map[string]ohSqKey // task folder to the key of the last ask of ClosedCurator after an acceptance
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
	// Tools are the tool names the role's agent file names, as the worker may add them
	// to the claude command; empty leaves the key out.
	Tools []string `json:"tools,omitempty"`
	// Repo is the folder of the repo the Job works in, which the worker runs it in;
	// empty leaves it out and the worker finds the repo itself.
	Repo string `json:"repo,omitempty"`
	// Base is the branch the Job names for its clone or worktree; empty leaves it out.
	Base string `json:"base,omitempty"`
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
	for _, dir := range s.landFirst(dirs) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.task(dir); err != nil {
			return err
		}
	}
	return nil
}

// landFirst is dirs with the tasks whose land is pending (LandPending) first, each
// group in the order given. It asks each task once, and landPending reuses the answer
// for the rest of the pass.
func (s *Stream) landFirst(dirs []string) []string {
	s.landSeen = nil
	if s.LandPending == nil {
		return dirs
	}
	s.landSeen = map[string]bool{}
	var first, rest []string
	for _, dir := range dirs {
		pending := s.LandPending(dir)
		if abs, err := filepath.Abs(dir); err == nil {
			s.landSeen[abs] = pending
		}
		if pending {
			first = append(first, dir)
		} else {
			rest = append(rest, dir)
		}
	}
	return append(first, rest...)
}

// landPending is LandPending for dir, as landFirst found it earlier in this pass.
func (s *Stream) landPending(dir string) bool {
	if s.LandPending == nil {
		return false
	}
	if v, ok := s.landSeen[dir]; ok {
		return v
	}
	return s.LandPending(dir)
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
	// A land armed without a write to the Feed is not seen in the key.
	if quiet && s.landPending(dir) {
		quiet = false
	}
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
	if !pending && !hasOpenJob(snap) && !s.holds(ts) && !s.landPending(dir) {
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
	s.ocPostCurator(dir, taskID, st)
	if s.Ticked != nil {
		s.Ticked(dir, w)
	}
	if s.PersonWait != nil {
		if after, err := Load(fs); err == nil && hasOpenJob(after) {
			if err := s.PersonWait(dir, waitsOnPerson(after, s.now())); err != nil {
				s.diag("grid worker next: task %s: person wait: %s\n", taskID, oneLine(err))
			}
		}
	}
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
		// A Job that names its own model or effort runs on it; an empty one keeps the role's.
		if sp.Model != "" {
			line.Model = sp.Model
		}
		if sp.Effort != "" {
			line.Effort = sp.Effort
		}
		line.Base = sp.Base
		if s.Tools != nil {
			tools, err := s.Tools(dir, sp.Role)
			if err != nil {
				s.diag("grid worker next: task %s: job %s assigned, no tools for role %q: %s\n", taskID, sp.Job, sp.Role, oneLine(err))
			} else {
				line.Tools = tools
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
	// J-1 carries the writer's machine as home, as `grid task new` sets it; a body with home keeps it.
	if _, has := body["home"]; !has && s.Machine != "" && body != nil {
		body["home"] = s.Machine
	}
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

// waitsOnPerson reports whether the task has open Jobs and each waits on a person: a
// person Job, a Job with an open question, or a Job not yet ready whose every wait
// (the graph's own, as Ready reports it) is on such a Job. Nothing an agent could do next.
func waitsOnPerson(snap Snapshot, now time.Time) bool {
	r, err := Ready(snap, now)
	if err != nil {
		return false
	}
	ready := map[string]bool{}
	for _, id := range r.Ready {
		ready[id] = true
	}
	waits := map[string][]string{}
	for _, w := range r.Waiting {
		waits[w.Job] = append(waits[w.Job], w.Target)
	}
	memo := map[string]bool{}
	var person func(id string) bool
	person = func(id string) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		memo[id] = false // a cycle is not a person's wait
		j := snap.State.Jobs[id]
		v := false
		switch {
		case j == nil:
		case j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected:
			v = true // settled: nothing to wait for
		case contract.NormalizeRole(j.Role) == "person" || len(j.OpenQuestions) > 0:
			v = true
		case ready[id] || (j.Status != contract.Posted && j.Status != contract.Ready) || len(waits[id]) == 0:
		default:
			v = true
			for _, t := range waits[id] {
				if t == "" || !person(t) {
					v = false
					break
				}
			}
		}
		memo[id] = v
		return v
	}
	open := false
	for id, j := range snap.State.Jobs {
		switch j.Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
			continue
		}
		open = true
		if !person(id) {
			return false
		}
	}
	return open
}

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
