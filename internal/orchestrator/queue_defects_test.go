package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func personAfter(t *testing.T, f *fixture, id string, after []any) {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Approve the plan."}
	body["after"] = after
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

// accept runs J-1 to a result and accepts it, with or without the wake key.
func (f *fixture) acceptJ1(t *testing.T, wake string) {
	t.Helper()
	f.tick(t)
	f.ack(t)
	f.post(t, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()}, f.worker)
	if wake == "" {
		return
	}
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Wake: wake, Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "ok"}}); err != nil {
		t.Fatal(err)
	}
}

// D11: a person's Job is not answered while a Job it waits for is not accepted.
func TestPersonJobIsNotAnsweredBeforeItsAfter(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	personAfter(t, f, "J-2", []any{"J-1"})
	f.tick(t)
	_, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-2", "approve", f.now)
	if !errors.Is(err, feed.ErrInvalidRecord) || !strings.Contains(err.Error(), "J-1") {
		t.Fatalf("want an invalid record naming J-1, got %v", err)
	}
	if got := jobStatus(t, f, "J-2"); got != contract.Posted {
		t.Fatalf("J-2 is %s after a refused answer", got)
	}
	f.ack(t)
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "ok"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-2", "approve", f.now); err != nil {
		t.Fatalf("answer once J-1 is accepted: %v", err)
	}
	if got := jobStatus(t, f, "J-2"); got != contract.Accepted {
		t.Fatalf("J-2 is %s", got)
	}
}

// D8: an after target that is not posted is refused when the Job is posted.
func TestPostRefusesUnknownAfterTarget(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	body := jobBody(t, "J-2")
	body["after"] = []any{"J-9"}
	_, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "job", Job: "J-2", Body: body})
	if !errors.Is(err, feed.ErrInvalidRecord) || !strings.Contains(err.Error(), "J-9") {
		t.Fatalf("want an invalid record naming J-9, got %v", err)
	}
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if s.Jobs["J-2"] != nil {
		t.Fatal("the refused Job was posted")
	}
	body["after"] = []any{"J-1"}
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "job", Job: "J-2", Body: body}); err != nil {
		t.Fatalf("an existing target is refused: %v", err)
	}
}

// D8: a Job already on the Feed behind a missing target waits alone; the rest of
// the queue is still scheduled, and what is behind the held Job waits as well.
func TestMissingAfterTargetDoesNotWedgeTheQueue(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-9"})
	f.job(t, "J-3", []any{"J-2"})
	out := f.tick(t)
	if out.Assigned != 1 {
		t.Fatalf("want J-1 assigned, got %d: %+v", out.Assigned, out.Rows)
	}
	for id, want := range map[string]contract.Status{"J-1": contract.Assigned, "J-2": contract.Posted, "J-3": contract.Posted} {
		if got := jobStatus(t, f, id); got != want {
			t.Fatalf("%s is %s, want %s", id, got, want)
		}
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	waits := map[string]string{}
	for _, w := range ready.Waiting {
		waits[w.Job] = w.Target
	}
	if waits["J-2"] != "J-9" || waits["J-3"] != "J-2" {
		t.Fatalf("waits %v", waits)
	}
}

// D12: accepting a Job consumes the wake its result raised, with or without --wake.
func TestAcceptedJobConsumesItsResultWake(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.acceptJ1(t, "")
	if w := f.tick(t).Wake; w == nil {
		t.Fatal("the result raised no wake")
	}
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "ok"}}); err != nil {
		t.Fatal(err)
	}
	if w := f.tick(t).Wake; w != nil {
		t.Fatalf("the wake of an accepted Job is still standing: %+v", w)
	}
	if w := f.tick(t).Wake; w != nil {
		t.Fatalf("the wake came back: %+v", w)
	}
}
