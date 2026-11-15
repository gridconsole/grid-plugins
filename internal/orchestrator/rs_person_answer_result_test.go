package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// rsResults is what `grid feed read --job <job> --type result` shows
// (internal/cli/feed.go:113 reads the Feed with this filter).
func rsResults(t *testing.T, f *fixture, job string) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Job: job, Type: "result"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// rsReadsAnswer checks that the Job behind the person Job J-1 finds the person's
// decision where it reads any dependency's outcome: exactly one record, the one
// that closed J-1, carrying the answer text once.
func rsReadsAnswer(t *testing.T, f *fixture, want string) {
	t.Helper()
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s", got)
	}
	if got := jobStatus(t, f, "J-2"); got == contract.Posted {
		t.Fatalf("J-2 still waits on the answered J-1")
	}
	got := rsResults(t, f, "J-1")
	if len(got) != 1 {
		t.Fatalf("the dependent Job reading J-1 by result wants one record with the answer, got %d: %+v", len(got), got)
	}
	if a, _ := got[0].Body["answer"].(string); a != want {
		t.Fatalf("the record read for J-1 carries answer %q, want %q: %+v", a, want, got[0])
	}
	if target, _ := got[0].Body["target"].(string); target != "J-1" || got[0].Verdict != "counted" {
		t.Fatalf("the record read for J-1 is not its counted closing: %+v", got[0])
	}
	// A worker Job's result read is unchanged: J-3 posted none, so none is shown.
	if other := rsResults(t, f, "J-3"); len(other) != 0 {
		t.Fatalf("J-3 has no result, got %+v", other)
	}
}

// Round 10 P3: the lease holder records the person's answer and closes J-1.
func TestRsPersonAnswerReadAsResultLeased(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-1", "Call it Lark.", f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	rsReadsAnswer(t, f, "Call it Lark.")
}

// Round 10 P3: a person answers without the lease; the holder's next pass closes
// J-1 with the answer, and the answer record itself is not shown a second time.
func TestRsPersonAnswerReadAsResultUnleased(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1", "Call it Wren.", f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	rsReadsAnswer(t, f, "Call it Wren.")
}
