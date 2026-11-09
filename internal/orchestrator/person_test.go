package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// personFixture posts J-1 for a person, J-2 behind it, and J-3 independent.
func personFixture(t *testing.T) *fixture {
	f := setup(t)
	body := jobBody(t, "J-1")
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: body, Job: "J-1"}, f.s.Writer)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", nil)
	return f
}

func jobStatus(t *testing.T, f *fixture, id string) contract.Status {
	t.Helper()
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	return s.Jobs[id].Status
}

func TestPersonJobHoldsOnlyThoseBehindIt(t *testing.T) {
	f := personFixture(t)
	out := f.tick(t)
	if out.Assigned != 1 {
		t.Fatalf("want only the independent Job assigned, got %d: %+v", out.Assigned, out.Rows)
	}
	if got := jobStatus(t, f, "J-3"); got != contract.Assigned {
		t.Fatalf("independent Job J-3 is %s", got)
	}
	for _, id := range []string{"J-1", "J-2"} {
		if got := jobStatus(t, f, id); got != contract.Posted {
			t.Fatalf("%s is %s, want posted while the person has not answered", id, got)
		}
	}
}

func TestPersonAnswerReleasesDependents(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-1", "Call it Lark.", f.now); err != nil {
		t.Fatal(err)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s", got)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-2"); got != contract.Assigned {
		t.Fatalf("J-2 is %s, want assigned after the answer", got)
	}
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range r.Records {
		if e.Type == "post" {
			text, _ := e.Body["text"].(string)
			if strings.Contains(text, "lead answered J-1 at ") && strings.Contains(text, "Call it Lark.") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the answer, who and when were not recorded in the Feed")
	}
}

func TestPersonAnswerRefusals(t *testing.T) {
	f := personFixture(t)
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-3", "x", f.now); err == nil {
		t.Fatal("a worker Job took a person's answer")
	}
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-1", " ", f.now); err == nil {
		t.Fatal("an empty answer was taken")
	}
}
