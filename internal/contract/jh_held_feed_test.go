package contract_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

const (
	jhFeedTask    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	jhFeedMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
)

// jhFeed is a real Feed with J-1 posted and a lead holding the task lease.
func jhFeed(t *testing.T) (feed.Store, feed.Writer) {
	t.Helper()
	store := feed.Store{TaskDir: t.TempDir(), TaskID: jhFeedTask}
	w := feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@" + jhFeedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = "J-1"
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	return store, w
}

// A held control with waits_on goes through the Feed's own validation, folds the
// Job to held with its waits, and the state grid feed state prints for it (the
// folded state as JSON) validates against cmd-feed-state and reads status held,
// state Started and the wait with its kind and ref.
func TestHoldControlPostsThroughTheFeedAndShowsInState(t *testing.T) {
	store, w := jhFeed(t)
	waits := []any{map[string]any{"row": "A3", "kind": "landed", "ref": "J-6"}}
	if _, err := store.Post(feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "held", "reason": "held by J-1.1", "waits_on": waits}}, w); err != nil {
		t.Fatalf("a held control with waits_on was refused: %v", err)
	}
	for name, bad := range map[string][]any{
		"unknown kind": {map[string]any{"row": "A3", "kind": "merge", "ref": "J-6"}},
		"no ref":       {map[string]any{"row": "A3", "kind": "landed"}},
		"extra key":    {map[string]any{"row": "A3", "kind": "landed", "ref": "J-6", "met": true}},
	} {
		if _, err := store.Post(feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "held", "reason": "held", "waits_on": bad}}, w); err == nil {
			t.Errorf("%s: the Feed took the control", name)
		}
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	if j.Status != contract.Held || !reflect.DeepEqual(j.WaitsOn, []contract.HeldWait{{Row: "A3", Kind: contract.WaitLanded, Ref: "J-6"}}) {
		t.Fatalf("Job = %s waits %+v, want held on landed J-6", j.Status, j.WaitsOn)
	}
	out, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("cmd-feed-state", out); err != nil {
		t.Fatalf("cmd-feed-state: %v\n%s", err, out)
	}
	var got struct {
		Jobs map[string]struct {
			Status      string                            `json:"status"`
			State       string                            `json:"state"`
			StateReason string                            `json:"state_reason"`
			WaitsOn     []struct{ Row, Kind, Ref string } `json:"waits_on"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	row := got.Jobs["J-1"]
	if row.Status != "held" || row.State != "Started" || row.StateReason != "held" || len(row.WaitsOn) != 1 || row.WaitsOn[0].Kind != "landed" || row.WaitsOn[0].Ref != "J-6" {
		t.Fatalf("feed state row = %+v, want held, Started, waiting on landed J-6", row)
	}
}

// The job record takes status held too.
func TestHoldJobRecordTakesHeld(t *testing.T) {
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	rec["status"] = "held"
	data, _ := json.Marshal(rec)
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("job-record with status held: %v", err)
	}
}
