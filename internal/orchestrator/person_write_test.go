package orchestrator

import (
	"errors"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func personWriterOf(role string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: "dana", Role: role}, Worker: "dana@" + machine}
}

func recordCount(t *testing.T, f *fixture) int {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// Row 1: the task lease is held by the fixture's lead; a person posts a Job
// without it, and one pass of the holder takes the Job up.
func TestPersonJobPostWhileLeaseHeld(t *testing.T) {
	f := setup(t)
	body := jobBody(t, "J-1")
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	rec, err := PostPerson(f.s.Feed, personWriterOf("member"), feed.Input{Type: "job", Body: body, Job: "J-1"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Lease != nil || rec.Fence != 0 || rec.Author.Role != "member" {
		t.Fatalf("want an unleased person record, got lease=%v fence=%d role=%s", rec.Lease, rec.Fence, rec.Author.Role)
	}
	f.job(t, "J-2", []any{"J-1"})
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range Tree(snap) {
		found = found || row.ID == "J-1" && row.Role == "Person"
	}
	if !found {
		t.Fatal("the Tree does not show the person's Job after one tick")
	}
	if got := jobStatus(t, f, "J-2"); got != contract.Posted {
		t.Fatalf("J-2 is %s, want posted behind the person's open Job", got)
	}
}

// A person's post is refused when it names a Job the task does not have, as the
// leased post is.
func TestPersonJobPostChecksAfterTargets(t *testing.T) {
	f := setup(t)
	body := jobBody(t, "J-1")
	body["after"] = []any{"J-9"}
	before := recordCount(t, f)
	if _, err := PostPerson(f.s.Feed, personWriterOf("owner"), feed.Input{Type: "job", Body: body, Job: "J-1"}); err == nil {
		t.Fatal("a Job behind a missing Job was posted")
	}
	if recordCount(t, f) != before {
		t.Fatal("a refused post left a record")
	}
}

// Row 2: the lease is held; a person answers a person Job, the Job closes and
// the next pass releases the Jobs behind it.
func TestPersonAnswerWhileLeaseHeld(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	rec, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1", "Call it Lark.", f.now)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Lease != nil || rec.Fence != 0 || rec.Author.Role != "member" {
		t.Fatalf("want an unleased person record, got lease=%v fence=%d role=%s", rec.Lease, rec.Fence, rec.Author.Role)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Posted {
		t.Fatalf("J-1 is %s before the holder's pass; the person holds no lease to close it", got)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s", got)
	}
	if got := jobStatus(t, f, "J-2"); got != contract.Assigned {
		t.Fatalf("J-2 is %s, want assigned after the answer", got)
	}
	// The same refusals as a leased answer.
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1", "again", f.now); err == nil {
		t.Fatal("a closed Job took a second answer")
	}
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-3", "x", f.now); err == nil {
		t.Fatal("a worker Job took a person's answer")
	}
}

// Row 4: no agent role takes the unleased path, for a post or for an answer,
// and a refused write leaves the Feed as it was.
func TestUnleasedPersonWritesRefuseAgents(t *testing.T) {
	for _, role := range []string{"worker", "builder", "agent", "reviewer", "cli", "guest", "external"} {
		t.Run(role, func(t *testing.T) {
			f := personFixture(t)
			w := personWriterOf(role)
			if role == "guest" {
				// dana is the addressee of J-1 and may answer it (gy_guest_person_answer_test.go).
				w.Author.ID, w.Worker = "eve", "eve@"+machine
			}
			before := recordCount(t, f)
			if PersonWriter(w) {
				t.Fatalf("%s is a person writer", role)
			}
			if _, err := PostPerson(f.s.Feed, w, feed.Input{Type: "job", Body: jobBody(t, "J-9"), Job: "J-9"}); err == nil {
				t.Fatalf("%s posted a Job without the task lease", role)
			}
			if _, err := AnswerPerson(f.s.Feed, w, "J-1", "Call it Lark.", f.now); !errors.Is(err, feed.ErrStaleLease) {
				t.Fatalf("%s answered a person Job without the task lease: %v", role, err)
			}
			if recordCount(t, f) != before || jobStatus(t, f, "J-1") != contract.Posted {
				t.Fatalf("%s changed the Feed", role)
			}
		})
	}
	// An answer record an agent wrote by hand to the Job's record is not applied by the pass.
	f := personFixture(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"worker", "agent"} {
		w := personWriterOf(role)
		if _, err := f.s.Feed.Post(feed.Input{Type: "answer", Re: snap.State.Jobs["J-1"].History[0], Body: map[string]any{"answer": "Call it Lark."}}, w); err != nil {
			t.Fatal(err)
		}
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Posted {
		t.Fatalf("J-1 is %s; an agent's answer closed a person Job", got)
	}
	if !PersonWriter(personWriterOf("lead")) || !PersonWriter(personWriterOf("member")) || !PersonWriter(personWriterOf("owner")) {
		t.Fatal("a person of the team is not a person writer")
	}
	// A person who holds a lease is on the leased path, not this one.
	leased := personWriterOf("member")
	leased.Lease = "task"
	if PersonWriter(leased) {
		t.Fatal("a leased writer took the unleased path")
	}
}
