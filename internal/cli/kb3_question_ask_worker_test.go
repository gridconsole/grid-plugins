package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A worker-run agent holds its own Job in GRID_LEASE. ask reaches the Feed with
// a person gate, and the default workflow does not list worker for person gates.
func TestKb3QAskByWorkerIsRefusedNoJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", "")
	args := []string{"question", "ask", "Which export format?", "--who", "dana", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json"}
	if code := h.run(path, args...); code == 0 || !strings.Contains(h.errb.String(), "role worker may not post person gates") {
		t.Fatalf("ask %d: %s", code, h.errb)
	}
	jobs, _ := store.Read(feed.Filter{Type: "job"})
	if len(jobs.Records) != 1 {
		t.Fatalf("want only the harness Job, got %d", len(jobs.Records))
	}
}
