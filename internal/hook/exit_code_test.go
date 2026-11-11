package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// jobWithFeed makes the fixture a Job's session on a task that has a Feed and
// no result yet, and returns the path of the session's activity file.
func (f *eventFixture) jobWithFeed(t *testing.T) string {
	t.Helper()
	f.workerSession(t)
	f.env["GRID_TASK"] = f.dir
	if err := os.MkdirAll(filepath.Join(f.dir, "feed"), 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(f.dir, "sessions", ".hooks", "session-demo.json")
}

func readActivity(t *testing.T, path string) session.Activity {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var a session.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	return a
}

// Row 1: the raw end reason must survive in the session activity, so a normal
// end and an unknown one are told apart.
func TestWorkerExitKeepsTheRawReasonInTheActivityFile(t *testing.T) {
	f := eventSetup(t)
	path := f.jobWithFeed(t)
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "prompt_input_exit"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "prompt_input_exit") {
		t.Fatalf("activity file has no prompt_input_exit: %s", raw)
	}
}

// Row 2: reason and code sit under the same key.
func TestWorkerExitStoresCodeUnderTheReasonsKey(t *testing.T) {
	f := eventSetup(t)
	path := f.jobWithFeed(t)
	f.env["GRID_LEASE_EPOCH"] = "3"
	f.env["GRID_ATTEMPT"] = "1"
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "prompt_input_exit"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	a := readActivity(t, path)
	if a.ExitReason != "done" || a.ExitCode != "prompt_input_exit" {
		t.Fatalf("reason/code %q/%q", a.ExitReason, a.ExitCode)
	}
	if len(a.Exits) != 1 || len(a.ExitCodes) != 1 {
		t.Fatalf("exits %v codes %v", a.Exits, a.ExitCodes)
	}
	for key, reason := range a.Exits {
		if reason != "done" || a.ExitCodes[key] != "prompt_input_exit" {
			t.Fatalf("key %q: reason %q code %q", key, reason, a.ExitCodes[key])
		}
	}
}

// Row 3: every way a session ends, with the mapped reason and the raw code.
func TestWorkerExitReasonAndCodeTable(t *testing.T) {
	for _, tc := range []struct {
		name, event, reason, wantReason, wantCode string
	}{
		{"clear", "SessionEnd", "clear", "cancelled", "clear"},
		{"logout", "SessionEnd", "logout", "cancelled", "logout"},
		{"other", "SessionEnd", "other", "done", "other"},
		{"empty", "SessionEnd", "", "done", "none"},
		{"crash", "SessionEnd", "crash", "crash", "crash"},
		{"usage_limit", "SessionEnd", "usage_limit", "usage_limit", "usage_limit"},
		{"subagent stop", "SubagentStop", "", "done", "subagent_stop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := eventSetup(t)
			path := f.jobWithFeed(t)
			// A SubagentStop reaches workerExit through the worker-exit hook entry:
			// the combined SubagentStop entry runs the result-before-stop reminder
			// first, which exits 2 for a Job with no result and returns before it.
			event := tc.event
			if event == "SubagentStop" {
				event = "worker-exit"
			}
			out, log, code := f.run(t, event, hookio.Input{HookEventName: tc.event, Reason: tc.reason, AgentID: "agent-1"})
			if code != 0 || log != "" || out != "" {
				t.Fatalf("hook must stay a silent record: exit %d out %q log %q", code, out, log)
			}
			a := readActivity(t, path)
			if a.ExitReason != tc.wantReason || a.ExitCode != tc.wantCode {
				t.Fatalf("got %q/%q, want %q/%q", a.ExitReason, a.ExitCode, tc.wantReason, tc.wantCode)
			}
			if len(a.Exits) != 1 || len(a.ExitCodes) != 1 {
				t.Fatalf("exits %v codes %v", a.Exits, a.ExitCodes)
			}
			for key, reason := range a.Exits {
				if reason != tc.wantReason || a.ExitCodes[key] != tc.wantCode {
					t.Fatalf("key %q: %q/%q", key, reason, a.ExitCodes[key])
				}
			}
		})
	}
}

// Row 4: the first end for a key wins, reason and code alike.
func TestWorkerExitRepeatChangesNeitherReasonNorCode(t *testing.T) {
	f := eventSetup(t)
	path := f.jobWithFeed(t)
	for _, reason := range []string{"prompt_input_exit", "crash"} {
		if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: reason}); code != 0 || log != "" {
			t.Fatalf("end: %d %s", code, log)
		}
	}
	a := readActivity(t, path)
	if a.ExitReason != "done" || a.ExitCode != "prompt_input_exit" {
		t.Fatalf("second end changed the record: %q/%q", a.ExitReason, a.ExitCode)
	}
	if len(a.Exits) != 1 || len(a.ExitCodes) != 1 {
		t.Fatalf("exits %v codes %v", a.Exits, a.ExitCodes)
	}
	for key := range a.Exits {
		if a.Exits[key] != "done" || a.ExitCodes[key] != "prompt_input_exit" {
			t.Fatalf("key %q: %q/%q", key, a.Exits[key], a.ExitCodes[key])
		}
	}
}

// Row 4: a Job that already has a result writes nothing, code included.
func TestWorkerExitWritesNoCodeForAJobWithAResult(t *testing.T) {
	f := eventSetup(t)
	path := f.jobWithFeed(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "result", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Task, Worker string
		Fence        int
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	id, err := feed.TaskID(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(fx.Task), []byte(id))
	var one bytes.Buffer
	if err := json.Compact(&one, raw); err != nil {
		t.Fatal(err)
	}
	one.WriteByte('\n')
	if err := os.WriteFile(filepath.Join(f.dir, "feed", fmt.Sprintf("%s.%d.jsonl", fx.Worker, fx.Fence)), one.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "prompt_input_exit"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	if got, err := os.ReadFile(path); err == nil {
		t.Fatalf("a Job with a result got an activity record: %s", got)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// mainActivity is the Activity shape on main, before exit_code and exit_codes.
type mainActivity struct {
	Exits      map[string]string `json:"exits,omitempty"`
	SessionID  string            `json:"session_id"`
	Seen       map[string]bool   `json:"seen,omitempty"`
	ExitReason string            `json:"exit_reason,omitempty"`
}

// Row 5: old files load, and new files load in a reader that ignores the new fields.
func TestExitCodeFieldsAreCompatibleBothWays(t *testing.T) {
	f := eventSetup(t)
	s := session.Store{TaskDir: f.dir}
	dir := filepath.Join(f.dir, "sessions", ".hooks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"exits":{"exit::J-1::":"done"},"session_id":"old-one","last":"2026-11-11T02:34:36Z","seen":{"exit::J-1::":true},"exit_reason":"done"}`
	if err := os.WriteFile(filepath.Join(dir, "old-one.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := s.Activity("old-one")
	if err != nil || a.ExitReason != "done" || a.Exits["exit::J-1::"] != "done" || a.ExitCode != "" || a.ExitCodes != nil {
		t.Fatalf("old file: %+v %v", a, err)
	}

	path := f.jobWithFeed(t)
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "prompt_input_exit"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m mainActivity
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("main-shaped reader: %v", err)
	}
	if m.ExitReason != "done" || m.SessionID != "session-demo" || len(m.Exits) != 1 {
		t.Fatalf("main-shaped reader: %+v", m)
	}
}
