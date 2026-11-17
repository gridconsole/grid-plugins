package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// b5FjHeld is a home a FirstJob body already holds; the stream keeps it.
const b5FjHeld = "01BX5ZZKBKACTAV9WEVGEMMVS0"

// b5FjPosted runs one pass with first as FirstJob and answers the body of the only J-1 on the Feed.
func b5FjPosted(t *testing.T, first func(string) map[string]any) map[string]any {
	t.Helper()
	h := newStreamHome(t, 1)
	s := h.stream(4242, nil)
	s.FirstJob = first
	h.pass(s)
	if ids := wlFirstJobIDs(t, h, 0); !wlFirstJobEqual(ids, "J-1") {
		t.Fatalf("the Feed holds job records %v, want exactly J-1", ids)
	}
	jobs := pgClosedOfType(t, h, 0, "job")
	data, err := json.Marshal(jobs[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("posted J-1 is not a valid job-record: %v", err)
	}
	return jobs[0].Body
}

// The J-1 the stream posts carries the stream's machine as home, as grid task new's J-1 does.
func TestB5FjStreamSetsHome(t *testing.T) {
	var asked []string
	body := b5FjPosted(t, wlFirstJobFunc(t, &asked))
	if got, _ := body["home"].(string); got != machine {
		t.Fatalf("J-1 home = %v, want %q (the stream's Machine)", body["home"], machine)
	}
}

// A FirstJob body that already holds home keeps it.
func TestB5FjStreamKeepsHome(t *testing.T) {
	body := b5FjPosted(t, func(string) map[string]any {
		b := jobBody(t, "J-1")
		b["home"] = b5FjHeld
		return b
	})
	if got, _ := body["home"].(string); got != b5FjHeld {
		t.Fatalf("J-1 home = %v, want the FirstJob's own %q", body["home"], b5FjHeld)
	}
}
