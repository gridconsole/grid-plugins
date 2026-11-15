package orchestrator

import (
	"errors"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// r20CrRunning is the fixture with J-1 Running under worker@<machine>, the task
// lease held by the fixture's lead.
func r20CrRunning(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %s, want running", got)
	}
	return f
}

// r20CrInput is the request body the CLI builds (internal/cli/orchestrator.go:421).
func r20CrInput(job string) feed.Input {
	return feed.Input{Type: "control", Job: job, Body: map[string]any{"target": job, "status": "cancelled", "reason": "No longer needed", "close_reason": "No longer needed"}}
}

func r20CrRequest(t *testing.T, f *fixture, role, who string) feed.Record {
	t.Helper()
	w := personWriterOf(role)
	w.Author.ID, w.Worker = who, who+"@"+machine
	rec, err := PostPerson(f.s.Feed, w, r20CrInput("J-1"))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "control" || rec.Lease != nil || rec.Fence != 0 || rec.Gen != 0 || rec.Author.ID != who {
		t.Fatalf("want an unleased request by %s, got %+v", who, rec)
	}
	return rec
}

// r20CrRe is every record on the Feed whose re is id.
func r20CrRe(t *testing.T, f *fixture, id string) []feed.Record {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Record
	for _, e := range r.Records {
		if e.Re != nil && *e.Re == id {
			out = append(out, e.Record)
		}
	}
	return out
}

// r20CrNewTenure ends the fixture's task tenure and takes a new one, as a stream
// restart does.
func r20CrNewTenure(t *testing.T, f *fixture) {
	t.Helper()
	ts := lease.TaskStore{Feed: f.s.Feed}
	if err := ts.ReleaseTask(f.s.Writer); err != nil {
		t.Fatal(err)
	}
	old := f.s.Writer.Fence
	grant, err := ts.TakeTask(f.s.Writer, os.Getpid(), contract.FormatTime(f.now))
	if err != nil {
		t.Fatal(err)
	}
	f.s.Writer.Fence, f.s.Writer.Gen = grant.Fence, grant.Gen
	if f.s.Writer.Fence == old {
		t.Fatalf("the new tenure kept fence %d", old)
	}
	if _, err := f.s.Feed.Handover(feed.Input{Body: map[string]any{}}, f.s.Writer); err != nil {
		t.Fatal(err)
	}
}

// N1/N6/N2: a person's request leaves J-1 Running; the holder's next pass writes
// the leased cancel, re the request, and the Job is Cancelled.
func TestR20CrN2HolderCarriesOutRequest(t *testing.T) {
	f := r20CrRunning(t)
	req := r20CrRequest(t, f, "owner", "dana")
	if got := jobStatus(t, f, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %s after the request alone", got)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s after the holder's pass", got)
	}
	done := r20CrRe(t, f, req.ID)
	if len(done) != 1 {
		t.Fatalf("want one record re the request, got %d", len(done))
	}
	c := done[0]
	if c.Type != "control" || c.Lease == nil || *c.Lease != "task" || c.Fence != f.s.Writer.Fence || c.Worker != f.s.Writer.Worker || c.Author.ID != f.s.Writer.Author.ID || c.ID == req.ID {
		t.Fatalf("want the holder's leased control, got %+v", c)
	}
	if c.Body["target"] != "J-1" || c.Body["status"] != "cancelled" || c.Body["reason"] != "No longer needed" || c.Body["close_reason"] != "No longer needed" {
		t.Fatalf("leased control body %v", c.Body)
	}
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := s.Jobs["J-1"]; j.CloseReason == nil || *j.CloseReason != "No longer needed" || j.History[len(j.History)-1] == req.ID {
		t.Fatalf("J-1 close reason %v, history %v", j.CloseReason, j.History)
	}
}

// Review note 4: with nobody holding the task lease the request waits: no leased
// control and J-1 still Running. The next holder's pass carries it out.
func TestR20CrNoHolderWaits(t *testing.T) {
	f := r20CrRunning(t)
	if err := (lease.TaskStore{Feed: f.s.Feed}).ReleaseTask(f.s.Writer); err != nil {
		t.Fatal(err)
	}
	if held, err := (lease.TaskStore{Feed: f.s.Feed}).Read("task"); err != nil || held.Holder != nil {
		t.Fatalf("the task lease is still held: %+v %v", held, err)
	}
	req := r20CrRequest(t, f, "member", "sam")
	if _, err := f.s.Tick(); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatalf("a pass ran without the task lease: %v", err)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Running || len(r20CrRe(t, f, req.ID)) != 0 {
		t.Fatalf("J-1 is %s with no holder", got)
	}
	grant, err := (lease.TaskStore{Feed: f.s.Feed}).TakeTask(f.s.Writer, os.Getpid(), contract.FormatTime(f.now))
	if err != nil {
		t.Fatal(err)
	}
	f.s.Writer.Fence, f.s.Writer.Gen = grant.Fence, grant.Gen
	if _, err := f.s.Feed.Handover(feed.Input{Body: map[string]any{}}, f.s.Writer); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Cancelled || len(r20CrRe(t, f, req.ID)) != 1 {
		t.Fatalf("J-1 is %s after the new holder's pass", got)
	}
}

// N3/N4 at the store: the schema refuses a guest's or an agent's leaseless cancel,
// so Store.Post writes none; PostPerson refuses them before the lock.
func TestR20CrN3N4StoreRefusesGuestAndAgent(t *testing.T) {
	for _, role := range []string{"guest", "worker", "cli", "agent", "reviewer"} {
		f := r20CrRunning(t)
		before := recordCount(t, f)
		w := personWriterOf(role)
		var invalid *feed.InvalidRecordError
		if _, err := f.s.Feed.Post(r20CrInput("J-1"), w); !errors.As(err, &invalid) {
			t.Fatalf("%s: the store took a leaseless cancel: %v", role, err)
		}
		if _, err := PostPerson(f.s.Feed, w, r20CrInput("J-1")); !errors.As(err, &invalid) {
			t.Fatalf("%s: PostPerson took a leaseless cancel: %v", role, err)
		}
		if recordCount(t, f) != before {
			t.Fatalf("%s: a refused post left a record", role)
		}
		f.tick(t)
		if got := jobStatus(t, f, "J-1"); got != contract.Running {
			t.Fatalf("%s: J-1 is %s", role, got)
		}
	}
	// The same post by a person of the team is taken.
	for _, role := range []string{"owner", "member", "lead"} {
		f := r20CrRunning(t)
		if _, err := f.s.Feed.Post(r20CrInput("J-1"), personWriterOf(role)); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
	}
}

// N5: an unleased control with any status but cancelled is refused by schema
// validation, an InvalidRecordError (the CLI's exit 3), and nothing is written.
func TestR20CrN5OtherStatusRefused(t *testing.T) {
	f := r20CrRunning(t)
	before := recordCount(t, f)
	for _, status := range []string{"accepted", "rejected", "ready", "stale", "parked", "posted"} {
		in := r20CrInput("J-1")
		in.Body["status"] = status
		var invalid *feed.InvalidRecordError
		_, err := f.s.Feed.Post(in, personWriterOf("owner"))
		if !errors.As(err, &invalid) || !strings.Contains(err.Error(), "$") {
			t.Fatalf("%s: want a schema InvalidRecordError, got %v", status, err)
		}
		if _, err := PostPerson(f.s.Feed, personWriterOf("owner"), in); !errors.As(err, &invalid) {
			t.Fatalf("%s: PostPerson: want an InvalidRecordError, got %v", status, err)
		}
	}
	if recordCount(t, f) != before {
		t.Fatal("a refused control left a record")
	}
	if _, err := f.s.Feed.Post(r20CrInput("J-1"), personWriterOf("owner")); err != nil {
		t.Fatalf("the cancelled request was refused: %v", err)
	}
}

// R8 and R9: a request for a Job the task does not have, or one already closed, is
// refused under the lock with a plain error (exit 1), and nothing is written.
func TestR20CrR8R9Refusals(t *testing.T) {
	f := r20CrRunning(t)
	before := recordCount(t, f)
	_, err := PostPerson(f.s.Feed, personWriterOf("owner"), r20CrInput("J-9"))
	var invalid *feed.InvalidRecordError
	if err == nil || err.Error() != "grid job cancel: no Job J-9 in this task" || errors.As(err, &invalid) {
		t.Fatalf("R8: got %v", err)
	}
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "cancelled", "reason": "done", "close_reason": "done"}}); err != nil {
		t.Fatal(err)
	}
	before++
	_, err = PostPerson(f.s.Feed, personWriterOf("owner"), r20CrInput("J-1"))
	if err == nil || err.Error() != "grid job cancel: J-1 is already cancelled" || errors.As(err, &invalid) {
		t.Fatalf("R9: got %v", err)
	}
	if recordCount(t, f) != before {
		t.Fatal("a refused request left a record")
	}
}

// N7: two requests for one Job: the earliest is carried out, the other gets one
// message that it was not used; a later pass writes nothing more for either.
func TestR20CrN7EarliestWins(t *testing.T) {
	f := r20CrRunning(t)
	a := r20CrRequest(t, f, "owner", "dana")
	b := r20CrRequest(t, f, "member", "sam")
	reqs := []feed.Record{a, b}
	sort.Slice(reqs, func(i, j int) bool {
		if reqs[i].At != reqs[j].At {
			return reqs[i].At < reqs[j].At
		}
		return reqs[i].ID < reqs[j].ID
	})
	first, late := reqs[0], reqs[1]
	f.tick(t)
	won, lost := r20CrRe(t, f, first.ID), r20CrRe(t, f, late.ID)
	if len(won) != 1 || won[0].Type != "control" || won[0].Body["status"] != "cancelled" {
		t.Fatalf("want one leased control re the first request, got %+v", won)
	}
	if len(lost) != 1 || lost[0].Type != "post" || lost[0].Lease == nil || !strings.Contains(stringValue(lost[0].Body["text"]), late.ID) || !strings.Contains(stringValue(lost[0].Body["text"]), "not used") {
		t.Fatalf("want one not-used message re the later request, got %+v", lost)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s", got)
	}
	f.tick(t)
	if len(r20CrRe(t, f, first.ID)) != 1 || len(r20CrRe(t, f, late.ID)) != 1 {
		t.Fatal("a second pass wrote again for a request already carried out")
	}
}

// N8: a request whose Job closed before the pass gets one not-used message and no
// control; a later pass writes nothing more.
func TestR20CrN8ClosedFirst(t *testing.T) {
	f := r20CrRunning(t)
	req := r20CrRequest(t, f, "owner", "dana")
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "rejected", "reason": "closed by the holder", "close_reason": "closed by the holder"}}); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	got := r20CrRe(t, f, req.ID)
	if len(got) != 1 || got[0].Type != "post" || !strings.Contains(stringValue(got[0].Body["text"]), "not used") || !strings.Contains(stringValue(got[0].Body["text"]), req.ID) {
		t.Fatalf("want one not-used message, got %+v", got)
	}
	if s := jobStatus(t, f, "J-1"); s != contract.Rejected {
		t.Fatalf("J-1 is %s; the request moved a closed Job", s)
	}
	f.tick(t)
	if len(r20CrRe(t, f, req.ID)) != 1 {
		t.Fatal("a second pass wrote again")
	}
}

// N9: the pass repeated under a new task fence (a stream restart) writes nothing
// more for a request already carried out.
func TestR20CrN9NewTenure(t *testing.T) {
	f := r20CrRunning(t)
	req := r20CrRequest(t, f, "lead", "lee")
	f.tick(t)
	if len(r20CrRe(t, f, req.ID)) != 1 || jobStatus(t, f, "J-1") != contract.Cancelled {
		t.Fatal("the first pass did not carry out the request")
	}
	r20CrNewTenure(t, f)
	f.tick(t)
	if got := r20CrRe(t, f, req.ID); len(got) != 1 || got[0].Type != "control" {
		t.Fatalf("after a new tenure: %+v", got)
	}
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.Type == "post" && strings.Contains(stringValue(e.Body["text"]), req.ID) {
			t.Fatalf("the new tenure's pass wrote a message for a request already carried out: %v", e.Body["text"])
		}
	}
}
