package orchestrator

import (
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

var approvalFieldsFixture = map[string]any{
	"plan_hash":   "ab12cd",
	"plan_digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	"lands_hash":  "ff00",
	"note":        "looks right",
}

func acceptingControl(t *testing.T, f *fixture, job string) map[string]any {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.Body["target"] == job && e.Body["status"] == "accepted" {
			return e.Body
		}
	}
	return nil
}

// A task-lease writer answers a person Job with the plan revision, the lands hash
// and the note: they are in the accepting control, which is the Job's result.
func TestAnswerPersonWithLeasedWriterPutsTheFieldsInTheControl(t *testing.T) {
	f := setup(t)
	f.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, f.s.Writer)
	if _, err := AnswerPersonWith(f.s.Feed, f.s.Writer, "J-90", "approved", approvalFieldsFixture, time.Now()); err != nil {
		t.Fatal(err)
	}
	body := acceptingControl(t, f, "J-90")
	if body == nil {
		t.Fatal("no accepting control")
	}
	for k, want := range approvalFieldsFixture {
		if body[k] != want {
			t.Errorf("control %s = %v, want %v", k, body[k], want)
		}
	}
}

// A person with no lease cannot write the control: the fields go into the answer,
// and the holder's next pass copies them, and the reason, into the accepting control.
func TestAnswerPersonWithUnleasedPersonCarriesTheFieldsToTheHoldersPass(t *testing.T) {
	f := setup(t)
	f.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, f.s.Writer)
	dana := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + machine}
	fields := map[string]any{"reason": "moved from approval.json: approved by erin"}
	for k, v := range approvalFieldsFixture {
		fields[k] = v
	}
	if _, err := AnswerPersonWith(f.s.Feed, dana, "J-90", "approved", fields, time.Now()); err != nil {
		t.Fatal(err)
	}
	if acceptingControl(t, f, "J-90") != nil {
		t.Fatal("an unleased answer accepted the Job by itself")
	}
	answers, err := f.s.Feed.Read(feed.Filter{Type: "answer"})
	if err != nil || len(answers.Records) != 1 {
		t.Fatalf("answers: %+v %v", answers, err)
	}
	for k, want := range approvalFieldsFixture {
		if answers.Records[0].Body[k] != want {
			t.Errorf("answer %s = %v, want %v", k, answers.Records[0].Body[k], want)
		}
	}
	f.tick(t)
	body := acceptingControl(t, f, "J-90")
	if body == nil {
		t.Fatal("the holder's pass did not accept the Job")
	}
	for k, want := range fields {
		if body[k] != want {
			t.Errorf("control %s = %v, want %v", k, body[k], want)
		}
	}
}
