package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// gqWorkflowJobs returns the job records on the Feed of the task at path.
func gqWorkflowJobs(t *testing.T, path string) []feed.Entry {
	t.Helper()
	id, err := feed.TaskID(path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := (feed.Store{TaskDir: path, TaskID: id}).Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// gqWorkflowBody returns the body of the one job record as a map.
func gqWorkflowBody(t *testing.T, rec feed.Entry) map[string]any {
	t.Helper()
	data, err := json.Marshal(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// A task made with a workflow gets the same J-1 as one made without.
func TestTaskNewWithWorkflowPostsTheFirstJob(t *testing.T) {
	h := newTaskHarness(t)
	path := newTask(t, h, "--workflow", "build")
	recs := gqWorkflowJobs(t, path)
	if len(recs) != 1 || recs[0].Job == nil || *recs[0].Job != "J-1" {
		t.Fatalf("a task made with a workflow must have J-1, got %d job records", len(recs))
	}
	body := gqWorkflowBody(t, recs[0])
	if body["role"] != "orchestrator" {
		t.Fatalf("J-1 role = %v, want orchestrator", body["role"])
	}
	read, _ := body["read"].([]any)
	if len(read) != 1 || read[0] != "intent.md#request" {
		t.Fatalf("J-1 read = %v, want [intent.md#request]", body["read"])
	}

	// The same body as a task made without a workflow.
	plain := gqWorkflowBody(t, gqWorkflowJobs(t, newTask(t, h))[0])
	want, _ := json.Marshal(plain)
	got, _ := json.Marshal(body)
	if string(want) != string(got) {
		t.Fatalf("J-1 differs with a workflow:\nwith:    %s\nwithout: %s", got, want)
	}

	// The worker's stream finds the Job and spawns it.
	if lines := streamPass(t, h); !strings.Contains(lines, `"J-1"`) {
		t.Fatalf("one stream pass emitted no spawn for J-1: %q\nstderr: %s", lines, h.errb)
	}
}

func TestTaskNewWithWorkflowFirstJobHasNoFindings(t *testing.T) {
	h := newTaskHarness(t)
	path := newTask(t, h, "--workflow", "build")
	recs := gqWorkflowJobs(t, path)
	if len(recs) != 1 {
		t.Fatalf("want one job record, got %d", len(recs))
	}
	findings, err := postFindings(h.env, path, gqWorkflowBody(t, recs[0]))
	if err != nil || len(findings) != 0 {
		t.Fatalf("J-1 of a workflow task has findings %v, err %v", findings, err)
	}
}

func TestTaskNewTwiceEachTaskGetsItsOwnFirstJob(t *testing.T) {
	h := newTaskHarness(t)
	first := newTask(t, h, "--workflow", "build")
	h.out.Reset()
	second := newTask(t, h, "--workflow", "build")
	if first == second {
		t.Fatalf("both tasks share the folder %s", first)
	}
	for _, path := range []string{first, second} {
		recs := gqWorkflowJobs(t, path)
		if len(recs) != 1 || recs[0].Job == nil || *recs[0].Job != "J-1" {
			t.Fatalf("%s: want its own J-1, got %d job records", path, len(recs))
		}
	}
}
