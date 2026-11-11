package cli

import (
	"encoding/json"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

const joRpTitle = "Approve the plan again after the reopen"

// joRpClosed is reopenedPlan up to and including `task close T-07`: the plan Job
// J-2 is approved and the task is closed, so the next step is the reopen.
func joRpClosed(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store, text := approveJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	if code := h.run(path, "task", "close", "T-07"); code != 0 {
		t.Fatalf("task close: %d %s", code, h.errb.String())
	}
	return h, path, store
}

// joRpBody is a person Job body of the given id and title.
func joRpBody(t *testing.T, id, title string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["title"] = title
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// joRpIDs counts the `job` records of the Feed by body id, whatever their verdict.
func joRpIDs(t *testing.T, store feed.Store) map[string]int {
	t.Helper()
	r, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int{}
	for _, e := range r.Records {
		id, _ := e.Body["id"].(string)
		ids[id]++
	}
	return ids
}

// joRpReopenJob is the open Job that carries the reopen title, or nil.
func joRpReopenJob(t *testing.T, store feed.Store) *contract.Job {
	t.Helper()
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range state.Jobs {
		if j.Title == joRpTitle && j.Status != contract.Accepted && j.Status != contract.Cancelled && j.Status != contract.Rejected {
			return j
		}
	}
	return nil
}

// A `job` record whose after names a record not on this Feed is held and kept out of
// State, so its id is invisible to the first pick. The reopen still takes a free id.
func TestJoRpHeldIDSkipped(t *testing.T) {
	h, path, store := joRpClosed(t)
	missing, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	// feed post takes the task lease on the command line; without it the record has
	// no lease and the schema refuses it (exit 3).
	if code := h.run(path, "feed", "post", "--type", "job", "--job", "J-3", "--lease", "task", "--after", missing, "--body", joRpBody(t, "J-3", "Planted")); code != 0 {
		t.Fatalf("planting the held J-3: %d %s", code, h.errb.String())
	}
	r, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	held := false
	for _, e := range r.Records {
		if id, _ := e.Body["id"].(string); id == "J-3" && e.Verdict == "held" {
			held = true
		}
	}
	if !held {
		t.Fatalf("the planted J-3 is not held: %+v", r.Records)
	}
	if code := h.run(path, "task", "reopen", "T-07"); code != 0 {
		t.Fatalf("task reopen: %d %s", code, h.errb.String())
	}
	if j := joRpReopenJob(t, store); j == nil || j.ID != "J-4" {
		t.Fatalf("the reopen Job is not the open J-4: %+v", j)
	}
	if ids := joRpIDs(t, store); ids["J-3"] != 1 || ids["J-4"] != 1 {
		t.Fatalf("job records by id %v, want J-3 once (the planted one) and J-4 once", ids)
	}
}

// A `job` post between the reopen's first read and its post takes the id the first
// read picked. The post checks the id under .orchestrator.lock and picks again.
func TestJoRpStalePick(t *testing.T) {
	h, path, store := joRpClosed(t)
	post, err := reopenPlanJob(h.env, path)
	if err != nil || post == nil {
		t.Fatalf("reopenPlanJob: closure %v, error %v", post != nil, err)
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", nil)); code != 0 {
		t.Fatalf("the member's J-3: %d %s", code, h.errb.String())
	}
	if err := post(); err != nil {
		t.Fatalf("the closure: %v", err)
	}
	if ids := joRpIDs(t, store); ids["J-3"] != 1 || ids["J-4"] != 1 {
		t.Fatalf("job records by id %v, want J-3 once (the member's) and J-4 once", ids)
	}
	if j := joRpReopenJob(t, store); j == nil || j.ID != "J-4" {
		t.Fatalf("the reopen Job is not the open J-4: %+v", j)
	}
}
