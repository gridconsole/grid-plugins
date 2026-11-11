package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// iuRkVerdict is the verdict the Feed reader gives the record id, "" when the
// Feed has no such record.
func iuRkVerdict(t *testing.T, store feed.Store, id string) string {
	t.Helper()
	r, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.ID == id {
			return e.Verdict
		}
	}
	return ""
}

// A job record the fold holds (its after target is on no record) is in neither
// State nor Items, but its id is taken: the curator Job gets the next one.
func TestIuRkKeeperSkipsHeldID(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	// J-1 is posted after a ULID that no record carries, so the fold holds it.
	missing, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	w, err := orchestratorWriter(h.env, "task", h.env.getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	body := curatorJobBody("J-1", id, []string{"first-fact"})
	body["role"] = "builder"
	held, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: body, After: []string{missing}}, w)
	if err != nil {
		t.Fatal(err)
	}
	if snap, err := orchestrator.Load(store); err != nil {
		t.Fatal(err)
	} else if v := iuRkVerdict(t, store, held.ID); snap.State.Jobs["J-1"] != nil || v != "held" {
		t.Fatalf("the J-1 record is not held: job %v, verdict %q", snap.State.Jobs["J-1"], v)
	}

	if code := h.deliverRun(dir, "--outcome", "x", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}

	jobs, snap := curatorJobs(t, store)
	if len(jobs) != 1 || jobs[0].ID != "J-2" {
		t.Fatalf("want one curator Job J-2, got %+v (stdout %q)", jobs, h.out.String())
	}
	if !strings.Contains(h.out.String(), "(Job J-2)") {
		t.Fatalf("count line does not name J-2: %q", h.out.String())
	}
	if snap.State.Jobs["J-1"] != nil || snap.State.Jobs["J-2"] == nil {
		t.Fatalf("State holds J-1 = %v, J-2 = %v", snap.State.Jobs["J-1"], snap.State.Jobs["J-2"])
	}
	if v := iuRkVerdict(t, store, held.ID); v != "held" {
		t.Fatalf("verdict of the J-1 record = %q, want held", v)
	}
	r, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var j1 []string
	for _, e := range r.Records {
		if e.Type == "job" && e.Body["id"] == "J-1" {
			j1 = append(j1, e.ID)
		}
	}
	if len(j1) != 1 || j1[0] != held.ID {
		t.Fatalf("J-1 records = %v, want only the held %s", j1, held.ID)
	}
}
