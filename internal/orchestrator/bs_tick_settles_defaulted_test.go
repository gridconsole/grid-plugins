package orchestrator

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// bsTsOrFailFirstClose makes the Job port's Close fail on its first call and run
// the real Close after, and returns the number of calls it has seen.
func bsTsOrFailFirstClose(t *testing.T) *int {
	t.Helper()
	calls := new(int)
	rnQdPortForTest = func(port question.JobPort) question.JobPort {
		real := port.Close
		port.Close = func(f feed.Store, lease, person feed.Writer, id, result string) error {
			*calls++
			if *calls == 1 {
				return errors.New("close failed")
			}
			return real(f, lease, person, id, result)
		}
		return port
	}
	t.Cleanup(func() { rnQdPortForTest = nil })
	return calls
}

// 2. The first pass defaults the question but its Close fails, so the person Job
// stays open. The second pass, on the same stream, closes it with the default as
// the answer and the Job behind it carries on. On base the person Job stays open.
func TestBsTsOrPassClosesStrandedPersonJob(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(-time.Hour), time.Hour)
	rnQdPostAfter(t, h, "J-2", q.Job)
	calls := bsTsOrFailFirstClose(t)
	s := h.stream(4242, nil)

	h.pass(s)

	if got := rnQdKinds(rnQdRecord(t, h, "Q-1")); !slices.Equal(got, []string{"defaulted"}) {
		t.Fatalf("after the first pass Q-1 history = %v, want [defaulted]", got)
	}
	if st := rnQdStatus(t, h, q.Job); st == contract.Accepted {
		t.Fatalf("person Job %s is accepted after a failed Close", q.Job)
	}
	if *calls != 1 {
		t.Fatalf("Close ran %d times in the first pass, want 1", *calls)
	}

	h.pass(s)

	if st := rnQdStatus(t, h, q.Job); st != contract.Accepted {
		t.Errorf("person Job %s is %s after the second pass, want accepted", q.Job, st)
	}
	if a, ok := rnQdAnswerOf(t, h, q.Job); !ok || a != rnQdDefault {
		t.Errorf("%s accepted with answer %q (accepted %v), want the default %q", q.Job, a, ok, rnQdDefault)
	}
	if st := rnQdStatus(t, h, "J-2"); st != contract.Assigned {
		t.Errorf("J-2 is %s, want assigned", st)
	}
	if got := rnQdKinds(rnQdRecord(t, h, "Q-1")); !slices.Equal(got, []string{"defaulted"}) {
		t.Errorf("after the second pass Q-1 history = %v, want [defaulted]", got)
	}
}

// 3. A third pass posts nothing for the question or its person Job and leaves
// questions/Q-1.md as it was.
func TestBsTsOrThirdPassPostsNothing(t *testing.T) {
	h := newStreamHome(t, 1)
	q := rnQdAsk(t, h, time.Now().Add(-time.Hour), time.Hour)
	rnQdPostAfter(t, h, "J-2", q.Job)
	bsTsOrFailFirstClose(t)
	s := h.stream(4242, nil)
	h.pass(s)
	h.pass(s)
	if st := rnQdStatus(t, h, q.Job); st != contract.Accepted {
		t.Fatalf("person Job %s is %s after the second pass, want accepted", q.Job, st)
	}
	file, post := rnQdFile(t, h, "Q-1"), rnQdRecord(t, h, "Q-1").Post
	about := rnQdAbout(t, h, post, q.Job)

	h.pass(s)

	if n := rnQdAbout(t, h, post, q.Job); n != about {
		t.Errorf("the third pass posted %d record(s) for Q-1 or %s", n-about, q.Job)
	}
	if after := rnQdFile(t, h, "Q-1"); !bytes.Equal(file, after) {
		t.Errorf("the third pass changed questions/Q-1.md:\n%s\nthen\n%s", file, after)
	}
}
