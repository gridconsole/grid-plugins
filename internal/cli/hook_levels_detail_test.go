package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// grid hook levels --json stays the plain name-to-level map, byte for byte.
func TestHookLevelsWithoutDetailIsUnchanged(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "hook", "levels", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	want := `{"history-log":"warn","home-allow":"warn","host-guard":"warn","job-lint":"block","needs-you":"warn","only-writer":"warn","plan-guard":"warn","session-check":"warn","session-end":"warn","session-start":"warn","stop-check":"block","usage-log":"warn"}` + "\n"
	if h.out.String() != want {
		t.Errorf("hook levels --json:\n got %s\nwant %s", h.out.String(), want)
	}
	h.out.Reset()
	if code := h.run(h.home, "hook", "levels"); code != 0 {
		t.Fatal(code)
	}
	if got := h.out.String(); strings.Contains(got, "[") || !strings.HasPrefix(got, "history-log warn\nhome-allow warn\n") {
		t.Errorf("hook levels text changed: %s", got)
	}
}

// grid hook levels --json --detail answers [{n, level, events, d}] for every rule,
// in the order of grid hook levels, with the level the person set.
func TestHookLevelsDetailShape(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "hook", "level", "plan-guard", "block"); code != 0 {
		t.Fatalf("set: exit %d: %s", code, h.errb)
	}
	h.out.Reset()
	if code := h.run(h.home, "hook", "levels", "--json", "--detail"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("not a JSON array: %v: %s", err, h.out)
	}
	names := settings.HookNames()
	if len(rows) != len(names) {
		t.Fatalf("%d rows, want %d", len(rows), len(names))
	}
	for i, r := range rows {
		if len(r) != 5 || r["n"] != names[i] {
			t.Errorf("row %d = %v, want the keys n, level, gate, events, d for %s", i, r, names[i])
		}
		if _, ok := r["events"].([]any); !ok {
			t.Errorf("%s: events is %v, want an array (empty when no hook event runs the rule)", names[i], r["events"])
		}
		if d, _ := r["d"].(string); d == "" || strings.Contains(d, "\n") {
			t.Errorf("%s: d = %q, want one line", names[i], d)
		}
	}
	byName := map[string]map[string]any{}
	for _, r := range rows {
		byName[r["n"].(string)] = r
	}
	if byName["plan-guard"]["level"] != "block" || byName["stop-check"]["level"] != "block" || byName["usage-log"]["level"] != "warn" {
		t.Errorf("levels wrong: plan-guard %v, stop-check %v, usage-log %v", byName["plan-guard"]["level"], byName["stop-check"]["level"], byName["usage-log"]["level"])
	}
	start := byName["session-start"]["events"].([]any)
	if len(start) != 1 || start[0].(map[string]any)["event"] != "SessionStart" || start[0].(map[string]any)["matcher"] != "startup|resume|clear|compact|fork" {
		t.Errorf("session-start events = %v, want SessionStart with the hooks.json matcher", start)
	}
	stop := byName["stop-check"]["events"].([]any)
	if len(stop) != 2 || stop[0].(map[string]any)["event"] != "Stop" {
		t.Errorf("stop-check events = %v", stop)
	}
	if _, has := stop[0].(map[string]any)["matcher"]; has {
		t.Errorf("an event with no matcher carries a matcher key: %v", stop[0])
	}
}
