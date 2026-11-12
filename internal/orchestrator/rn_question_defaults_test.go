package orchestrator

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/question"
)

const rnQdDefault = "Keep parked"

// rnQdOwner is the person who asks and answers: a writer of the team with no lease.
func rnQdOwner() feed.Writer {
	return feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine}
}

// rnQdPostJob is the Job post of cli/question.go's postQuestionJob for a person
// asker: the id is the next one the Feed holds, and the person posts it unleased.
func rnQdPostJob(f feed.Store, _, person feed.Writer, body map[string]any) (string, error) {
	snap, err := Load(f)
	if err != nil {
		return "", err
	}
	top := 0
	for _, id := range snap.JobIDs {
		n, _ := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-"))
		top = max(top, n)
	}
	id := fmt.Sprintf("J-%d", top+1)
	body["id"] = id
	_, err = PostPerson(f, person, feed.Input{Type: "job", Body: body, Job: id})
	return id, err
}

// rnQdStore is the owner's question store for task 0 with the real Close of the
// question verb and a Post like the verb's.
func rnQdStore(h *streamHome) question.Store {
	return question.Store{
		TaskDir: h.dirs[0], TaskID: h.ids[0], Writer: rnQdOwner(), ControlWriter: rnQdOwner(),
		Jobs: question.JobPort{Post: rnQdPostJob, Close: CloseQuestionJob},
	}
}

func rnQdInput(applies time.Time, ageAfter time.Duration) question.Input {
	return question.Input{Question: "How should it continue?", Who: "owner", Options: []string{rnQdDefault, "Retry"}, Recommended: rnQdDefault,
		Default: rnQdDefault, Applies: applies, Blocking: true, AgeAfter: ageAfter}
}

// rnQdAsk asks one question as a person and returns its record, with its person Job.
func rnQdAsk(t *testing.T, h *streamHome, applies time.Time, ageAfter time.Duration) question.Record {
	t.Helper()
	r, err := rnQdStore(h).Ask(rnQdInput(applies, ageAfter))
	if err != nil {
		t.Fatal(err)
	}
	if r.Job == "" {
		t.Fatalf("ask posted no person Job: %+v", r)
	}
	return r
}

// rnQdPostAfter posts a Job that lists the person Job in its after list, the way
// an owner posts Jobs (take the task lease, post, release).
func rnQdPostAfter(t *testing.T, h *streamHome, id, person string) {
	t.Helper()
	st := h.store(0)
	ts := lease.TaskStore{Feed: st}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = g.Fence
	if _, err := st.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	body := jobBody(t, id)
	body["after"] = []any{map[string]any{"job": person, "kind": "done", "why": "needs the answer"}}
	if _, err := st.Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
		t.Fatal(err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
}

// rnQdRecord reads one question of task 0 from the Feed projection.
func rnQdRecord(t *testing.T, h *streamHome, id string) question.Record {
	t.Helper()
	records, err := rnQdStore(h).List()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no question %s in %+v", id, records)
	return question.Record{}
}

// rnQdKinds is the kinds of the history events of a question.
func rnQdKinds(r question.Record) []string {
	out := []string{}
	for _, e := range r.History {
		out = append(out, e.Kind)
	}
	return out
}

func rnQdFile(t *testing.T, h *streamHome, id string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.dirs[0], "questions", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func rnQdSnap(t *testing.T, h *streamHome) Snapshot {
	t.Helper()
	snap, err := Load(h.store(0))
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func rnQdStatus(t *testing.T, h *streamHome, job string) contract.Status {
	t.Helper()
	j := rnQdSnap(t, h).State.Jobs[job]
	if j == nil {
		t.Fatalf("no Job %s", job)
	}
	return j.Status
}

// rnQdWaitsOn says whether Ready holds job back for target.
func rnQdWaitsOn(t *testing.T, h *streamHome, job, target string, now time.Time) bool {
	t.Helper()
	ready, err := Ready(rnQdSnap(t, h), now)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(ready.Ready, job) {
		return false
	}
	for _, w := range ready.Waiting {
		if w.Job == job && w.Target == target {
			return true
		}
	}
	return false
}

// rnQdAnswerOf is the answer of the control that accepted a Job, or "" and false.
func rnQdAnswerOf(t *testing.T, h *streamHome, job string) (string, bool) {
	t.Helper()
	read, err := h.store(0).Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range read.Records {
		if r.Body["target"] == job && r.Body["status"] == "accepted" {
			a, _ := r.Body["answer"].(string)
			return a, true
		}
	}
	return "", false
}

// rnQdAbout counts the Feed records that belong to a question post or to its
// person Job: they target it, answer it, or carry its Job id.
func rnQdAbout(t *testing.T, h *streamHome, post, job string) int {
	t.Helper()
	read, err := h.store(0).Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range read.Records {
		switch {
		case r.ID == post, r.Re != nil && *r.Re == post, r.Body["target"] == post, r.Body["target"] == job, r.Job != nil && *r.Job == job:
			n++
		}
	}
	return n
}

// rnQdClock is the stream's clock in the tests that move time.
type rnQdClock struct{ at time.Time }

func (c *rnQdClock) now() time.Time { return c.at }

// 1. A pass applies the default of a question that is due: the question holds a
// defaulted event, its person Job is accepted with the default as the answer, and
// the Job behind that person Job carries on in the same pass (assigned, with its
// spawn line). On base the person Job stays open and the Job waits for it.
func TestRnQdPassDefaultsDueQuestionAndReleasesJobAfterPerson(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(-time.Hour), time.Hour)
	if q.ID != "Q-1" || q.Job != "J-1" {
		t.Fatalf("want Q-1 behind J-1, got %s behind %s", q.ID, q.Job)
	}
	rnQdPostAfter(t, h, "J-2", q.Job)
	if !rnQdWaitsOn(t, h, "J-2", q.Job, time.Now()) {
		t.Fatalf("before the pass J-2 must wait for the person Job %s", q.Job)
	}

	h.pass(h.stream(4242, nil))

	if got := rnQdKinds(rnQdRecord(t, h, "Q-1")); !slices.Equal(got, []string{"defaulted"}) {
		t.Errorf("Q-1 history = %v, want [defaulted]", got)
	}
	if file := rnQdFile(t, h, "Q-1"); !bytes.Contains(file, []byte(`"kind": "defaulted"`)) {
		t.Errorf("questions/Q-1.md holds no defaulted event:\n%s", file)
	}
	if st := rnQdStatus(t, h, "J-1"); st != contract.Accepted {
		t.Errorf("person Job J-1 is %s, want accepted", st)
	}
	if a, ok := rnQdAnswerOf(t, h, "J-1"); !ok || a != rnQdDefault {
		t.Errorf("J-1 accepted with answer %q (accepted %v), want the default %q", a, ok, rnQdDefault)
	}
	// Same pass: J-2 was Ready after J-1 was accepted, so the pass assigned it.
	if st := rnQdStatus(t, h, "J-2"); st != contract.Assigned {
		t.Errorf("J-2 is %s, want assigned in the same pass", st)
	}
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 || spawns[0]["id"] != "J-2" {
		t.Errorf("want one spawn line for J-2, got %v", spawns)
	}
}

// 2. A second pass records nothing more for the question or its person Job.
func TestRnQdSecondPassRecordsNothingNew(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(-time.Hour), time.Hour)
	rnQdPostAfter(t, h, "J-2", q.Job)
	s := h.stream(4242, nil)
	h.pass(s)
	if got := rnQdKinds(rnQdRecord(t, h, "Q-1")); !slices.Equal(got, []string{"defaulted"}) {
		t.Fatalf("after the first pass Q-1 history = %v, want [defaulted]", got)
	}
	file, post := rnQdFile(t, h, "Q-1"), rnQdRecord(t, h, "Q-1").Post
	about := rnQdAbout(t, h, post, q.Job)
	read, err := h.store(0).Read(feed.Filter{Type: "question"})
	if err != nil {
		t.Fatal(err)
	}
	questions := len(read.Records)

	h.pass(s)

	if got := rnQdKinds(rnQdRecord(t, h, "Q-1")); !slices.Equal(got, []string{"defaulted"}) {
		t.Errorf("after the second pass Q-1 history = %v, want [defaulted]", got)
	}
	if after := rnQdFile(t, h, "Q-1"); !bytes.Equal(file, after) {
		t.Errorf("the second pass changed questions/Q-1.md:\n%s\nthen\n%s", file, after)
	}
	if n := rnQdAbout(t, h, post, q.Job); n != about {
		t.Errorf("the second pass posted %d record(s) for Q-1 or J-1", n-about)
	}
	if read, err = h.store(0).Read(feed.Filter{Type: "question"}); err != nil || len(read.Records) != questions {
		t.Errorf("question records %d then %d (%v)", questions, len(read.Records), err)
	}
}

// 3. A blocking question that is not due ages one step per age interval: 1 after
// the first pass past one step, 2 after the next, and 2 after a third.
func TestRnQdPassRaisesAgingTwiceThenStops(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(72*time.Hour), time.Hour)
	asked := rnQdRecord(t, h, "Q-1").Asked.Time
	clock := &rnQdClock{at: asked}
	s := h.stream(4242, nil)
	s.Now = clock.now
	for _, step := range []struct {
		after time.Duration
		aging int
	}{{0, 0}, {61 * time.Minute, 1}, {121 * time.Minute, 2}, {181 * time.Minute, 2}} {
		clock.at = asked.Add(step.after)
		h.pass(s)
		r := rnQdRecord(t, h, "Q-1")
		if r.Aging != step.aging {
			t.Errorf("at +%s aging = %d, want %d (history %v)", step.after, r.Aging, step.aging, rnQdKinds(r))
		}
		want := slices.Repeat([]string{"aged"}, step.aging)
		if got := rnQdKinds(r); !slices.Equal(got, want) {
			t.Errorf("at +%s history = %v, want %v", step.after, got, want)
		}
	}
	// Aging decides nothing: the person Job stays open for the person.
	if st := rnQdStatus(t, h, q.Job); st == contract.Accepted {
		t.Errorf("aging closed the person Job %s", q.Job)
	}
}

// 4. A question that a person answered or withdrew before its deadline, and a
// question file with no Feed post, are not touched by a pass past the deadline.
// A due question of the same task is defaulted in that pass, so the pass ran.
func TestRnQdPassLeavesDecidedAndUnpostedQuestionsAlone(t *testing.T) {
	h := newStreamHome(t, 1)
	soon := time.Now().Add(24 * time.Hour)
	rnQdAsk(t, h, soon, time.Hour)        // Q-1, answered
	rnQdAsk(t, h, soon, time.Hour)        // Q-2, withdrawn
	due := rnQdAsk(t, h, soon, time.Hour) // Q-3, due when the clock moves
	if _, err := rnQdStore(h).Answer("Q-1", "Retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := rnQdStore(h).Withdraw("Q-2", "Not needed"); err != nil {
		t.Fatal(err)
	}
	// A question file the Feed does not know: no Post, so nothing to decide.
	legacy := filepath.Join(h.dirs[0], "questions", "Q-9.md")
	legacyBody := []byte("legacy question, not in the Feed\n")
	if err := os.WriteFile(legacy, legacyBody, 0o600); err != nil {
		t.Fatal(err)
	}
	before := map[string][]string{}
	for _, id := range []string{"Q-1", "Q-2"} {
		before[id] = rnQdKinds(rnQdRecord(t, h, id))
	}
	aboutBefore := map[string]int{}
	for _, id := range []string{"Q-1", "Q-2"} {
		aboutBefore[id] = rnQdAbout(t, h, rnQdRecord(t, h, id).Post, "-")
	}

	clock := &rnQdClock{at: soon.Add(time.Hour)} // past every deadline
	s := h.stream(4242, nil)
	s.Now = clock.now
	h.pass(s)
	h.pass(s)

	for _, id := range []string{"Q-1", "Q-2"} {
		r := rnQdRecord(t, h, id)
		if got := rnQdKinds(r); !slices.Equal(got, before[id]) {
			t.Errorf("%s history = %v, want %v untouched", id, got, before[id])
		}
		if n := rnQdAbout(t, h, r.Post, "-"); n != aboutBefore[id] {
			t.Errorf("%s: the passes posted %d record(s) for it", id, n-aboutBefore[id])
		}
	}
	if r := rnQdRecord(t, h, "Q-1"); r.State != "answered" || r.Answer != "Retry" {
		t.Errorf("Q-1 is %s %q, want answered Retry", r.State, r.Answer)
	}
	if r := rnQdRecord(t, h, "Q-2"); r.State != "withdrawn" {
		t.Errorf("Q-2 is %s, want withdrawn", r.State)
	}
	if b, err := os.ReadFile(legacy); err != nil || !bytes.Equal(b, legacyBody) {
		t.Errorf("the unposted question file changed: %q %v", b, err)
	}
	if got := rnQdKinds(rnQdRecord(t, h, due.ID)); !slices.Equal(got, []string{"defaulted"}) {
		t.Errorf("%s is due but its history is %v, want [defaulted]: the pass did not run", due.ID, got)
	}
}

// rnQdProbe is a Close wrapper that tries both locks on a new descriptor without
// waiting, at the moment the Job port is called; a lock found held is recorded.
type rnQdProbe struct {
	mu     sync.Mutex
	calls  []string
	locked []string
}

func (p *rnQdProbe) try(path string) error {
	f, err := fslock.OpenNoFollow(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	err = fslock.TryLockExclusive(f)
	if errors.Is(err, fslock.ErrLocked) {
		p.mu.Lock()
		p.locked = append(p.locked, filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path))
		p.mu.Unlock()
		return nil
	}
	if err != nil {
		return err
	}
	fslock.Unlock(f)
	return nil
}

func (p *rnQdProbe) wrap(h *streamHome) func(question.JobPort) question.JobPort {
	return func(port question.JobPort) question.JobPort {
		real := port.Close
		port.Close = func(f feed.Store, lease, person feed.Writer, id, result string) error {
			p.mu.Lock()
			p.calls = append(p.calls, id)
			p.mu.Unlock()
			for _, path := range []string{
				filepath.Join(h.dirs[0], "questions", ".lock"),
				filepath.Join(f.Dir(), ".orchestrator.lock"),
			} {
				if err := p.try(path); err != nil {
					return err
				}
			}
			return real(f, lease, person, id, result)
		}
		return port
	}
}

// 5. The pass closes the person Job with questions/.lock and .orchestrator.lock
// both free. The probe runs inside the real Close, through a pass, and the same
// probe is shown to see a lock that is held.
func TestRnQdCloseRunsWithBothLocksFree(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(-time.Hour), time.Hour)
	rnQdPostAfter(t, h, "J-2", q.Job)
	p := &rnQdProbe{}
	rnQdPortForTest = p.wrap(h)
	t.Cleanup(func() { rnQdPortForTest = nil })

	h.pass(h.stream(4242, nil))

	if !slices.Equal(p.calls, []string{q.Job}) {
		t.Fatalf("the Job port Close was called for %v, want once for %s", p.calls, q.Job)
	}
	if len(p.locked) != 0 {
		t.Errorf("Close ran while %v was held", p.locked)
	}
	if st := rnQdStatus(t, h, q.Job); st != contract.Accepted {
		t.Errorf("the wrapped Close did not give the person Job its result: %s", st)
	}

	// The probe is not blind: with questions/.lock held on another descriptor it says so.
	lock, err := fslock.OpenNoFollow(filepath.Join(h.dirs[0], "questions", ".lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	seen := &rnQdProbe{}
	if err := seen.try(filepath.Join(h.dirs[0], "questions", ".lock")); err != nil || len(seen.locked) != 1 {
		t.Errorf("the probe missed a held lock: %v %v", seen.locked, err)
	}
	fslock.Unlock(lock)
}
