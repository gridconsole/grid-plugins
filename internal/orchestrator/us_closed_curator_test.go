package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// usClosedBody is the job-record of a curator Job with the given id: the valid
// fixture Job, role curator. The real body is internal/cli's curatorJobBody; the
// stream needs only a valid record.
func usClosedBody(t *testing.T, id string) map[string]any {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "curator"
	body["title"] = "Curate 1 brain proposal"
	return body
}

// usClosedHook is a ClosedCurator that answers a curator body while waiting is true
// and nil after, and writes down the arguments of every call. It is safe for the
// goroutines of the two-stream test.
type usClosedHook struct {
	t       *testing.T
	mu      sync.Mutex
	waiting bool
	calls   [][3]string
}

func newUsClosedHook(t *testing.T) *usClosedHook { return &usClosedHook{t: t, waiting: true} }

func (k *usClosedHook) ask(dir, taskID, id string) (map[string]any, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls = append(k.calls, [3]string{dir, taskID, id})
	if !k.waiting {
		return nil, nil
	}
	return usClosedBody(k.t, id), nil
}

// filed says the proposals are settled: later calls answer nil.
func (k *usClosedHook) filed() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.waiting = false
}

func (k *usClosedHook) asked() [][3]string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([][3]string(nil), k.calls...)
}

// usClosedStream is a stream of one supervisor pid with its own output buffers, so
// two of them may pass at once. hook may be nil.
func usClosedStream(h *streamHome, pid int, probe func(session.Live) session.ProcessState, hook func(dir, taskID, id string) (map[string]any, error)) (*Stream, *bytes.Buffer) {
	diag := &bytes.Buffer{}
	return &Stream{
		Member: "worker", Machine: machine, Role: "cli", PID: pid, PIDStart: streamStart,
		Tasks: func() ([]string, error) { return h.dirs, nil }, Probe: probe,
		Out: &bytes.Buffer{}, Diag: diag, ClosedCurator: hook,
	}, diag
}

// usClosedCurators is the job records of task i whose body role is curator.
func usClosedCurators(t *testing.T, h *streamHome, i int) []feed.Entry {
	t.Helper()
	var out []feed.Entry
	for _, r := range pgClosedOfType(t, h, i, "job") {
		if role, _ := r.Body["role"].(string); strings.EqualFold(role, "curator") {
			out = append(out, r)
		}
	}
	return out
}

// usClosedAssigns counts the assign records written for a Job.
func usClosedAssigns(t *testing.T, h *streamHome, i int, id string) int {
	t.Helper()
	n := 0
	for _, r := range pgClosedOfType(t, h, i, "control") {
		if a, ok := r.Body["assign"].(map[string]any); ok && a["job"] == id {
			n++
		}
	}
	return n
}

// usClosedCancelledFor lists the close reasons of the cancel controls aimed at a Job.
func usClosedCancelledFor(t *testing.T, h *streamHome, i int, id string) []string {
	t.Helper()
	var out []string
	for _, r := range pgClosedOfType(t, h, i, "control") {
		if r.Body["target"] == id && r.Body["status"] == string(contract.Cancelled) {
			reason, _ := r.Body["close_reason"].(string)
			out = append(out, reason)
		}
	}
	return out
}

// Row 1: a closed task keeps the curator Job it holds open and assigns it; the worker Job beside it is
// cancelled with the stop reason; nothing else is posted and nothing is re-planned.
func TestUsClosedCuratorJobIsKeptAndTheWorkerJobCancelled(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "curator", "builder")
	pgClosedSetState(t, h, 0, "closed")
	s := h.stream(4242, nil)
	h.pass(s)

	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Assigned {
		t.Errorf("the curator Job J-1 is %s, want %s", got, contract.Assigned)
	}
	if got := usClosedCancelledFor(t, h, 0, "J-1"); len(got) != 0 {
		t.Errorf("the curator Job was cancelled: %v", got)
	}
	if got := pgClosedStatus(t, h, 0, "J-2"); got != contract.Cancelled {
		t.Errorf("the worker Job J-2 is %s, want cancelled", got)
	}
	if got := usClosedCancelledFor(t, h, 0, "J-2"); len(got) != 1 || got[0] != "task stopped" {
		t.Errorf("worker Job cancel reasons = %v, want [task stopped]", got)
	}
	sp := kinds(h.lines(), "spawn")
	if len(sp) != 1 || sp[0]["id"] != "J-1" {
		t.Errorf("want one spawn line, for the curator Job J-1, got %v", sp)
	}
	pgClosedNoReplan(t, h, 0, "J-1", "J-2")

	// Later passes leave both as they are: the curator Job stays, the worker Job stays cancelled.
	before := len(pgClosedRecords(t, h, 0))
	h.pass(s)
	h.pass(h.stream(4242, nil))
	if after := len(pgClosedRecords(t, h, 0)); after != before {
		t.Errorf("later passes wrote %d records", after-before)
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Assigned {
		t.Errorf("after later passes the curator Job is %s", got)
	}
}

// Row 2: a declined task with an open J-1 and a hook that answers a body gets one curator Job, posted under
// the task lease and assigned in the same pass; J-1 is cancelled; a second and third pass add none.
func TestUsClosedDeclinedTaskGetsOneCuratorJob(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	pgClosedSetState(t, h, 0, "closed")
	jobsBefore := len(pgClosedOfType(t, h, 0, "job"))

	h.pass(s)
	jobs := pgClosedOfType(t, h, 0, "job")
	if len(jobs) != jobsBefore+1 {
		t.Fatalf("want one new job record, the Feed holds %d after %d", len(jobs), jobsBefore)
	}
	curators := usClosedCurators(t, h, 0)
	if len(curators) != 1 || curators[0].Job == nil || *curators[0].Job != "J-2" {
		t.Fatalf("want one curator Job J-2 on the Feed, got %v", curators)
	}
	if curators[0].Author.Role != "cli" {
		t.Errorf("the curator Job was posted by role %q, want the stream's role cli", curators[0].Author.Role)
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Errorf("J-1 is %s, want cancelled", got)
	}
	if got := usClosedCancelledFor(t, h, 0, "J-1"); len(got) != 1 || got[0] != "task stopped" {
		t.Errorf("J-1 cancel reasons = %v, want [task stopped]", got)
	}
	if got := pgClosedStatus(t, h, 0, "J-2"); got != contract.Assigned {
		t.Errorf("the curator Job J-2 is %s, want %s", got, contract.Assigned)
	}
	if sp := kinds(h.lines(), "spawn"); len(sp) != 1 || sp[0]["id"] != "J-2" {
		t.Errorf("want one spawn line for J-2, got %v", sp)
	}
	pgClosedNoReplan(t, h, 0, "J-1", "J-2")
	want := [3]string{h.dirs[0], h.ids[0], "J-2"}
	if got := hook.asked(); len(got) != 1 || got[0] != want {
		t.Errorf("hook calls = %v, want one call %v (task folder, Feed id, next Job id)", got, want)
	}

	records := len(pgClosedRecords(t, h, 0))
	h.pass(s)
	h.pass(s)
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Errorf("after three passes the Feed holds %d curator Jobs, want 1", got)
	}
	if after := len(pgClosedRecords(t, h, 0)); after != records {
		t.Errorf("passes two and three wrote %d records", after-records)
	}
	if got := len(hook.asked()); got != 1 {
		t.Errorf("the hook was asked %d times; with a curator Job open it is not asked again", got)
	}
	if got := usClosedAssigns(t, h, 0, "J-2"); got != 1 {
		t.Errorf("the curator Job was assigned %d times, want 1", got)
	}
}

// Row 4 (hook side): a closed task whose hook answers nil, errors, or answers a record the Job schema
// refuses behaves as today: the open Job is cancelled, nothing spawns, nothing is posted and the lease is free.
func TestUsClosedNoCuratorBodyBehavesAsToday(t *testing.T) {
	invalid := func(string, string, string) (map[string]any, error) { return map[string]any{"id": "J-2", "role": "curator"}, nil }
	for name, tc := range map[string]struct {
		hook func(dir, taskID, id string) (map[string]any, error)
		diag string
	}{
		"no hook":     {nil, ""},
		"answers nil": {func(string, string, string) (map[string]any, error) { return nil, nil }, ""},
		"errors":      {func(string, string, string) (map[string]any, error) { return nil, errors.New("brain unreadable") }, "brain unreadable"},
		"bad record":  {invalid, "not valid"},
	} {
		t.Run(name, func(t *testing.T) {
			h := newStreamHome(t, 1)
			h.post(0, "J-1")
			pgClosedSetState(t, h, 0, "closed")
			s, diag := usClosedStream(h, 4242, nil, tc.hook)
			if err := s.Pass(context.Background()); err != nil {
				t.Fatal(err)
			}
			if sp := strings.Contains(s.Out.(*bytes.Buffer).String(), `"spawn"`); sp {
				t.Errorf("a closed task printed a spawn line: %s", s.Out.(*bytes.Buffer))
			}
			if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
				t.Errorf("J-1 is %s, want cancelled", got)
			}
			if got := len(usClosedCurators(t, h, 0)); got != 0 {
				t.Errorf("a curator Job was posted: %d", got)
			}
			pgClosedNoReplan(t, h, 0, "J-1")
			pgClosedLeaseFree(t, h, 0)
			if tc.diag != "" && !strings.Contains(diag.String(), tc.diag) {
				t.Errorf("diag %q does not say %q", diag.String(), tc.diag)
			}
		})
	}
}

// Row 2 and the closed-task rule: while the curator Job is open, every other Job of the closed task is still
// cancelled, one posted after the close included, and the curator Job is not.
func TestUsClosedOtherJobsStayCancelledBesideTheCuratorJob(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s) // J-1 cancelled, curator Job J-2 posted and assigned
	h.lines()

	pgClosedHolderPost(t, h, 0, "J-3", nil) // a Job posted into the closed task by the lease holder
	h.pass(s)
	if got := pgClosedStatus(t, h, 0, "J-3"); got != contract.Cancelled {
		t.Errorf("J-3 is %s, want cancelled", got)
	}
	if got := pgClosedStatus(t, h, 0, "J-2"); got != contract.Assigned {
		t.Errorf("the curator Job J-2 is %s, want %s", got, contract.Assigned)
	}
	if sp := kinds(h.lines(), "spawn"); len(sp) != 0 {
		t.Errorf("the second pass printed a spawn line for a Job it had printed: %v", sp)
	}
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Errorf("curator Jobs on the Feed = %d, want 1", got)
	}
	pgClosedNoReplan(t, h, 0, "J-1", "J-2", "J-3")
}

// The curator Job is the only open Job and it closes: the next pass asks again, the proposals are filed so
// the hook answers nil, and the lease is given back as for any closed task.
func TestUsClosedLeaseIsFreeOnceTheCuratorJobCloses(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	if tn := h.tenure(0); tn.Holder == nil {
		t.Fatal("the stream gave the lease back with the curator Job open")
	}

	// The keeper files the proposals and its Job closes: a cancel under the holder's lease stands in for the accept.
	hook.filed()
	tn := h.tenure(0)
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + machine, Lease: "task", Gen: tn.Gen, Fence: tn.Fence}
	body := map[string]any{"target": "J-2", "status": "cancelled", "reason": "done", "close_reason": "done"}
	if _, err := h.store(0).Post(feed.Input{Type: "control", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	h.pass(s)
	pgClosedLeaseFree(t, h, 0)
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Errorf("curator Jobs on the Feed = %d, want 1", got)
	}
	if got := len(hook.asked()); got != 2 {
		t.Errorf("hook calls = %d, want 2 (the pass that posted, the pass after the close)", got)
	}
	records := len(pgClosedRecords(t, h, 0))
	h.pass(s)
	if after := len(pgClosedRecords(t, h, 0)); after != records {
		t.Errorf("a pass over a task with every Job closed wrote %d records", after-records)
	}
}

// Row 5: two streams on one home pass the same closed task with a waiting proposal: one after the other, then
// at once from two goroutines. The Feed ends with exactly one curator Job and at most one assignment of it.
func TestUsClosedTwoStreamsPostOneCuratorJob(t *testing.T) {
	alive := func(session.Live) session.ProcessState { return session.Alive }
	check := func(t *testing.T, h *streamHome, hook *usClosedHook) {
		t.Helper()
		curators := usClosedCurators(t, h, 0)
		if len(curators) != 1 || curators[0].Job == nil {
			t.Fatalf("want exactly one curator Job on the Feed, got %v", curators)
		}
		id := *curators[0].Job
		if got := usClosedAssigns(t, h, 0, id); got > 1 {
			t.Errorf("the curator Job %s was assigned %d times", id, got)
		}
		if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
			t.Errorf("J-1 is %s, want cancelled", got)
		}
		if got := pgClosedStatus(t, h, 0, id); got == contract.Cancelled || got == contract.Rejected {
			t.Errorf("the curator Job %s is %s", id, got)
		}
		pgClosedNoReplan(t, h, 0, "J-1", id)
		if n := len(hook.asked()); n < 1 {
			t.Errorf("the hook was never asked")
		}
	}
	// pass runs every stream once; all start together when several run at once.
	pass := func(t *testing.T, streams ...*Stream) {
		t.Helper()
		errs := make([]error, len(streams))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, s := range streams {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = s.Pass(context.Background())
			}()
		}
		close(start)
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	build := func(t *testing.T) (*streamHome, *usClosedHook) {
		h := newStreamHome(t, 1)
		h.post(0, "J-1")
		pgClosedSetState(t, h, 0, "closed")
		return h, newUsClosedHook(t)
	}

	t.Run("one after the other, one supervisor", func(t *testing.T) {
		h, hook := build(t)
		a, _ := usClosedStream(h, 4242, nil, hook.ask)
		b, _ := usClosedStream(h, 4242, nil, hook.ask)
		pass(t, a)
		pass(t, b)
		pass(t, a)
		check(t, h, hook)
	})
	t.Run("one after the other, two supervisors", func(t *testing.T) {
		h, hook := build(t)
		a, _ := usClosedStream(h, 1111, alive, hook.ask)
		b, _ := usClosedStream(h, 2222, alive, hook.ask)
		pass(t, a)
		pass(t, b)
		check(t, h, hook)
	})
	t.Run("at once, one supervisor", func(t *testing.T) {
		h, hook := build(t)
		a, _ := usClosedStream(h, 4242, nil, hook.ask)
		b, _ := usClosedStream(h, 4242, nil, hook.ask)
		pass(t, a, b)
		pass(t, a, b)
		check(t, h, hook)
	})
	t.Run("at once, two supervisors", func(t *testing.T) {
		h, hook := build(t)
		a, _ := usClosedStream(h, 1111, alive, hook.ask)
		b, _ := usClosedStream(h, 2222, alive, hook.ask)
		pass(t, a, b)
		pass(t, a, b)
		check(t, h, hook)
	})
}
