package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

const jfTakenPrefix = "plan.md's Job"

// jfTakenPosts are the post records of the Feed.
func jfTakenPosts(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "post"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// jfTakenNotices are the posts whose text starts with the notice words.
func jfTakenNotices(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	var out []feed.Entry
	for _, r := range jfTakenPosts(t, f) {
		if text, _ := r.Body["text"].(string); strings.HasPrefix(text, jfTakenPrefix) {
			out = append(out, r)
		}
	}
	return out
}

// jfTakenPlanBody is the record body of the plan's own J-1.
func jfTakenPlanBody(t *testing.T, f *fixture) map[string]any {
	t.Helper()
	j, err := job.LoadPlan(f.s.Feed.TaskDir, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestJfTakenMemberPostGetsOneNotice(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, plannedText(t))
	body := jobBody(t, "J-1")
	body["role"] = "Person"
	body["title"] = "Dana's own"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	if _, err := PostPerson(f.s.Feed, personWriterOf("member"), feed.Input{Type: "job", Body: body, Job: "J-1"}); err != nil {
		t.Fatal(err)
	}
	approvePlan(t, f)
	f.tick(t)
	f.tick(t)
	notices := jfTakenNotices(t, f)
	if len(notices) != 1 {
		t.Fatalf("want one notice, got %d: %+v", len(notices), notices)
	}
	n := notices[0]
	want := "plan.md's Job J-1 was not posted: dana already posted a Job with that id."
	if n.Lease == nil || *n.Lease != "task" || n.Body["kind"] != "message" || n.Body["ref"] != "task" || n.Body["text"] != want {
		t.Fatalf("notice: lease=%v body=%v", n.Lease, n.Body)
	}
	if strings.Contains(n.Body["text"].(string), "Dana's own") {
		t.Fatal("notice carries a title")
	}
	records := planJobRecords(t, f)
	if len(records) != 1 || records[0].Author.ID != "dana" {
		t.Fatalf("want the member's one J-1 record, got %+v", records)
	}
}

// A person posting the notice's text without the lease does not silence it.
func TestJfTakenPersonCopyOfTextDoesNotSilence(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, plannedText(t))
	body := jobBody(t, "J-1")
	body["role"] = "Person"
	body["title"] = "Dana's own"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	if _, err := PostPerson(f.s.Feed, personWriterOf("member"), feed.Input{Type: "job", Body: body, Job: "J-1"}); err != nil {
		t.Fatal(err)
	}
	text := "plan.md's Job J-1 was not posted: dana already posted a Job with that id."
	if _, err := f.s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, personWriterOf("member")); err != nil {
		t.Fatal(err)
	}
	approvePlan(t, f)
	f.tick(t)
	leased := 0
	for _, r := range jfTakenNotices(t, f) {
		if r.Lease != nil && *r.Lease == "task" {
			leased++
		}
	}
	if leased != 1 {
		t.Fatalf("want one leased notice, got %d", leased)
	}
}

func TestJfTakenHolderPublicationGetsNone(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, plannedText(t))
	approvePlan(t, f)
	f.post(t, feed.Input{Type: "job", Job: "J-1", Body: jfTakenPlanBody(t, f)}, f.s.Writer)
	f.tick(t)
	if n := jfTakenNotices(t, f); len(n) != 0 {
		t.Fatalf("holder's own Job gave a notice: %+v", n)
	}
}

func TestJfTakenSameTitleAndRoleGetsNone(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, plannedText(t))
	if _, err := PostPerson(f.s.Feed, personWriterOf("member"), feed.Input{Type: "job", Job: "J-1", Body: jfTakenPlanBody(t, f)}); err != nil {
		t.Fatal(err)
	}
	approvePlan(t, f)
	f.tick(t)
	f.tick(t)
	if n := jfTakenNotices(t, f); len(n) != 0 {
		t.Fatalf("same title and role gave a notice: %+v", n)
	}
}
