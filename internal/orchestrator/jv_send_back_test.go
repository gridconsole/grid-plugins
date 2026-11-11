package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jvSendBackFixture is personFixture with J-1 an approve Job of dana, and J-4 a
// person Job of another kind.
func jvSendBackFixture(t *testing.T) *fixture {
	f := setup(t)
	body := jobBody(t, "J-1")
	body["role"] = "Person"
	body["kind"] = "approve"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Approve the plan."}
	f.post(t, feed.Input{Type: "job", Body: body, Job: "J-1"}, f.s.Writer)
	f.job(t, "J-2", []any{"J-1"})
	other := jobBody(t, "J-4")
	other["role"] = "Person"
	other["intent"] = map[string]any{"owner": "dana", "item": "i-2", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: other, Job: "J-4"}, f.s.Writer)
	return f
}

func jvSendBackWriter(f *fixture, id, role string) feed.Writer {
	machine := f.s.Writer.Worker[strings.Index(f.s.Writer.Worker, "@")+1:]
	return feed.Writer{Author: feed.Author{ID: id, Role: role}, Worker: id + "@" + machine}
}

func jvSendBackControl(t *testing.T, f *fixture, target string) map[string]any {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for i := len(r.Records) - 1; i >= 0; i-- {
		if r.Records[i].Body["target"] == target && r.Records[i].Body["status"] != nil {
			return r.Records[i].Body
		}
	}
	return nil
}

func jvSendBackCount(t *testing.T, f *fixture) int {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// Row 4: send_back acts only for a person of the team on an approve Job.
func TestJvSendBackTeamOnly(t *testing.T) {
	back := map[string]any{"send_back": true, "sent_back_hash": "abcdef"}
	f := jvSendBackFixture(t)
	f.tick(t)
	leased := f.s.Writer
	leased.Author = feed.Author{ID: "orchestrator", Role: "agent"}
	for name, w := range map[string]feed.Writer{"guest": jvSendBackWriter(f, "dana", "guest"), "leased orchestrator": leased} {
		n := jvSendBackCount(t, f)
		if _, err := AnswerPersonWith(f.s.Feed, w, "J-1", "split the parser", back, f.now); err == nil {
			t.Fatalf("%s sent the plan back", name)
		}
		if got := jvSendBackCount(t, f); got != n {
			t.Fatalf("%s wrote %d records", name, got-n)
		}
	}
	dana := jvSendBackWriter(f, "dana", "member")
	// Not an approve Job: a plain answer, neither key copied.
	rec, err := AnswerPersonWith(f.s.Feed, dana, "J-4", "Call it Lark.", back, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.Body["send_back"]; ok {
		t.Fatalf("answer %v", rec.Body)
	}
	f.tick(t)
	if c := jvSendBackControl(t, f, "J-4"); c == nil || c["status"] != "accepted" || c["send_back"] != nil || c["sent_back_hash"] != nil {
		t.Fatalf("J-4 control %v", c)
	}
	// The approve Job from a team person: rejected with the words.
	if _, err := AnswerPersonWith(f.s.Feed, dana, "J-1", "split the parser", back, f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Rejected {
		t.Fatalf("J-1 is %s", got)
	}
	c := jvSendBackControl(t, f, "J-1")
	if c == nil || c["close_reason"] != "sent back by dana" || c["answer"] != "split the parser" || c["sent_back_hash"] != "abcdef" || c["plan_hash"] != nil {
		t.Fatalf("J-1 control %v", c)
	}
	if got := jobStatus(t, f, "J-2"); got == contract.Assigned {
		t.Fatal("J-2 was assigned after J-1 was sent back")
	}
	// The leased path of a person of the team writes the same control at once.
	g := jvSendBackFixture(t)
	g.tick(t)
	if _, err := AnswerPersonWith(g.s.Feed, g.s.Writer, "J-1", "split the parser", back, g.now); err != nil {
		t.Fatal(err)
	}
	if got := jobStatus(t, g, "J-1"); got != contract.Rejected {
		t.Fatalf("leased: J-1 is %s", got)
	}
}
