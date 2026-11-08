package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestFeedJobPlannedFieldsRoundTrip(t *testing.T) {
	h, task := feedHarness(t)
	body, err := os.ReadFile("../../contract/fixtures/job-record/valid/planned-fields.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(body, &want); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "feed", "post", "--type", "job", "--job", "J-1", "--body", string(body), "--json"); code != 0 {
		t.Fatalf("post: %d %s", code, h.errb)
	}
	var posted struct {
		Post feed.Record `json:"post"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &posted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(map[string]any(posted.Post.Body), want) {
		t.Fatalf("posted body = %#v, want %#v", posted.Post.Body, want)
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
	record := read.Records[0]
	if record.ID != posted.Post.ID || record.Verdict != "counted" || !reflect.DeepEqual(map[string]any(record.Body), want) {
		t.Fatalf("persisted record = %+v", record)
	}
	if code := h.run(task, "feed", "state", "--json"); code != 0 {
		t.Fatalf("state: %d %s", code, h.errb)
	}
	var state contract.State
	if err := json.Unmarshal(h.out.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	// The fold projects selected Job fields; full bodies remain in feed read.
	j := state.Jobs["J-1"]
	if j == nil || j.ID != "J-1" || j.Title != want["title"] || j.Role != want["role"] || len(j.History) != 1 || j.History[0] != posted.Post.ID {
		t.Fatalf("folded Job = %+v", j)
	}
	if len(state.Verdicts) != 1 || state.Verdicts[0].Kind != "counted" {
		t.Fatalf("fold verdicts = %+v", state.Verdicts)
	}
}
