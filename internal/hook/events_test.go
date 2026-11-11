package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

type eventFixture struct {
	home, dir string
	o         Options
	env       map[string]string
}

func eventSetup(t *testing.T) *eventFixture {
	t.Helper()
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "grid")
	r := where.Resolver{GridHome: home, Home: root, Ceiling: root, Machine: "machine-demo"}
	made, err := (task.Store{Resolver: r}).New(task.NewOptions{Cwd: root, Words: "Cache configuration", Project: where.Personal})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(made.Path, "history.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	return &eventFixture{home: home, dir: made.Path, env: env, o: Options{Resolver: r, Cwd: made.Path, Getenv: func(k string) string { return env[k] }}}
}
func (f *eventFixture) run(t *testing.T, event string, in hookio.Input) (string, string, int) {
	t.Helper()
	if in.Cwd == "" {
		in.Cwd = f.dir
	}
	if in.SessionID == "" {
		in.SessionID = "session-demo"
	}
	if in.HookEventName == "" {
		in.HookEventName = event
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	o := f.o
	o.Event = event
	o.Stdin = bytes.NewReader(b)
	o.Stdout = &out
	o.Stderr = &log
	code := Run(o)
	return out.String(), log.String(), code
}
func TestHookHeartbeatCannotRenewOrEndOnStop(t *testing.T) {
	f := eventSetup(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := session.Store{TaskDir: f.dir, Now: func() time.Time { return now }}
	key := session.Key{SessionID: "session-demo", MachineID: "machine-demo"}
	r, err := s.Start(session.Record{Key: key, PID: 123, PIDStart: "start", Lease: "J-1", Holder: "member@example.com", Fence: 2, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Reconcile(key, session.Observation{Process: session.Alive, PID: 123, PIDStart: "start"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.ReadHeartbeat(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"PostToolUse", "Stop"} {
		_, log, code := f.run(t, event, hookio.Input{ToolName: "Read"})
		if code != 0 || log != "" {
			t.Fatalf("%s: %d %s", event, code, log)
		}
	}
	after, err := s.ReadHeartbeat(r)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("hook renewed supervision heartbeat")
	}
	got, err := s.Show(key)
	if err != nil || got.Ended != nil || got.Last.Equal(r.Last) {
		t.Fatalf("activity not separate from liveness: %+v %v", got, err)
	}
	f.env["GRID_FEED_TASK"], f.env["GRID_JOB"], f.env["GRID_WORKER"] = f.dir, "J-1", "worker-demo"
	_, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "cancelled"})
	if code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	got, err = s.Show(key)
	if err != nil || got.State != session.StoppedByYou {
		t.Fatalf("end state: %+v %v", got, err)
	}
	if _, err = s.ReadHeartbeat(r); !os.IsNotExist(err) {
		t.Fatalf("end retained live file: %v", err)
	}
}
func TestUsageHooksKeepNumbersAndFoldRetries(t *testing.T) {
	f := eventSetup(t)
	in := hookio.Input{ToolName: "Agent", ToolUseID: "tool-1", ToolResponse: map[string]any{"result": "PRIVATE RESULT", "usage": map[string]any{"input_tokens": 12, "output_tokens": 4}, "totalTokens": 16, "totalDurationMs": 20}}
	for i := 0; i < 2; i++ {
		_, log, code := f.run(t, "PostToolUse", in)
		if code != 0 || log != "" {
			t.Fatalf("usage: %d %s", code, log)
		}
	}
	s := session.Store{TaskDir: f.dir}
	rows, err := s.Activities()
	if err != nil {
		t.Fatal(err)
	}
	sum := session.FoldUsage(rows)
	if sum.Input != 12 || sum.Output != 4 || sum.Total != 16 {
		t.Fatalf("totals: %+v", sum)
	}
	b, _ := json.Marshal(rows)
	if bytes.Contains(b, []byte("PRIVATE")) {
		t.Fatal("copied result text")
	}
	if _, err := os.Stat(filepath.Join(f.dir, "feed")); !os.IsNotExist(err) {
		t.Fatal("usage posted to Feed")
	}
}
func TestRecallPointersReadDedupeAndPlanChange(t *testing.T) {
	f := eventSetup(t)
	s := brain.Store{GridHome: f.home}
	n := brain.Note{Name: "cache", Topic: "decisions", Type: "convention", Description: "Use when changing cache configuration.", Summary: "Keep cache settings explicit.", Source: "task:T-01", Level: "home", Body: "PRIVATE BODY that must never be dumped.", Origin: brain.Origin{Kind: "session", ID: "session-demo"}}
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	e, err := s.Accept("home", "cache")
	if err != nil {
		t.Fatal(err)
	}
	read := hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, e.Path)}}
	for i := 0; i < 2; i++ {
		_, log, code := f.run(t, "PostToolUse", read)
		if code != 0 || log != "" {
			t.Fatalf("read: %d %s", code, log)
		}
	}
	b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "brain read") != 1 {
		t.Fatalf("read history: %s", b)
	}
}

// TestStopGuardBlocksOnceAndMarksOwnedBrief was deleted: the brief Outcome and
// Verdict check, and the brief marking, were removed by the owner's direction
// (verdicts-11nov v-5, one stop check). TestStopJobWithoutResultBlocksOnce in
// stop_test.go still covers the remaining check, block once then stop.
func TestInternalErrorsAllowAndGuardStillDenies(t *testing.T) {
	f := eventSetup(t)
	sessions := filepath.Join(f.dir, "sessions")
	if err := os.MkdirAll(sessions, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessions, ".hooks"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	f.env["GRID_FEED_TASK"], f.env["GRID_JOB"], f.env["GRID_WORKER"] = f.dir, "J-1", "worker-demo"
	out, log, code := f.run(t, "SessionEnd", hookio.Input{})
	if code != 0 || out != "" || !strings.Contains(log, "grid hook:") {
		t.Fatalf("error was not logged and allowed: %q %q %d", out, log, code)
	}
	// host-guard warns by default; set to block it keeps the deny this test covers.
	if err := os.WriteFile(filepath.Join(f.home, "settings.json"), []byte(`{"hooks":{"host-guard":"block"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, code = f.run(t, "PermissionRequest", hookio.Input{ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(f.dir, "task.md"), "content": "bad"}})
	if code != 0 || !strings.Contains(out, `"behavior":"deny"`) {
		t.Fatalf("guard lost priority: %s %d", out, code)
	}
}
func TestHistoryBashFailureUsesRecordedContent(t *testing.T) {
	f := eventSetup(t)
	in := hookio.Input{ToolName: "Bash", ToolUseID: "tool-1", ToolInput: map[string]any{"command": "grid task change"}}
	_, log, code := f.run(t, "PreToolUse", in)
	if code != 0 || log != "" {
		t.Fatalf("before: %d %s", code, log)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "notes.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, log, code = f.run(t, "PostToolUseFailure", in)
	if code != 0 || log != "" {
		t.Fatalf("after: %d %s", code, log)
	}
	b, _ := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if !strings.Contains(string(b), "file written · notes.md · by claude · via Bash") {
		t.Fatalf("history: %s", b)
	}
}
