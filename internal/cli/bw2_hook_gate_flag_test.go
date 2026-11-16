package cli

import (
	"encoding/json"
	"testing"
)

// bw2Rows runs grid hook levels --json --detail and returns the rows by rule name.
func bw2Rows(t *testing.T, h *harness) map[string]map[string]any {
	t.Helper()
	h.out.Reset()
	if code := h.run(h.home, "hook", "levels", "--json", "--detail"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("not a JSON array: %v: %s", err, h.out)
	}
	by := map[string]map[string]any{}
	for _, r := range rows {
		by[r["n"].(string)] = r
	}
	return by
}

func TestBw2DetailMarksTheSixGates(t *testing.T) {
	h := newHarness(t)
	by := bw2Rows(t, h)
	gates := map[string]bool{"home-allow": true, "host-guard": true, "job-lint": true, "only-writer": true, "plan-guard": true, "stop-check": true}
	if len(by) != 12 {
		t.Fatalf("%d rows, want 12", len(by))
	}
	for name, r := range by {
		g, ok := r["gate"].(bool)
		if !ok {
			t.Errorf("%s: gate = %v, want a bool", name, r["gate"])
			continue
		}
		if g != gates[name] {
			t.Errorf("%s: gate = %v, want %v", name, g, gates[name])
		}
	}
}

func TestBw2GateFlagDoesNotFollowTheLevel(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "hook", "level", "plan-guard", "off"); code != 0 {
		t.Fatalf("set: exit %d: %s", code, h.errb)
	}
	r := bw2Rows(t, h)["plan-guard"]
	if r["level"] != "off" || r["gate"] != true {
		t.Errorf("plan-guard = %v, want level off and gate true", r)
	}
}
