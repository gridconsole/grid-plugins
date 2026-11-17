package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// pjVbFeed is a task whose lease the test holds, as contractCommandFeed sets it up, with the
// builder J-4 posted. It answers the harness, the task folder and the record id of J-4's post.
func pjVbFeed(t *testing.T) (*harness, string, string) {
	t.Helper()
	h, task := feedHarness(t)
	h.env.Machine = feedMachine
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	writer := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(writer, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", grant.Fence))
	pjVbRun(t, h, task, "feed", "handover", "--reason", "Example handover", "--lease", "task", "--json")
	out := pjVbRun(t, h, task, "job", "post", "J-4", "--type", "job", "--body", pjVbBody(t, "J-4", nil), "--json")
	var rec struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &rec); err != nil || rec.ID == "" {
		t.Fatalf("job post printed %q (%v)", out, err)
	}
	return h, task, rec.ID
}

func pjVbRun(t *testing.T, h *harness, task string, args ...string) string {
	t.Helper()
	if code := h.run(task, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return h.out.String()
}

// pjVbBody is the fixture job record under id, with the fields of set laid over it.
func pjVbBody(t *testing.T, id string, set map[string]any) string {
	t.Helper()
	data, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = id
	for k, v := range set {
		body[k] = v
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// pjVbRecords are the records on the task's Feed.
func pjVbRecords(t *testing.T, task string) []feed.Record {
	t.Helper()
	read, err := (feed.Store{TaskDir: task, TaskID: feedTaskID}).Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Record
	for _, e := range read.Records {
		out = append(out, e.Record)
	}
	return out
}

// Row 4: a second `grid judge review` with the same wake exits 0 and posts nothing: the wake is
// already answered (ErrDuplicateWake), and the Feed holds no new record.
func TestPjVbJudgeReviewSameWakePostsNothing(t *testing.T) {
	h, task, event := pjVbFeed(t)
	wake := event + "/1"
	out := pjVbRun(t, h, task, "judge", "review", "J-4", "--wake", wake, "--json")
	var first struct {
		Posted []struct {
			ID, Type, Job string
		} `json:"posted"`
	}
	if err := json.Unmarshal([]byte(out), &first); err != nil || len(first.Posted) != 1 || first.Posted[0].Job != "J-4.1" || first.Posted[0].Type != "job" {
		t.Fatalf("first review printed %q (%v), want one job post J-4.1", out, err)
	}
	var review *feed.Record
	before := pjVbRecords(t, task)
	for i, r := range before {
		if r.Type == "job" && r.Job != nil && *r.Job == "J-4.1" {
			review = &before[i]
		}
	}
	if review == nil || review.Body["role"] != "reviewer" || review.Body["discovered_from"] != "J-4" || review.Wake == nil || *review.Wake != wake {
		t.Fatalf("review record %+v, want a reviewer Job discovered from J-4 carrying the wake", review)
	}
	if review.ID != first.Posted[0].ID {
		t.Fatalf("the review record is %s, the verb printed %s", review.ID, first.Posted[0].ID)
	}

	code := h.run(task, "judge", "review", "J-4", "--wake", wake)
	if code != 0 {
		t.Fatalf("second review with the same wake: exit %d: %s%s", code, h.out, h.errb)
	}
	if !strings.Contains(h.out.String(), "nothing posted") || !strings.Contains(h.out.String(), review.ID) {
		t.Fatalf("second review printed %q, want nothing posted naming %s", h.out, review.ID)
	}
	if after := pjVbRecords(t, task); len(after) != len(before) {
		t.Fatalf("the Feed holds %d records after the second call, %d before", len(after), len(before))
	}
}

// Row 5 on a real Feed: the review J-4.1 of J-4 is cancelled before it ran; three calls of
// `grid judge person J-4 --from J-4.1` (two wakes and none) raise exactly one person Job.
func TestPjVbJudgePersonForACancelledReviewIsRaisedOnce(t *testing.T) {
	h, task, event := pjVbFeed(t)
	pjVbRun(t, h, task, "job", "post", "J-4.1", "--type", "job", "--json", "--body", pjVbBody(t, "J-4.1", map[string]any{
		"role": "reviewer", "discovered_from": "J-4", "verify": map[string]any{"kind": "reviewer-check", "job": "J-4.1"},
	}))
	pjVbRun(t, h, task, "job", "cancel", "J-4.1", "--reason", "prepare_failed: the agent could not be built")
	for _, wake := range []string{event + "/1", event + "/2", ""} {
		args := []string{"judge", "person", "J-4", "--from", "J-4.1", "--owner", "owner-1"}
		if wake != "" {
			args = append(args, "--wake", wake)
		}
		pjVbRun(t, h, task, args...)
	}
	var persons []feed.Record
	for _, r := range pjVbRecords(t, task) {
		if r.Type == "job" && r.Body["role"] == "Person" {
			persons = append(persons, r)
		}
	}
	if len(persons) != 1 {
		t.Fatalf("%d person Jobs after three calls, want exactly one: %+v", len(persons), persons)
	}
	p := persons[0].Body
	intent, _ := p["intent"].(map[string]any)
	goal, _ := intent["goal"].(string)
	if p["discovered_from"] != "J-4" || intent["owner"] != "owner-1" {
		t.Fatalf("person Job %v, want owner-1 and discovered from J-4", p)
	}
	for _, want := range []string{"J-4.1", "prepare_failed: the agent could not be built", "accept J-4 without a review", "ask for rework"} {
		if !strings.Contains(goal, want) {
			t.Fatalf("goal %q does not name %q", goal, want)
		}
	}
}
