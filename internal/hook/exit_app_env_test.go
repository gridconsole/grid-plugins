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

// appWorkerSession starts the same live record workerSession does, but marks the
// hook environment the way the app's worker starts a Job session: GRID_TASK,
// GRID_JOB, GRID_WORKER_ID and GRID_ROLE=worker, and never GRID_FEED_TASK or GRID_WORKER.
func (f *eventFixture) appWorkerSession(t *testing.T) (session.Store, session.Key) {
	t.Helper()
	s := session.Store{TaskDir: f.dir}
	key := session.Key{SessionID: "session-demo", MachineID: "machine-demo"}
	if _, err := s.Start(session.Record{Key: key, PID: 123, PIDStart: "start", Lease: "J-1", Holder: "member@example.com", Fence: 2, Attempt: 1}); err != nil {
		t.Fatal(err)
	}
	f.env["GRID_TASK"] = f.dir
	f.env["GRID_JOB"] = "J-1"
	f.env["GRID_WORKER_ID"] = "member@example.com"
	f.env["GRID_ROLE"] = "worker"
	return s, key
}

// Row 1 and 2: the app's names mark a Worker, so a crash ends the record Errored and
// stores the reason under the Job's key, as the old-name fixture of exit_test.go does.
func TestAppEnvWorkerExitEndsTheRecordErrored(t *testing.T) {
	f := eventSetup(t)
	s, key := f.appWorkerSession(t)
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "crash"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	got, err := s.Show(key)
	if err != nil || got.Ended == nil || got.State != session.Errored {
		t.Fatalf("a crash in a Job session with the app's names did not end the record Errored: %+v %v", got, err)
	}
	a, err := s.Activity(key.SessionID)
	if err != nil || a.ExitReason != "crash" || a.Exits["exit::J-1::"] != "crash" {
		t.Fatalf("exit not stored under the Job's key: %+v %v", a, err)
	}
}

// Row 2: a Job that already has a result is left alone, with the app's names.
func TestAppEnvWorkerExitSkipsAJobThatAlreadyHasAResult(t *testing.T) {
	f := eventSetup(t)
	s, key := f.appWorkerSession(t)
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
	if a, err := s.Activity(key.SessionID); err != nil || a.ExitReason != "" || len(a.Exits) != 0 {
		t.Fatalf("exit recorded for a Job with a result: %+v %v", a, err)
	}
}

// Row 3: the marker is (GRID_FEED_TASK or GRID_TASK) and GRID_JOB and (GRID_WORKER or
// GRID_WORKER_ID). Each missing piece, with the app's names, leaves the record alone.
// The first case is a plain session: SessionStart's writeEnv exports GRID_TASK and it
// has no GRID_JOB.
func TestAppEnvWorkerExitNeedsTheWholeMarker(t *testing.T) {
	for _, tc := range []struct {
		name   string
		delete []string
	}{
		{"without GRID_JOB", []string{"GRID_JOB"}},
		{"without both worker ids", []string{"GRID_WORKER_ID", "GRID_WORKER"}},
		{"without both task names", []string{"GRID_TASK", "GRID_FEED_TASK"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := eventSetup(t)
			s, key := f.appWorkerSession(t)
			for _, k := range tc.delete {
				delete(f.env, k)
			}
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

// The old and the app's names mix: each pair is accepted, so a launch that sets one of
// each still marks a Worker.
func TestAppEnvWorkerExitAcceptsMixedNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  map[string]string
		drop []string
	}{
		{"old task name, app worker name", map[string]string{"GRID_FEED_TASK": ""}, []string{"GRID_TASK"}},
		{"app task name, old worker name", map[string]string{"GRID_WORKER": "worker-demo"}, []string{"GRID_WORKER_ID"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := eventSetup(t)
			s, key := f.appWorkerSession(t)
			for k := range tc.set {
				if k == "GRID_FEED_TASK" {
					f.env[k] = f.dir
				} else {
					f.env[k] = tc.set[k]
				}
			}
			for _, k := range tc.drop {
				delete(f.env, k)
			}
			if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "crash"}); code != 0 || log != "" {
				t.Fatalf("end: %d %s", code, log)
			}
			if got, err := s.Show(key); err != nil || got.State != session.Errored {
				t.Fatalf("mixed names did not mark a Worker: %+v %v", got, err)
			}
		})
	}
}
