package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func gyGpGuest(id string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: id, Role: "guest"}, Worker: id + "@" + machine}
}

// gyGpForce appends one answer record to the Feed below the store, so none of the
// store's admission checks run: a copy of a member's answer, signed as guest eve.
func gyGpForce(t *testing.T, f *fixture, author string) feed.Record {
	t.Helper()
	scratch := personFixture(t)
	scratch.tick(t)
	rec, err := AnswerPerson(scratch.s.Feed, personWriterOf("member"), "J-1", "Call it Lark.", scratch.now)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	re := snap.State.Jobs["J-1"].History[0]
	rec.Re = &re
	if rec.ID, err = feed.NewID(); err != nil {
		t.Fatal(err)
	}
	rec.Author, rec.Tier = feed.Author{ID: author, Role: "guest"}, contract.PersonGuest
	rec.Worker = author + "@" + machine
	line, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(f.s.Feed.Dir(), fmt.Sprintf("%s.%d.jsonl", rec.Worker, rec.Fence)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(append(line, '\n'))
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func gyGpVerdict(t *testing.T, f *fixture, id string) string {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range snap.State.Verdicts {
		if v.ID == id {
			if v.Kind != "counted" {
				t.Logf("verdict of %s: %s, %s", id, v.Kind, v.Reason)
			}
			return v.Kind
		}
	}
	t.Fatalf("record %s is not in the Feed", id)
	return ""
}

// The addressee of a person Job answers it without a lease; the holder's next pass
// accepts the Job. Every other guest gets today's refusal and leaves no record.
func TestGyGpAnswerPersonGuest(t *testing.T) {
	f := personFixture(t)
	body := jobBody(t, "J-4")
	body["role"] = "Person"
	body["kind"] = "approve"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Approve the plan."}
	f.post(t, feed.Input{Type: "job", Body: body, Job: "J-4"}, f.s.Writer)
	f.tick(t)
	before := recordCount(t, f)
	for _, tc := range []struct{ name, id, job string }{
		{"another guest", "eve", "J-1"},
		{"a different case", "Dana", "J-1"},
		{"a worker Job", "dana", "J-3"},
		{"a plan approval", "dana", "J-4"},
	} {
		if _, err := AnswerPerson(f.s.Feed, gyGpGuest(tc.id), tc.job, "Yes", f.now); !errors.Is(err, feed.ErrStaleLease) {
			t.Fatalf("%s: want the stale lease refusal, got %v", tc.name, err)
		}
	}
	if recordCount(t, f) != before {
		t.Fatal("a refused guest answer left a record")
	}
	rec, err := AnswerPerson(f.s.Feed, gyGpGuest("dana"), "J-1", "Call it Lark.", f.now)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "answer" || rec.Author.ID != "dana" || rec.Author.Role != "guest" || rec.Lease != nil || rec.Fence != 0 {
		t.Fatalf("want an unleased answer by guest dana, got %+v", rec)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Posted {
		t.Fatalf("J-1 is %s before the holder's pass", got)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s after the holder's pass", got)
	}
	if got := f.state(t); got == nil {
		t.Fatal("no state")
	}
	s, _ := f.s.Feed.State()
	if s.Jobs["J-1"].Reason != "answered by dana" || jobStatus(t, f, "J-2") != contract.Assigned {
		t.Fatalf("reason %q, J-2 %s", s.Jobs["J-1"].Reason, jobStatus(t, f, "J-2"))
	}
	n := recordCount(t, f)
	if _, err := AnswerPerson(f.s.Feed, gyGpGuest("dana"), "J-1", "again", f.now); err == nil || recordCount(t, f) != n {
		t.Fatalf("an accepted Job took a second answer: %v", err)
	}
}

// Row 3: an answer by a guest who is not the addressee is refused by the store; one
// forced in below the store is skipped by the holder's pass and marked ignored.
func TestGyGpHandAnswerIgnored(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	re := snap.State.Jobs["J-1"].History[0]
	before := recordCount(t, f)
	_, err = f.s.Feed.Post(feed.Input{Type: "answer", Re: re, Body: map[string]any{"answer": "Call it Lark."}}, gyGpGuest("eve"))
	if err == nil || !strings.Contains(err.Error(), "guest answer must name a question addressed to that guest") {
		t.Fatalf("the store took a guest answer that is not addressed to that guest: %v", err)
	}
	if recordCount(t, f) != before {
		t.Fatal("the refused answer left a record")
	}
	forced := gyGpForce(t, f, "eve")
	if got := gyGpVerdict(t, f, forced.ID); got != "ignored" {
		t.Fatalf("the fold marked the forced answer %q, want ignored", got)
	}
	// The pass itself also skips it, even if it were counted.
	snap, err = Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	snap.Records[forced.ID] = forced
	n := recordCount(t, f)
	if err := f.s.exclusive(func() error { return f.s.personAnswers(snap) }); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Posted {
		t.Fatalf("J-1 is %s; a forced answer closed a person Job", got)
	}
	if recordCount(t, f) != n {
		t.Fatal("the pass wrote a message for a forced answer")
	}
	// The same record signed as the addressee is counted, so the filter is the author.
	ok := gyGpForce(t, f, "dana")
	if got := gyGpVerdict(t, f, ok.ID); got != "counted" {
		t.Fatalf("the fold marked the addressee's answer %q, want counted", got)
	}
}

func TestGyGpGuestAddresseeRule(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		role, author                     string
		leased                           bool
		jobRole, owner, kind             string
		want                             bool
	}{
		{"addressee", "guest", "dana", false, "Person", "dana", "ask", true},
		{"plugin prefix", "grid-guest", "dana", false, "Person", "dana", "", true},
		{"member", "member", "dana", false, "Person", "dana", "ask", false},
		{"leased", "guest", "dana", true, "Person", "dana", "ask", false},
		{"other author", "guest", "eve", false, "Person", "dana", "ask", false},
		{"case folded", "guest", "Dana", false, "Person", "dana", "ask", false},
		{"trimmed", "guest", "dana ", false, "Person", "dana", "ask", false},
		{"both empty", "guest", "", false, "Person", "", "ask", false},
		{"worker Job", "guest", "dana", false, "Worker", "dana", "ask", false},
		{"approve", "guest", "dana", false, "Person", "dana", "approve", false},
		{"Approve", "guest", "dana", false, "Person", "dana", " Approve", false},
	} {
		if got := contract.GuestAddressee(tc.role, tc.author, tc.leased, tc.jobRole, tc.owner, tc.kind); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
