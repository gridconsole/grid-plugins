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

// arFeedRows is what grid feed state --json prints for one Job's acceptance.
func arFeedRows(t *testing.T, h *harness, path, job string) []contract.AcceptanceRow {
	t.Helper()
	if code := h.run(path, "feed", "state", "--json"); code != 0 {
		t.Fatalf("feed state: %d %s", code, h.errb)
	}
	out := h.out.Bytes()
	if err := contract.Validate("cmd-feed-state", out); err != nil {
		t.Fatalf("cmd-feed-state: %v\n%s", err, out)
	}
	var state struct {
		Jobs map[string]struct {
			Acceptance []contract.AcceptanceRow `json:"acceptance"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(out, &state); err != nil {
		t.Fatal(err)
	}
	return state.Jobs[job].Acceptance
}

// A running worker marks three of its Job's five acceptance rows met with grid
// job post --type checkpoint, and grid feed state --json reads 3 of 5. A Job
// record posted before (one row, no state) reads pending, as old records do.
func TestArJobPostMarksRowsFeedStateReadsThreeOfFive(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", w.Fence))
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	var five map[string]any
	_ = json.Unmarshal(raw, &five)
	old["id"] = "J-2"
	rows := []any{}
	for i := 1; i <= 5; i++ {
		rows = append(rows, map[string]any{"id": fmt.Sprintf("A%d", i), "text": fmt.Sprintf("Row %d holds", i), "traces": "item"})
	}
	five["acceptance"] = rows
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: five}, w); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-2", Body: old}, w); err != nil {
		t.Fatal(err)
	}
	if got := arFeedRows(t, h, path, "J-2"); len(got) != 1 || got[0] != (contract.AcceptanceRow{ID: "A1", State: "pending"}) {
		t.Fatalf("old record rows %+v, want A1 pending", got)
	}
	if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"state":"assigned"`) {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
	if code := h.run(path, "job", "ack", "J-1", "--attempt", "1", "--json"); code != 0 {
		t.Fatalf("ack: %d %s", code, h.errb)
	}
	for _, row := range []string{"A1", "A3", "A4"} {
		mark := fmt.Sprintf(`{"job":"J-1","acceptance":{"id":%q,"state":"met","note":"go test exit 0"}}`, row)
		if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "checkpoint", "--body", mark); code != 0 {
			t.Fatalf("mark %s: %d %s", row, code, h.errb)
		}
	}
	got := arFeedRows(t, h, path, "J-1")
	met := 0
	for _, r := range got {
		if r.State == "met" {
			met++
		}
	}
	if met != 3 || len(got) != 5 || got[1].State != "pending" || got[4].State != "pending" {
		t.Fatalf("rows %+v: %d of %d, want 3 of 5 with A2 and A5 pending", got, met, len(got))
	}
	if got := arFeedRows(t, h, path, "J-2"); len(got) != 1 || got[0].State != "pending" {
		t.Fatalf("old record rows changed: %+v", got)
	}
}
