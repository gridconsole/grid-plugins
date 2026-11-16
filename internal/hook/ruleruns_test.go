package hook

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/hookevents"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// These tests read the events file by path and by its JSON keys, not through the
// package that writes it, so they pin the file's format as well.
type runLine struct {
	Rule    string `json:"rule"`
	Event   string `json:"event"`
	At      string `json:"at"`
	Outcome string `json:"outcome"`
}

func eventsPath(home string) string { return filepath.Join(home, ".hook-counts", "events.jsonl") }

func readRunLines(t *testing.T, path string) []runLine {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []runLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l runLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatalf("events line %q: %v", sc.Text(), err)
		}
		out = append(out, l)
	}
	return out
}

func rulesOf(lines []runLine) map[string]int {
	out := map[string]int{}
	for _, l := range lines {
		out[l.Rule]++
	}
	return out
}

// A hook run leaves one line per rule it consulted, with the rule, the event, a
// UTC RFC 3339 time and an outcome; a rule asked at several places in the run
// (history-log is asked at four) leaves one.
func TestRuleRunsOneLinePerRulePerRun(t *testing.T) {
	f := eventSetup(t)
	if _, _, code := f.run(t, "SessionStart", hookio.Input{Source: "startup"}); code != 0 {
		t.Fatalf("SessionStart exit %d", code)
	}
	lines := readRunLines(t, eventsPath(f.home))
	got := rulesOf(lines)
	if got["session-start"] != 1 || got["usage-log"] != 1 || len(lines) != 2 {
		t.Fatalf("SessionStart wrote %v, want session-start and usage-log once each", lines)
	}
	for _, l := range lines {
		if l.Event != "SessionStart" || l.Outcome != "ok" {
			t.Errorf("line %+v, want event SessionStart and outcome ok", l)
		}
		at, err := time.Parse(time.RFC3339, l.At)
		if err != nil || !strings.HasSuffix(l.At, "Z") || time.Since(at) > time.Minute || time.Since(at) < -time.Minute {
			t.Errorf("at %q is not a recent UTC RFC 3339 time (%v)", l.At, err)
		}
	}

	before := len(lines)
	if _, _, code := f.run(t, "posttooluse", hookio.Input{ToolName: "Read"}); code != 0 {
		t.Fatalf("PostToolUse exit %d", code)
	}
	after := readRunLines(t, eventsPath(f.home))
	added := after[before:]
	if len(added) != 1 || added[0].Rule != "history-log" || added[0].Event != "PostToolUse" || added[0].Outcome != "ok" {
		t.Fatalf("PostToolUse added %+v, want one history-log line for PostToolUse, ok", added)
	}
}

// The outcome follows the rule's level and what the rule did.
func TestRuleRunsOutcomes(t *testing.T) {
	last := func(f *eventFixture, rule string) runLine {
		lines := readRunLines(t, eventsPath(f.home))
		for i := len(lines) - 1; i >= 0; i-- {
			if lines[i].Rule == rule {
				return lines[i]
			}
		}
		t.Fatalf("no line for %s in %v", rule, lines)
		return runLine{}
	}
	f := eventSetup(t)
	if err := (settings.Store{Home: f.home}).SetHook("session-start", "off"); err != nil {
		t.Fatal(err)
	}
	f.run(t, "SessionStart", hookio.Input{Source: "startup"})
	if l := last(f, "session-start"); l.Outcome != "skipped" {
		t.Errorf("session-start at off: outcome %q, want skipped", l.Outcome)
	}

	// A Job's agent that stops without a result: stop-check refuses at block, warns at warn.
	for _, tc := range []struct {
		level, outcome string
		code           int
	}{{"block", "blocked", 2}, {"warn", "warned", 0}, {"off", "skipped", 0}} {
		f := eventSetup(t)
		f.env["GRID_JOB"] = "J1"
		if err := (settings.Store{Home: f.home}).SetHook("stop-check", tc.level); err != nil {
			t.Fatal(err)
		}
		_, _, code := f.run(t, "Stop", hookio.Input{})
		if code != tc.code {
			t.Fatalf("stop-check %s: exit %d, want %d", tc.level, code, tc.code)
		}
		if l := last(f, "stop-check"); l.Outcome != tc.outcome || l.Event != "Stop" {
			t.Errorf("stop-check %s: line %+v, want outcome %s on Stop", tc.level, l, tc.outcome)
		}
	}
}

// A write that fails changes nothing the hook says or returns. A regular file
// stands where the .hook-counts folder would be, so no line can be written.
func TestRuleRunsFailedWriteChangesNothing(t *testing.T) {
	f := eventSetup(t)
	blocker := filepath.Join(f.home, ".hook-counts")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	out1, err1, code1 := f.run(t, "SessionStart", hookio.Input{Source: "startup"})
	if info, err := os.Lstat(blocker); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("the blocking file changed: %v %v", info, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	out2, err2, code2 := f.run(t, "SessionStart", hookio.Input{Source: "startup"})
	if len(readRunLines(t, eventsPath(f.home))) == 0 {
		t.Fatal("the run without the obstacle wrote no line; the comparison proves nothing")
	}
	if code1 != code2 || out1 != out2 || err1 != err2 || out1 == "" {
		t.Errorf("a failed write changed the hook:\nexit %d vs %d\nstdout %q vs %q\nstderr %q vs %q", code1, code2, out1, out2, err1, err2)
	}
}

// With no Grid home the hook writes nothing and does not make one.
func TestRuleRunsNoGridHomeWritesNothing(t *testing.T) {
	f := eventSetup(t)
	f.o.Resolver.GridHome = ""
	f.run(t, "posttooluse", hookio.Input{ToolName: "Read"})
	if _, err := os.Stat(filepath.Join(f.home, ".hook-counts")); err == nil {
		t.Error(".hook-counts exists")
	}
}

// The bound: a log at the limit is shifted to events.jsonl.1 and so on, and no more
// than the active file and two older copies stay. The oldest copy is dropped.
func TestRuleRunsLogIsBounded(t *testing.T) {
	const limit = hookevents.Limit
	f := eventSetup(t)
	dir := filepath.Join(f.home, ".hook-counts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	seed := func(name, marker string, size int) {
		line := []byte(`{"rule":"history-log","event":"` + marker + `","at":"2026-01-01T00:00:00Z","outcome":"ok"}` + "\n")
		var b []byte
		for len(b) < size {
			b = append(b, line...)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	seed("events.jsonl", "gen0", limit)
	seed("events.jsonl.1", "gen1", 1000)
	seed("events.jsonl.2", "gen2", 1000)
	f.run(t, "posttooluse", hookio.Input{ToolName: "Read"})

	first := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		line, _, _ := strings.Cut(string(b), "\n")
		return line
	}
	if !strings.Contains(first("events.jsonl.1"), "gen0") || !strings.Contains(first("events.jsonl.2"), "gen1") {
		t.Errorf("copies not shifted: .1 %q, .2 %q", first("events.jsonl.1"), first("events.jsonl.2"))
	}
	if _, err := os.Stat(filepath.Join(dir, "events.jsonl.3")); err == nil {
		t.Error("events.jsonl.3 exists: more than three files stay")
	}
	if strings.Contains(first("events.jsonl.2"), "gen2") {
		t.Error("the oldest copy was not dropped")
	}
	if info, err := os.Stat(filepath.Join(dir, "events.jsonl")); err != nil || info.Size() >= 1024 {
		t.Errorf("the active log after the shift: %v %v, want a fresh small file", info, err)
	}
	lines := readRunLines(t, filepath.Join(dir, "events.jsonl"))
	if len(lines) != 1 || lines[0].Rule != "history-log" || lines[0].Event != "PostToolUse" {
		t.Errorf("the fresh log holds %d lines, want the one new PostToolUse line", len(lines))
	}
}
