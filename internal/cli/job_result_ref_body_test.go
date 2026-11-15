package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A result whose body is only {"ref":"bodies/..."} names a stored body the
// agent cannot have written; the read would drop it in silence, so grid job
// result refuses it with exit 3 and writes nothing.
func TestJobResultRefusesBareRefBody(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
	if code := h.run(path, "job", "ack", "J-1", "--attempt", "1", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	before, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	h.errb.Reset()
	if code := h.run(path, "job", "result", "J-1", "--attempt", "1", "--body", `{"ref":"bodies/`+feedTaskID+`.json"}`); code != 3 {
		t.Fatalf("bare ref body: exit %d, want 3: %s", code, h.errb)
	}
	if !strings.Contains(h.errb.String(), "must be inline") {
		t.Fatalf("message does not say the body must be inline: %s", h.errb)
	}
	after, err := store.Read(feed.Filter{})
	if err != nil || len(after.Records) != len(before.Records) || len(after.Bad) != len(before.Bad) {
		t.Fatalf("records %d -> %d, bad %v -> %v, %v", len(before.Records), len(after.Records), before.Bad, after.Bad, err)
	}
}
