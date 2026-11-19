package orchestrator

import (
	"reflect"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jhOwner is a person writing without a lease, as an answer or an approval is.
var jhOwner = feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine}

// jhFinished runs J-1's first attempt to a done exit with a result whose row A1
// waits on a Job, and posts the Judge's hold control, so the Feed holds J-1 on
// the given waits with a result and a done exit of its attempt behind it.
func jhFinished(t *testing.T, f *fixture, waits ...contract.HeldWait) {
	t.Helper()
	launcher := b2Writer(f) // taken before the ack puts the worker's fence on f.worker
	f.ack(t)
	j := f.state(t)
	body := resultBody()
	body["acceptance"] = []any{map[string]any{"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": waits[0].Kind, "ref": waits[0].Ref}}}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
	f.post(t, feed.Input{Type: "exit", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"reason": "done", "code": 0}}, launcher)
	wire := make([]any, 0, len(waits))
	for _, w := range waits {
		wire = append(wire, map[string]any{"row": w.Row, "kind": w.Kind, "ref": w.Ref})
	}
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "held", "reason": "held by J-1.1", "waits_on": wire}}, f.s.Writer)
	if got := f.state(t); got.Status != contract.Held || len(got.WaitsOn) != len(waits) {
		t.Fatalf("J-1 = %s waits %+v, want held on %+v", got.Status, got.WaitsOn, waits)
	}
}

// jhControls reads the control bodies on the Feed, oldest first.
func jhControls(t *testing.T, f *fixture) []map[string]any {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, e := range r.Records {
		out = append(out, e.Body)
	}
	return out
}

// jhReleasedAttempt fails unless the tick released J-1: a ready control with the
// reason hold released, and a new assign of attempt 2 of the same Job, with no
// other Job posted and no person Job. It returns the assign body.
func jhReleasedAttempt(t *testing.T, f *fixture, jobs int) map[string]any {
	t.Helper()
	var released bool
	var assign map[string]any
	for _, c := range jhControls(t, f) {
		if c["target"] == "J-1" && c["status"] == "ready" && c["reason"] == "hold released" {
			released = true
		}
		if a, ok := c["assign"].(map[string]any); ok && a["job"] == "J-1" && a["attempt"] == float64(2) {
			assign = a
		}
	}
	if !released || assign == nil {
		t.Fatalf("controls = %v, want a ready control with reason hold released and an assign of attempt 2 of J-1", jhControls(t, f))
	}
	if got := f.state(t); got.Status != contract.Assigned || got.Attempt != 2 || got.WaitsOn != nil {
		t.Fatalf("J-1 = %s attempt %d waits %+v, want assigned attempt 2 with no waits", got.Status, got.Attempt, got.WaitsOn)
	}
	if n := jhPosts(t, f, "job"); n != jobs {
		t.Fatalf("%d job records after the release, want %d: no rework, person or new Job", n, jobs)
	}
	return assign
}

// A held Job whose attempt exited done with a result is not moved to Submitted or
// anywhere else by the tick, however many times it runs.
func TestHoldTickPostsNothingOverAHeldJob(t *testing.T) {
	f := b2Started(t)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitLanded, Ref: "J-6"})
	if j := f.state(t); j.Exited == "" || len(j.Results) == 0 {
		t.Fatalf("J-1 exited %q with %d results, want a done exit and a result", j.Exited, len(j.Results))
	}
	controls, jobs := b2Controls(t, f), jhPosts(t, f, "job")
	for i := 0; i < 3; i++ {
		if out := f.tick(t); out.Assigned != 0 {
			t.Fatalf("tick %d assigned %d, want none", i, out.Assigned)
		}
	}
	if b2Controls(t, f) != controls || jhPosts(t, f, "job") != jobs {
		t.Fatalf("the tick posted a control or a Job over a held Job: %v", jhControls(t, f))
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s, want held", got.Status)
	}
}

// A Job held on another Job is released and assigned its next attempt, as the
// same Job, once that Job is accepted.
func TestHoldTickReleasesWhenJobAccepted(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: "J-2"})
	jobs := jhPosts(t, f, "job")
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d with J-2 not accepted, want only J-2", out.Assigned)
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s while J-2 is not accepted, want held", got.Status)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.control(snap.State.Jobs["J-2"], contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d after J-2 was accepted, want J-1", out.Assigned)
	}
	jhReleasedAttempt(t, f, jobs)
}

// A Job held on a landing is released once a counted landed post names the Job it
// waits on; the accept of that Job alone does not release it.
func TestHoldTickReleasesWhenLanded(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.control(snap.State.Jobs["J-2"], contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitLanded, Ref: "J-2"})
	jobs := jhPosts(t, f, "job")
	if out := f.tick(t); out.Assigned != 0 {
		t.Fatalf("tick assigned %d with J-2 accepted but not landed", out.Assigned)
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s before the landing, want held", got.Status)
	}
	cli := feed.Writer{Author: feed.Author{ID: "cli", Role: "cli"}, Worker: "cli@" + machine}
	f.post(t, feed.Input{Type: "landed", Job: "J-2", Body: map[string]any{"main_sha": "9ab01cd"}}, cli)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d after the landing, want J-1", out.Assigned)
	}
	jhReleasedAttempt(t, f, jobs)
}

// A Job held on a question is released once the person answers it.
func TestHoldTickReleasesWhenQuestionAnswered(t *testing.T) {
	f := b2Started(t)
	q := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Which way?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person"}}}, f.s.Writer)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitQuestion, Ref: "Q-1"})
	jobs := jhPosts(t, f, "job")
	if out := f.tick(t); out.Assigned != 0 {
		t.Fatalf("tick assigned %d with the question open", out.Assigned)
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s while the question is open, want held", got.Status)
	}
	f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Proceed"}}, jhOwner)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d after the answer, want J-1", out.Assigned)
	}
	jhReleasedAttempt(t, f, jobs)
}

// A Blocked Job that waits on a permission is released once a person approves the
// rule of its blocked post, and the assign carries that rule as its allow and the
// approval as its grant, so the fold counts the allow and the next attempt is not
// blocked on the same rule again.
func TestHoldTickReleasesWhenPermissionGranted(t *testing.T) {
	f := b2Started(t)
	launcher := b2Writer(f)
	f.ack(t)
	j := f.state(t)
	const rule = "Bash(make deploy-prod)"
	blocked := f.post(t, feed.Input{Type: "blocked", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"need": "permission", "rule": rule, "command": "make deploy-prod"}}, f.worker)
	body := resultBody()
	body["acceptance"] = []any{map[string]any{"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": contract.WaitPermission, "ref": blocked.ID}}}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
	f.post(t, feed.Input{Type: "exit", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"reason": "done", "code": 0}}, launcher)
	jobs := jhPosts(t, f, "job")
	if out := f.tick(t); out.Assigned != 0 || f.state(t).Status != contract.Blocked {
		t.Fatalf("tick assigned %d, J-1 = %s, want a Blocked Job left alone with no approval", out.Assigned, f.state(t).Status)
	}
	// An approval of another rule does not release it.
	other := feed.Input{Type: "approval", Job: j.ID, Re: blocked.ID, Body: map[string]any{"rules": []string{"Bash(make release)"}, "decision": "approve"}}
	f.post(t, other, jhOwner)
	if snap, err := Load(f.s.Feed); err != nil || len(snap.Approved) != 0 {
		t.Fatalf("approved = %v err %v after an approval of another rule, want none", snap.Approved, err)
	}
	approval := f.post(t, feed.Input{Type: "approval", Job: j.ID, Re: blocked.ID, Body: map[string]any{"rules": []string{rule}, "decision": "approve"}}, jhOwner)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Approved[blocked.ID] || snap.Grants[blocked.ID] != (Grant{Approval: approval.ID, Rule: rule}) {
		t.Fatalf("approved %v grants %+v, want the blocked post covered by %s for %s", snap.Approved, snap.Grants, approval.ID, rule)
	}
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d after the approval, want J-1", out.Assigned)
	}
	assign := jhReleasedAttempt(t, f, jobs)
	if !reflect.DeepEqual(assign["allow"], []any{rule}) || assign["grant"] != approval.ID {
		t.Fatalf("assign = %v, want allow [%s] and grant %s", assign, rule, approval.ID)
	}
	read, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	_, loop, _ := feed.FoldReadLoopAt(read, f.now.Add(time.Second))
	if got := loop.Allow["J-1"]; !reflect.DeepEqual(got, []string{rule}) {
		t.Fatalf("the fold counts allow %v for J-1, want [%s]", loop.Allow, rule)
	}
}

// A decline, an approval by an agent and an approval naming another Job do not
// release a Blocked Job.
func TestHoldApprovalNeedsAPersonApprovingTheBlockedJob(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	f.ack(t)
	j := f.state(t)
	const rule = "Bash(make deploy-prod)"
	blocked := f.post(t, feed.Input{Type: "blocked", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"need": "permission", "rule": rule, "command": "make deploy-prod"}}, f.worker)
	rules := []string{rule}
	agent := feed.Writer{Author: feed.Author{ID: "agent", Role: "agent"}, Worker: "agent@" + machine}
	for name, in := range map[string]struct {
		in feed.Input
		w  feed.Writer
	}{
		"decline":       {feed.Input{Type: "approval", Job: j.ID, Re: blocked.ID, Body: map[string]any{"rules": rules, "decision": "decline", "why": "Not on this run"}}, jhOwner},
		"agent":         {feed.Input{Type: "approval", Job: j.ID, Re: blocked.ID, Body: map[string]any{"rules": rules, "decision": "approve"}}, agent},
		"another Job":   {feed.Input{Type: "approval", Job: "J-2", Re: blocked.ID, Body: map[string]any{"rules": rules, "decision": "approve"}}, jhOwner},
		"no blocked re": {feed.Input{Type: "approval", Job: j.ID, Body: map[string]any{"rules": rules, "decision": "approve"}}, jhOwner},
	} {
		if _, err := f.s.Feed.Post(in.in, in.w); err != nil {
			continue // the Feed refused the post, so there is nothing to release
		}
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Approved) != 0 {
			t.Errorf("%s: approved = %v, want none", name, snap.Approved)
		}
	}
}
