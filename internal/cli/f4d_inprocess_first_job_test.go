package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// task new run in process, from a host that stays alive after the call (the app
// server with the plugin as a library), then `worker next` run by another live
// process: the worker's stream takes J-1. The test process is the long-lived
// host: its pid is live for the whole test, as the server's is.
func TestF4dInProcessTaskNewFirstJobTakenByWorker(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("the worker stream needs Unix process identity")
	}
	h := newTaskHarness(t)
	h.env.OwnerPID = 0 // a lib.Call with no OwnerPID, as the app's libCall builds it
	path := newTask(t, h)
	id, err := feed.TaskID(path)
	if err != nil {
		t.Fatal(err)
	}
	ts := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: id}}
	for _, name := range []string{"task", "J-1"} {
		got, err := ts.Read(name)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if got.Holder != nil || got.PID != nil {
			t.Fatalf("after an in-process task new, lease %s is held by %s pid %d (host pid %d)", name, *got.Holder, *got.PID, os.Getpid())
		}
	}
	// A separate grid worker next: another live process than the host.
	worker := os.Getppid()
	began, err := session.ProcessStart(worker)
	if err != nil {
		t.Fatalf("no start for the worker process %d: %v", worker, err)
	}
	start := contract.FormatTime(began)
	var out bytes.Buffer
	s := &orchestrator.Stream{
		Member: "member", Machine: h.env.Machine, Role: "member",
		PID: worker, PIDStart: start, Out: &out, Diag: h.errb,
		Tasks: func() ([]string, error) { return workerTaskDirs(h.env) },
	}
	s.Configure = func(st *orchestrator.Store, taskID string) {
		configureTickStore(h.env, st, filepath.Base(filepath.Dir(filepath.Dir(st.Feed.TaskDir))), taskID)
		st.Diagnostics = h.errb
	}
	if err := s.Pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	var spawned bool
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var got map[string]struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(line), &got) == nil && got["spawn"].ID == firstJobID {
			spawned = true
		}
	}
	if !spawned {
		t.Fatalf("the worker did not take J-1 after an in-process task new\nstream: %q\nstderr: %s", out.String(), h.errb)
	}
	held, err := ts.Read("task")
	if err != nil || held.PID == nil || *held.PID != worker {
		t.Fatalf("the task lease is not the worker's after its pass: %+v %v", held, err)
	}
}
