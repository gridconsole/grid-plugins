package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// workerSession starts a live record for the fixture's session and marks the
// hook environment as a Grid Worker's.
func (f *eventFixture) workerSession(t *testing.T) (session.Store, session.Key) {
	t.Helper()
	s := session.Store{TaskDir: f.dir}
	key := session.Key{SessionID: "session-demo", MachineID: "machine-demo"}
	if _, err := s.Start(session.Record{Key: key, PID: 123, PIDStart: "start", Lease: "J-1", Holder: "member@example.com", Fence: 2, Attempt: 1}); err != nil {
		t.Fatal(err)
	}
	f.env["GRID_FEED_TASK"] = f.dir
	f.env["GRID_JOB"] = "J-1"
	f.env["GRID_WORKER"] = "worker-demo"
	return s, key
}

func TestWorkerExitMapsEverySessionEndReason(t *testing.T) {
	for _, tc := range []struct {
		reason string
		state  session.State
		exit   string
	}{
		// Claude Code's own SessionEnd reasons: none of them is a crash.
		{"prompt_input_exit", session.Finished, "done"},
		{"other", session.Finished, "done"},
		{"", session.Finished, "done"},
		{"clear", session.StoppedByYou, "cancelled"},
		{"logout", session.StoppedByYou, "cancelled"},
		// Grid's own reasons.
		{"done", session.Finished, "done"},
		{"usage_limit", session.Finished, "usage_limit"},
		{"cancelled", session.StoppedByYou, "cancelled"},
		{"crash", session.Errored, "crash"},
		{"loop", session.Errored, "loop"},
	} {
		t.Run("reason="+tc.reason, func(t *testing.T) {
			f := eventSetup(t)
			s, key := f.workerSession(t)
			_, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: tc.reason})
			if code != 0 || log != "" {
				t.Fatalf("end: %d %s", code, log)
			}
			got, err := s.Show(key)
			if err != nil || got.State != tc.state {
				t.Fatalf("state %q, want %q: %+v %v", got.State, tc.state, got, err)
			}
			a, err := s.Activity(key.SessionID)
			if err != nil || a.ExitReason != tc.exit {
				t.Fatalf("exit reason %q, want %q: %v", a.ExitReason, tc.exit, err)
			}
		})
	}
}

func TestWorkerExitNeedsTheWholeWorkerEnvironment(t *testing.T) {
	for _, missing := range []string{"GRID_FEED_TASK", "GRID_JOB", "GRID_WORKER"} {
		t.Run("without "+missing, func(t *testing.T) {
			f := eventSetup(t)
			s, key := f.workerSession(t)
			delete(f.env, missing)
			// A crash signal still means nothing for a session that is not a Worker's.
			if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "crash"}); code != 0 || log != "" {
				t.Fatalf("end: %d %s", code, log)
			}
			got, err := s.Show(key)
			if err != nil || got.Ended != nil || got.State == session.Errored {
				t.Fatalf("ordinary session was ended: %+v %v", got, err)
			}
			if a, err := s.Activity(key.SessionID); err != nil || a.ExitReason != "" || len(a.Exits) != 0 {
				t.Fatalf("exit recorded for an ordinary session: %+v %v", a, err)
			}
		})
	}
}

func TestWorkerExitSkipsAJobThatAlreadyHasAResult(t *testing.T) {
	f := eventSetup(t)
	s, key := f.workerSession(t)
	// The contract's own result fixture, filed under the file name its writer and fence give.
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
	feedDir := filepath.Join(f.dir, "feed")
	if err := os.MkdirAll(feedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var one bytes.Buffer
	if err := json.Compact(&one, raw); err != nil {
		t.Fatal(err)
	}
	one.WriteByte('\n')
	if err := os.WriteFile(filepath.Join(feedDir, fmt.Sprintf("%s.%d.jsonl", fx.Worker, fx.Fence)), one.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "crash"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	got, err := s.Show(key)
	if err != nil || got.Ended != nil {
		t.Fatalf("a Job with a result was ended again: %+v %v", got, err)
	}
}
