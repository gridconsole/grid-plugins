package orchestrator

import (
	"slices"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// qtPkExit posts a worker-exit for J-1 with the given body, as the fixture's
// worker, and returns its record id.
func qtPkExit(t *testing.T, f *fixture, body map[string]any) string {
	t.Helper()
	j := f.state(t)
	r, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	return r.ID
}

// qtPkQuestion returns the one question post on the Feed.
func qtPkQuestion(t *testing.T, f *fixture) feed.Entry {
	t.Helper()
	posts := jrParkPosts(t, f)
	if len(posts) != 1 {
		t.Fatalf("%d question posts on the Feed, want 1", len(posts))
	}
	return posts[0]
}

// qtPkCheckQuestion holds the parts of the question that do not change: text,
// options, recommended, and a Job that ends Parked with needs_person.
func qtPkCheckQuestion(t *testing.T, f *fixture) {
	t.Helper()
	q := qtPkQuestion(t, f)
	if got := q.Body["question"]; got != "Three attempts made no progress. How should J-1 continue?" {
		t.Fatalf("question %q", got)
	}
	r := jrParkDecode(t, f, "Q-1.md")
	if !slices.Equal(r.Options, []string{"Keep parked", "Retry"}) || r.Recommended != "Keep parked" {
		t.Fatalf("options %v recommended %q", r.Options, r.Recommended)
	}
	j := f.state(t)
	if j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("J-1 %q reason %q, want parked and needs_person", j.Status, j.Reason)
	}
}

func TestQtPkWhyWalk(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for _, body := range []map[string]any{
		{"reason": "crash", "code": 1},
		{"reason": "crash", "code": "claude not found"},
		{"reason": "crash"},
	} {
		f.ack(t)
		qtPkExit(t, f, body)
		f.tick(t)
	}
	want := "Attempt 1: crashed, exit 1; Attempt 2: claude not found; Attempt 3: crashed, no exit code reported"
	if got := qtPkQuestion(t, f).Body["why"]; got != want {
		t.Fatalf("why %q, want %q", got, want)
	}
	if got := jrParkDecode(t, f, "Q-1.md").Why; got != want {
		t.Fatalf("file why %q, want %q", got, want)
	}
	qtPkCheckQuestion(t, f)
}

func TestQtPkWhyDirect(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	exit := qtPkExit(t, f, map[string]any{"reason": "crash", "code": 1})
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	// The Job post has no worker-exit body: the dead-lease case.
	jobPost := ""
	for id, r := range snap.Records {
		if r.Type == "job" && r.Job != nil && *r.Job == "J-1" {
			jobPost = id
		}
	}
	if jobPost == "" {
		t.Fatal("no job post for J-1")
	}
	failures := []string{exit, jobPost, "r-missing"}
	m := &jobMemory{Failure: exit, Failures: failures}
	if err := f.s.park(snap, snap.State.Jobs["J-1"], m); err != nil {
		t.Fatal(err)
	}
	want := "Attempt 1: crashed, exit 1; Attempt 2: the agent stopped answering; Attempt 3: r-missing"
	if got := qtPkQuestion(t, f).Body["why"]; got != want {
		t.Fatalf("why %q, want %q", got, want)
	}
	qtPkCheckQuestion(t, f)
}

// A reason other than crash gives the reason word, and the code when there is one.
func TestQtPkWhyReasonWords(t *testing.T) {
	exit := func(body feed.Body) feed.Record { return feed.Record{Type: "worker-exit", Body: body} }
	snap := Snapshot{Records: map[string]feed.Record{
		"e1": exit(feed.Body{"reason": "loop"}),
		"e2": exit(feed.Body{"reason": "loop", "code": float64(3)}),
		"e3": exit(feed.Body{"reason": "cancelled", "code": "stopped by hand"}),
		"e4": exit(feed.Body{}),
		"e5": exit(feed.Body{"reason": "crash", "code": float64(137)}),
		"e6": exit(feed.Body{"code": "oom"}),
	}}
	want := "Attempt 1: loop; Attempt 2: loop, exit 3; Attempt 3: cancelled, stopped by hand; " +
		"Attempt 4: crashed, no exit code reported; Attempt 5: crashed, exit 137; Attempt 6: oom"
	if got := qtPkWhy(snap, []string{"e1", "e2", "e3", "e4", "e5", "e6"}); got != want {
		t.Fatalf("why %q, want %q", got, want)
	}
}
