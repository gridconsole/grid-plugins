package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// yvPamSubmitted is the Feed of a builder Job J-1 the worker has finished: the
// verdict on it has not come, so J-1 stays Submitted (the "done" case of TestMoves).
func yvPamSubmitted(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	postResult(t, f)
	f.exit(t, "done")
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Submitted {
		t.Fatalf("J-1 is %s, want submitted", got)
	}
	return f
}

// yvPamPost posts the person Job id for dana the way the Judge's personBody does:
// role Person, kind kind, after empty, discovered_from from (a Job id or nil).
func yvPamPost(t *testing.T, f *fixture, id, kind string, from any) {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "Person"
	body["kind"] = kind
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	body["after"] = []any{}
	body["discovered_from"] = from
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

// yvPamControls is the body of every control on the Feed that carries a status
// and names target.
func yvPamControls(t *testing.T, f *fixture, target string) []map[string]any {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, e := range r.Records {
		if e.Body["target"] == target && e.Body["status"] != nil {
			out = append(out, e.Body)
		}
	}
	return out
}

// yvPamWant fails unless the last control on target has the status, reason and
// answer given (an empty answer is not checked).
func yvPamWant(t *testing.T, f *fixture, target string, status contract.Status, reason, answer string) {
	t.Helper()
	if got := jobStatus(t, f, target); got != status {
		t.Fatalf("%s is %s, want %s", target, got, status)
	}
	cs := yvPamControls(t, f, target)
	if len(cs) == 0 {
		t.Fatalf("no status control on %s", target)
	}
	last := cs[len(cs)-1]
	if last["status"] != string(status) || last["reason"] != reason {
		t.Fatalf("control on %s: status %v reason %v, want %s %q", target, last["status"], last["reason"], status, reason)
	}
	if answer != "" && last["answer"] != answer {
		t.Fatalf("control on %s: answer %v, want %q", target, last["answer"], answer)
	}
}

// Row 1: an unleased answer to the person Job raised for J-1 closes the person
// Job and, in the holder's pass, J-1 that is still Submitted.
func TestYvPamUnleasedAnswerMovesBuilder(t *testing.T) {
	f := yvPamSubmitted(t)
	yvPamPost(t, f, "J-2", "ask", "J-1")
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-2", "Looks right.", f.now); err != nil {
		t.Fatal(err)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Submitted {
		t.Fatalf("J-1 is %s before the holder's pass; the person holds no lease", got)
	}
	f.tick(t)
	yvPamWant(t, f, "J-2", contract.Accepted, "answered by dana", "Looks right.")
	yvPamWant(t, f, "J-1", contract.Accepted, "answered by dana through J-2", "Looks right.")
}

// Row 2: a leased answer writes both controls at once, with no tick between.
func TestYvPamLeasedAnswerMovesBuilder(t *testing.T) {
	f := yvPamSubmitted(t)
	yvPamPost(t, f, "J-2", "ask", "J-1")
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-2", "Looks right.", f.now); err != nil {
		t.Fatal(err)
	}
	yvPamWant(t, f, "J-2", contract.Accepted, "answered by lead", "Looks right.")
	yvPamWant(t, f, "J-1", contract.Accepted, "answered by lead through J-2", "Looks right.")
	// The control on J-1 goes before the one on J-2: a pass that stops between
	// them repeats the person Job's part and finds J-1 moved.
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	at := map[any]int{}
	for i, e := range r.Records {
		if e.Body["status"] == "accepted" {
			at[e.Body["target"]] = i
		}
	}
	if at["J-1"] >= at["J-2"] {
		t.Fatalf("the control on J-1 (record %d) does not come before the one on J-2 (record %d)", at["J-1"], at["J-2"])
	}
}

// Row 3: the rounds are used. The person Job is discovered_from the review J-1.1
// of J-1: the builder J-1 moves, the review keeps the controls it had.
func TestYvPamRoundsUsedMovesBuilderNotReview(t *testing.T) {
	f := yvPamSubmitted(t)
	review := jobBody(t, "J-1.1")
	review["role"] = "reviewer"
	review["discovered_from"] = "J-1"
	f.post(t, feed.Input{Type: "job", Body: review, Job: "J-1.1"}, f.s.Writer)
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-1.1", "status": "accepted", "reason": "review done"}}, f.s.Writer)
	yvPamPost(t, f, "J-1.2", "ask", "J-1.1")
	before := len(yvPamControls(t, f, "J-1.1"))
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1.2", "Looks right.", f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	yvPamWant(t, f, "J-1.2", contract.Accepted, "answered by dana", "Looks right.")
	yvPamWant(t, f, "J-1", contract.Accepted, "answered by dana through J-1.2", "Looks right.")
	if after := len(yvPamControls(t, f, "J-1.1")); after != before || before != 1 {
		t.Fatalf("the review J-1.1 had %d status controls before the answer and has %d after, want 1 and 1", before, after)
	}
}

// Row 4: a plan sent back through an approve person Job rejects J-1 with the
// person's words, and carries no send_back key, which means a plan was sent back.
func TestYvPamSendBackRejectsBuilder(t *testing.T) {
	f := yvPamSubmitted(t)
	yvPamPost(t, f, "J-2", "approve", "J-1")
	if _, err := AnswerPersonWith(f.s.Feed, personWriterOf("member"), "J-2", "Not this one.", map[string]any{"send_back": true}, f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	yvPamWant(t, f, "J-2", contract.Rejected, "sent back by dana", "")
	yvPamWant(t, f, "J-1", contract.Rejected, "sent back by dana through J-2", "Not this one.")
	cs := yvPamControls(t, f, "J-1")
	last := cs[len(cs)-1]
	if last["close_reason"] != "sent back by dana through J-2" {
		t.Fatalf("close_reason = %v", last["close_reason"])
	}
	for _, key := range []string{"send_back", "sent_back_hash"} {
		if _, ok := last[key]; ok {
			t.Fatalf("the control on J-1 carries %s", key)
		}
	}
}

// Row 5: a builder Job that is not Submitted gets no control; the person Job
// still closes as it does today.
func TestYvPamBuilderNotSubmittedGetsNoSecondMove(t *testing.T) {
	for _, tc := range []struct {
		name string
		move map[string]any
		want contract.Status
	}{
		{"accepted before the answer", map[string]any{"status": "accepted", "reason": "verdict"}, contract.Accepted},
		{"cancelled", map[string]any{"status": "cancelled", "reason": "owner cancelled", "close_reason": "owner cancelled"}, contract.Cancelled},
		{"rejected", map[string]any{"status": "rejected", "reason": "no", "close_reason": "no"}, contract.Rejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := yvPamSubmitted(t)
			yvPamPost(t, f, "J-2", "ask", "J-1")
			tc.move["target"] = "J-1"
			f.post(t, feed.Input{Type: "control", Body: tc.move}, f.s.Writer)
			before := len(yvPamControls(t, f, "J-1"))
			if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-2", "Looks right.", f.now); err != nil {
				t.Fatal(err)
			}
			f.tick(t)
			if got := jobStatus(t, f, "J-2"); got != contract.Accepted {
				t.Fatalf("J-2 is %s, want accepted", got)
			}
			if got := jobStatus(t, f, "J-1"); got != tc.want {
				t.Fatalf("J-1 is %s, want %s", got, tc.want)
			}
			if cs := yvPamControls(t, f, "J-1"); len(cs) != before {
				t.Fatalf("J-1 had %d status controls before the answer and has %d: %v", before, len(cs), cs)
			}
		})
	}
}

// A person Job with no discovered_from, and one whose discovered_from is a Job the
// Feed does not hold, move nothing: only the person Job gets a control.
func TestYvPamNoBuilderToMove(t *testing.T) {
	for _, from := range []any{nil, "J-9"} {
		f := yvPamSubmitted(t)
		yvPamPost(t, f, "J-2", "ask", from)
		before := len(yvPamControls(t, f, "J-1"))
		if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-2", "Looks right.", f.now); err != nil {
			t.Fatal(err)
		}
		f.tick(t)
		if got := jobStatus(t, f, "J-2"); got != contract.Accepted {
			t.Fatalf("discovered_from %v: J-2 is %s", from, got)
		}
		if got := jobStatus(t, f, "J-1"); got != contract.Submitted || len(yvPamControls(t, f, "J-1")) != before {
			t.Fatalf("discovered_from %v: J-1 is %s with %d controls, want submitted with %d", from, got, len(yvPamControls(t, f, "J-1")), before)
		}
	}
}

// Two person Jobs raised for the same J-1 and answered before one pass: the pass
// holds one snapshot, so J-1 gets the one control and not a second that could
// undo the first.
func TestYvPamTwoPersonJobsOneMove(t *testing.T) {
	f := yvPamSubmitted(t)
	yvPamPost(t, f, "J-2", "ask", "J-1")
	yvPamPost(t, f, "J-3", "ask", "J-1")
	before := len(yvPamControls(t, f, "J-1"))
	for _, id := range []string{"J-2", "J-3"} {
		if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), id, "Looks right.", f.now); err != nil {
			t.Fatal(err)
		}
	}
	f.tick(t)
	for _, id := range []string{"J-2", "J-3"} {
		if got := jobStatus(t, f, id); got != contract.Accepted {
			t.Fatalf("%s is %s", id, got)
		}
	}
	if got := len(yvPamControls(t, f, "J-1")); got != before+1 {
		t.Fatalf("J-1 has %d status controls, want %d", got, before+1)
	}
}
