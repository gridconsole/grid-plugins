package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// A land Job's record (kind land, role runner, target, branch and sha) posts to
// the Feed, reads back whole from grid feed read, and folds into grid feed state
// with its kind. The record is built from job.Job, so the Job's own field tags
// are what the Feed holds.
func TestFeedLandJobRoundTrip(t *testing.T) {
	h, task := feedHarness(t)
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var j job.Job
	if err := json.Unmarshal(raw, &j); err != nil {
		t.Fatal(err)
	}
	j.ID, j.Kind, j.Role = "J-7", "land", "runner"
	j.Worktree = nil // a land Job runs in a worktree the verb makes
	j.Target, j.Branch, j.SHA = "main", "grid-T-01-J-3-a1", "0123456789abcdef0123456789abcdef01234567"
	body, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("job-record: %v\n%s", err, body)
	}
	var want map[string]any
	if err := json.Unmarshal(body, &want); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"kind": "land", "target": "main", "branch": "grid-T-01-J-3-a1", "sha": j.SHA} {
		if want[key] != value {
			t.Fatalf("record %s = %v, want %q", key, want[key], value)
		}
	}
	if code := h.run(task, "feed", "post", "--type", "job", "--job", "J-7", "--body", string(body), "--json"); code != 0 {
		t.Fatalf("post: %d %s", code, h.errb)
	}
	var posted struct {
		Post feed.Record `json:"post"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &posted); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "feed", "read", "--type", "job", "--json"); code != 0 {
		t.Fatalf("read: %d %s", code, h.errb)
	}
	var read feed.ReadResult
	if err := json.Unmarshal(h.out.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	if len(read.Bad) != 0 || len(read.Records) != 1 {
		t.Fatalf("read = %+v", read)
	}
	if record := read.Records[0]; record.ID != posted.Post.ID || record.Verdict != "counted" || !reflect.DeepEqual(map[string]any(record.Body), want) {
		t.Fatalf("persisted record = %+v\nwant body %#v", record, want)
	}
	if code := h.run(task, "feed", "state", "--json"); code != 0 {
		t.Fatalf("state: %d %s", code, h.errb)
	}
	if err := contract.Validate("cmd-feed-state", h.out.Bytes()); err != nil {
		t.Fatalf("cmd-feed-state: %v\n%s", err, h.out)
	}
	var state contract.State
	if err := json.Unmarshal(h.out.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if got := state.Jobs["J-7"]; got == nil || got.Kind != "land" || got.Role != "runner" {
		t.Fatalf("folded Job = %+v, want kind land and role runner", got)
	}
}
