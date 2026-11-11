package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func pfMsgWriter(id, role string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: id, Role: role}, Worker: id + "@" + machine}
}

// pfMsgSay posts a message to the task as a person, with no lease and no job.
func pfMsgSay(t *testing.T, f *fixture, w feed.Writer, text string) feed.Record {
	t.Helper()
	return f.post(t, feed.Input{Type: "post", Body: map[string]any{"kind": "message", "ref": "task", "text": text}}, w)
}

// pfMsgJobs returns every job record on the Feed, oldest first.
func pfMsgJobs(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	return r.Records
}

// pfMsgOrchestrators returns the orchestrator Jobs by the post id they were
// discovered from.
func pfMsgOrchestrators(t *testing.T, f *fixture) map[string]feed.Entry {
	t.Helper()
	out := map[string]feed.Entry{}
	for _, e := range pfMsgJobs(t, f) {
		if e.Body["role"] == gkReplanRole {
			from, _ := e.Body["discovered_from"].(string)
			out[from] = e
		}
	}
	return out
}

func pfMsgTicks(t *testing.T, f *fixture, n int) {
	t.Helper()
	for range n {
		f.tick(t)
	}
}

func TestPfMsgOwnerMessageGetsOneJob(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	post := pfMsgSay(t, f, pfMsgWriter("owner", "owner"), "Please also add a dark theme")
	if got := pfMsgOrchestrators(t, f); len(got) != 0 {
		t.Fatalf("orchestrator Jobs before any pass: %v", got)
	}
	pfMsgTicks(t, f, 2)
	jobs := pfMsgJobs(t, f)
	if len(jobs) != 2 {
		t.Fatalf("want J-1 and one new job record, got %d", len(jobs))
	}
	got := pfMsgOrchestrators(t, f)
	e, ok := got[post.ID]
	if len(got) != 1 || !ok {
		t.Fatalf("want one orchestrator Job discovered_from %s, got %v", post.ID, got)
	}
	if e.Body["title"] != "Read the person's message: Please also add a dark theme" {
		t.Fatalf("title = %v", e.Body["title"])
	}
	data, err := json.Marshal(e.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the body on the Feed is not a job-record: %v", err)
	}
	if !strings.Contains(string(data), "Please also add a dark theme") {
		t.Fatalf("the acceptance text does not quote the message: %s", data)
	}
}

func TestPfMsgNotARequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		post func(t *testing.T, f *fixture)
	}{
		{"guest message", func(t *testing.T, f *fixture) {
			pfMsgSay(t, f, pfMsgWriter("guest", "guest"), "Please add a dark theme")
		}},
		{"owner message to a job", func(t *testing.T, f *fixture) {
			pfMsgSay(t, f, pfMsgWriter("owner", "owner"), "To J-1: use the small font")
		}},
		{"tick note with the task lease", func(t *testing.T, f *fixture) {
			f.post(t, feed.Input{Type: "post", Body: map[string]any{"kind": "message", "ref": "task", "text": "Warning: a note from the tick"}}, f.s.Writer)
		}},
		{"answer notice with its control", func(t *testing.T, f *fixture) {
			said := pfMsgSay(t, f, pfMsgWriter("owner", "owner"), "owner answered J-1 at 2026-11-11T10:00:00Z: Proceed")
			f.post(t, feed.Input{Type: "control", Re: said.ID, Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "answered by owner", "answer": "Proceed"}}, pfMsgWriter("owner", "owner"))
		}},
		{"close post", func(t *testing.T, f *fixture) {
			f.post(t, feed.Input{Type: "post", Body: map[string]any{"kind": "close", "ref": "task", "text": "All done"}}, pfMsgWriter("owner", "owner"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			tc.post(t, f)
			pfMsgTicks(t, f, 2)
			if got := pfMsgOrchestrators(t, f); len(got) != 0 {
				t.Fatalf("orchestrator Jobs for a post that is no request: %v", got)
			}
		})
	}
}

func TestPfMsgTwoMessagesTwoJobs(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	owner := pfMsgWriter("owner", "owner")
	first := pfMsgSay(t, f, owner, "First request")
	second := pfMsgSay(t, f, owner, "Second request")
	pfMsgTicks(t, f, 2)
	got := pfMsgOrchestrators(t, f)
	a, b := got[first.ID], got[second.ID]
	if len(got) != 2 || a.Body["id"] == nil || b.Body["id"] == nil || a.Body["id"] == b.Body["id"] {
		t.Fatalf("want two Jobs with distinct ids, got %v", got)
	}
	before := len(pfMsgJobs(t, f))
	pfMsgTicks(t, f, 1)
	if after := len(pfMsgJobs(t, f)); after != before {
		t.Fatalf("a third pass posted %d job records", after-before)
	}
}

func TestPfMsgJobSetsOffNoOtherReplan(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	pfMsgSay(t, f, pfMsgWriter("owner", "owner"), "One request")
	pfMsgTicks(t, f, 3)
	jobs := pfMsgJobs(t, f)
	if len(jobs) != 2 {
		ids := []any{}
		for _, e := range jobs {
			ids = append(ids, e.Body["id"])
		}
		t.Fatalf("want J-1 and the message Job only, got %v", ids)
	}
}
