package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// tcPressRig is a task with two approve person Jobs: J-2, which a person has
// already answered without a lease (the answer waits for the holder's tick), and
// a later J-3 posted after it. The holder's tick has not run.
type tcPressRig struct {
	h      *harness
	path   string
	store  feed.Store
	text   string
	taskMD []byte
}

// tcPressSetup presses approve once as a person with no lease (an answer on J-2),
// restores the lease, and posts the later approve person Job J-3 holding the Jobs
// in after (nil: it holds none).
func tcPressSetup(t *testing.T, after []any) *tcPressRig {
	t.Helper()
	h, path, store, text := approveJobHarness(t)
	h.person()
	taskMD, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	r := &tcPressRig{h: h, path: path, store: store, text: text, taskMD: taskMD}
	if code := r.press(t); code != 0 {
		t.Fatalf("first press: exit %d: %s", code, h.errb)
	}
	if got := len(arAnswers(t, store)); got != 1 {
		t.Fatalf("after the first press the Feed holds %d answers, want 1", got)
	}
	// The lease comes back through the environment, as the holder's pass has it.
	h.env.Getenv = nil
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-3", after)), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "approve"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-3", "--body", string(out)); code != 0 {
		t.Fatalf("post J-3: exit %d: %s", code, h.errb)
	}
	return r
}

// press is grid approve by a person with no lease for the current plan, with task.md
// back to waiting as the app's real-plugin test leaves it between presses.
func (r *tcPressRig) press(t *testing.T) int {
	t.Helper()
	r.h.person()
	if err := os.WriteFile(filepath.Join(r.path, "task.md"), r.taskMD, 0o644); err != nil {
		t.Fatal(err)
	}
	r.h.out.Reset()
	r.h.errb.Reset()
	return r.h.run(r.path, "approve", r.path, "--plan-hash", plan.Hash(r.text), "--json")
}

// answersTo is how many answers of the Feed follow the first record of job.
func (r *tcPressRig) answersTo(t *testing.T, job string) int {
	t.Helper()
	st, err := r.store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[job]
	if j == nil || len(j.History) == 0 {
		t.Fatalf("%s has no history: %+v", job, j)
	}
	n := 0
	for _, a := range arAnswers(t, r.store) {
		if a.Re != nil && *a.Re == j.History[0] {
			n++
		}
	}
	return n
}

// A press on a later approve step lands on that step while an earlier approve Job
// still waits for the holder's tick with a person's answer: the Feed holds one
// answer for each Job.
func TestTcPressLaterStep(t *testing.T) {
	r := tcPressSetup(t, nil)
	if code := r.press(t); code != 0 {
		t.Fatalf("second press: exit %d: %s", code, r.h.errb)
	}
	if got := len(arAnswers(t, r.store)); got != 2 {
		t.Fatalf("the Feed holds %d answers, want 2: %+v", got, arAnswers(t, r.store))
	}
	if n := r.answersTo(t, "J-2"); n != 1 {
		t.Fatalf("%d answers re J-2's first record, want 1", n)
	}
	if n := r.answersTo(t, "J-3"); n != 1 {
		t.Fatalf("%d answers re J-3's first record, want 1", n)
	}
}

// A later approve Job that waits for a Job that is not accepted cannot be answered
// yet: the second press is the repeat press of J-2 and writes no new answer.
func TestTcPressLaterNotReady(t *testing.T) {
	r := tcPressSetup(t, []any{"J-1"})
	if code := r.press(t); code != 0 {
		t.Fatalf("second press: exit %d: %s", code, r.h.errb)
	}
	if got := len(arAnswers(t, r.store)); got != 1 {
		t.Fatalf("the Feed holds %d answers, want 1: %+v", got, arAnswers(t, r.store))
	}
	if n := r.answersTo(t, "J-2"); n != 1 {
		t.Fatalf("%d answers re J-2's first record, want 1", n)
	}
	if n := r.answersTo(t, "J-3"); n != 0 {
		t.Fatalf("%d answers re J-3's first record, want 0", n)
	}
}

// The holder's tick turns each waiting answer into its accepting control: after
// both presses J-2 and J-3 are accepted.
func TestTcPressTickAccepts(t *testing.T) {
	r := tcPressSetup(t, nil)
	if code := r.press(t); code != 0 {
		t.Fatalf("second press: exit %d: %s", code, r.h.errb)
	}
	// The holder's pass: the harness holds the task lease through the environment.
	r.h.env.Getenv = nil
	if code := r.h.run(r.path, "job", "next"); code != 0 {
		t.Fatalf("tick: exit %d: %s", code, r.h.errb)
	}
	for _, job := range []string{"J-2", "J-3"} {
		if s := arStatus(t, r.store, job); s != "accepted" {
			t.Fatalf("%s is %q after the tick, want accepted", job, s)
		}
		if got := arAccepts(t, r.store, job); len(got) != 1 {
			t.Fatalf("%s: accepting controls %v, want 1", job, got)
		}
	}
}
