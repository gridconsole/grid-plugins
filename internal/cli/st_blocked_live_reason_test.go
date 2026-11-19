package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// Live T-13 J-11: Ready releases a Job whose state.json still holds the wait
// reason an earlier tick wrote, and job blocked listed it with that old reason.
// job blocked shows the live wait from Ready, and nothing for a released Job.
func TestJobBlockedShowsTheLiveReason(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--type", "job", "--body", personBody(t, "J-2", []any{"J-1"})); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb)
	}
	w, err := orchestratorWriter(h.env, "task", os.Getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	const stale = "waiting for all children of J-9"
	for _, id := range []string{"J-1", "J-2"} {
		// The tick writes a wait reason as a control with no status (orchestrator tick.go).
		if _, err := store.Post(feed.Input{Type: "control", Body: map[string]any{"target": id, "reason": stale}}, w); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"J-1", "J-2"} {
		if j := st.Jobs[id]; j == nil || j.Reason != stale || j.Status != "posted" {
			t.Fatalf("%s in state.json: %+v, want posted with the old reason", id, j)
		}
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "blocked", "--json"); code != 0 {
		t.Fatalf("job blocked: %d %s", code, h.errb)
	}
	var rows []orchestrator.Row
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("%v: %s", err, h.out)
	}
	why := map[string]string{}
	for _, r := range rows {
		why[r.ID] = r.Why
	}
	if got, listed := why["J-1"]; listed {
		t.Errorf("J-1 is ready and listed as blocked: %q", got)
	}
	if got := why["J-2"]; got != "waiting for J-1 to be accepted" {
		t.Errorf("J-2 is listed with %q, want the live wait on J-1", got)
	}
}
