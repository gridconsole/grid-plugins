package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeEventsLog writes the lines of the events file by hand, so the read is
// tested against the file's format and not against the code that writes it.
func writeEventsLog(t *testing.T, h *harness, name string, lines ...string) {
	t.Helper()
	dir := filepath.Join(h.gridHome, ".hook-counts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

const (
	evOld  = `{"rule":"history-log","event":"PostToolUse","at":"2026-11-15T09:00:00Z","outcome":"ok"}`
	evNew  = `{"rule":"history-log","event":"Stop","at":"2026-11-15T10:00:00Z","outcome":"warned"}`
	evSess = `{"rule":"session-start","event":"SessionStart","at":"2026-11-15T09:30:00Z","outcome":"ok"}`
)

// grid hook events --json answers one row for every rule grid hook levels --json
// lists; a rule with a run has its newest event, time and outcome, a rule with no
// run has null fields.
func TestHookEventsJSONShape(t *testing.T) {
	h := newHarness(t)
	writeEventsLog(t, h, "events.jsonl", evOld, evSess, evNew)

	if code := h.run(h.home, "hook", "levels", "--json"); code != 0 {
		t.Fatalf("hook levels: exit %d: %s", code, h.errb)
	}
	var levels map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &levels); err != nil {
		t.Fatal(err)
	}
	var names []string
	for n := range levels {
		names = append(names, n)
	}
	sort.Strings(names)

	if code := h.run(h.home, "hook", "events", "--json"); code != 0 {
		t.Fatalf("hook events --json: exit %d: %s %s", code, h.out, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("not a JSON array: %v: %s", err, h.out)
	}
	var got []string
	byRule := map[string]map[string]any{}
	for _, r := range rows {
		if len(r) != 4 {
			t.Errorf("row %v has %d keys, want rule, lastEvent, at, outcome", r, len(r))
		}
		name, _ := r["rule"].(string)
		got = append(got, name)
		byRule[name] = r
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Errorf("rules %v, want the rules hook levels lists: %v", got, names)
	}
	if r := byRule["history-log"]; r["lastEvent"] != "Stop" || r["at"] != "2026-11-15T10:00:00Z" || r["outcome"] != "warned" {
		t.Errorf("history-log row %v, want its newest line (Stop, 10:00, warned)", r)
	}
	if r := byRule["session-start"]; r["lastEvent"] != "SessionStart" || r["outcome"] != "ok" {
		t.Errorf("session-start row %v", r)
	}
	for _, name := range []string{"stop-check", "host-guard"} {
		r, ok := byRule[name]
		if !ok || r["lastEvent"] != nil || r["at"] != nil || r["outcome"] != nil {
			t.Errorf("%s row %v, want null lastEvent, at and outcome", name, r)
		}
	}
	for _, key := range []string{`"lastEvent":null`, `"at":null`, `"outcome":null`} {
		if !strings.Contains(h.out.String(), key) {
			t.Errorf("raw output lacks %s", key)
		}
	}
}

// With no log at all every rule has null fields, and --all is an empty array.
func TestHookEventsNoLog(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "hook", "events", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil || len(rows) == 0 {
		t.Fatalf("rows %v: %v", rows, err)
	}
	for _, r := range rows {
		if r["lastEvent"] != nil || r["at"] != nil || r["outcome"] != nil {
			t.Errorf("row %v has a value with no log", r)
		}
	}
	if code := h.run(h.home, "hook", "events", "--all", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != "[]" {
		t.Errorf("--all with no log: exit %d, output %q, want []", code, h.out)
	}
}

// --all is the whole log, newest first, across the shifted copies, and a line that
// is not JSON is skipped.
func TestHookEventsAllNewestFirst(t *testing.T) {
	h := newHarness(t)
	writeEventsLog(t, h, "events.jsonl.1", evOld)
	writeEventsLog(t, h, "events.jsonl", evSess, "not json", evNew)
	if code := h.run(h.home, "hook", "events", "--all", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var rows []map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	var at []string
	for _, r := range rows {
		at = append(at, r["at"])
		if r["rule"] == "" || r["event"] == "" || r["outcome"] == "" || len(r) != 4 {
			t.Errorf("row %v, want rule, event, at, outcome", r)
		}
	}
	want := "2026-11-15T10:00:00Z,2026-11-15T09:30:00Z,2026-11-15T09:00:00Z"
	if strings.Join(at, ",") != want {
		t.Errorf("order %v, want newest first %s", at, want)
	}
	if code := h.run(h.home, "hook", "events", "--all"); code != 0 || !strings.HasPrefix(h.out.String(), "2026-11-15T10:00:00Z history-log Stop warned\n") {
		t.Errorf("plain --all: exit %d, %q", code, h.out)
	}
}

func TestHookEventsRefusesOperands(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "hook", "events", "extra"); code != 2 {
		t.Errorf("an operand: exit %d, want 2", code)
	}
	if code := h.run(h.home, "hook", "events", "--nope"); code != 2 {
		t.Errorf("an unknown flag: exit %d, want 2", code)
	}
}
