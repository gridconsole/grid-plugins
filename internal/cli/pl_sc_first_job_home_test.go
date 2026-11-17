package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// plScFirstJob reads J-1 of a task folder from its Feed and returns the record's
// writer id and its decoded body.
func plScFirstJob(t *testing.T, folder string) (string, map[string]any) {
	t.Helper()
	id, err := feed.TaskID(folder)
	if err != nil {
		t.Fatal(err)
	}
	read, err := feed.Store{TaskDir: folder, TaskID: id}.Read(feed.Filter{Type: "job"})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("want one job record, got %d: %v", len(read.Records), err)
	}
	rec := read.Records[0]
	if rec.Job == nil || *rec.Job != "J-1" {
		t.Fatalf("the job record is not J-1: %v", rec.Job)
	}
	data, err := json.Marshal(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("J-1 is not a valid job-record: %v\n%s", err, data)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return rec.Worker, body
}

// TestPlScFirstJobHome is V9: the first Job record carries home, the machine id
// of the writer that posted it (brain-board.md decision point 4).
func TestPlScFirstJobHome(t *testing.T) {
	h := newTaskHarness(t)
	folder := newTask(t, h)
	worker, body := plScFirstJob(t, folder)
	_, machine, ok := strings.Cut(worker, "@")
	if !ok || machine == "" {
		t.Fatalf("J-1 writer %q has no machine", worker)
	}
	if body["home"] != machine {
		t.Fatalf("J-1 home = %v, want the writer's machine %q", body["home"], machine)
	}
}
