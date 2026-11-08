package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

func livenessProject(h *harness) string {
	project := filepath.Join(h.gridHome, "projects", "example")
	h.write(filepath.Join(project, "project.md"), "---\nid: EXAMPLE\n---\n")
	task := filepath.Join(project, "tasks", "T-01-example")
	h.mkdir(task)
	return task
}

func TestLeaseCommands(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "lease", "status", "--json"); code != 0 {
		t.Fatalf("status: %d %s", code, h.errb)
	}
	var status lease.Status
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || status.Held {
		t.Fatalf("status: %+v %v", status, err)
	}
	project := filepath.Dir(filepath.Dir(task))
	if _, err := os.Stat(filepath.Join(project, "lease")); !os.IsNotExist(err) {
		t.Fatal("status wrote files")
	}
	if code := h.run(task, "lease", "take", "merge", "--holder", "session-1"); code != 0 || h.out.String() != "merge lease taken, fence 1\n" {
		t.Fatalf("take: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "lease", "take", "merge", "--project", "example", "--holder", "session-2"); code != 1 || !strings.HasPrefix(h.errb.String(), "merge lease held by session-1 since ") {
		t.Fatalf("refusal: %d %s", code, h.errb)
	}
	if code := h.run(task, "lease", "renew", "merge", "--holder", "session-1", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("renew: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || !status.Held || status.Fence != 1 {
		t.Fatalf("renew: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "release", "--holder", "session-2", "--fence", "1"); code != 1 {
		t.Fatalf("wrong owner: %d", code)
	}
	if code := h.run(task, "lease", "release", "merge", "--holder", "session-1", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("release: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || status.Held || status.Holder != nil {
		t.Fatalf("release: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "take", "--holder", "session-2"); code != 0 || h.out.String() != "merge lease taken, fence 2\n" {
		t.Fatalf("retake: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(task, "lease", "release", "--holder", "session-1", "--fence", "1"); code != 1 {
		t.Fatalf("stale owner: %d", code)
	}
}

func TestSessionCommands(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "session", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("empty list: %d %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(task, "sessions")); !os.IsNotExist(err) {
		t.Fatal("list wrote files")
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := session.Store{TaskDir: task, Now: func() time.Time { return now }}
	record := session.Record{Key: session.Key{SessionID: "session-1", MachineID: "machine-1"}, PID: 42, PIDStart: "start-1"}
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "session", "show", "session-1"); code != 0 || !strings.Contains(h.out.String(), "state: Liveness unknown\n") {
		t.Fatalf("show: %d %s %s", code, h.out, h.errb)
	}
	if _, err := store.Sleep(record.Key); err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.tmp, "session", "list", "--project", "example", "--json"); code != 0 {
		t.Fatalf("list project: %d %s", code, h.errb)
	}
	var views []sessionView
	if err := json.Unmarshal(h.out.Bytes(), &views); err != nil || len(views) != 1 || views[0].State != session.Sleeping || views[0].Task != "T-01-example" {
		t.Fatalf("list: %+v %v", views, err)
	}
	record.MachineID = "machine-2"
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "session", "show", "session-1"); code != 1 || !strings.Contains(h.errb.String(), "ambiguous") {
		t.Fatalf("ambiguous: %d %s", code, h.errb)
	}
	if code := h.run(h.tmp, "session", "show", "session-1", "--project", "example", "--task", "T-01", "--machine", "machine-1", "--json"); code != 0 {
		t.Fatalf("show exact: %d %s", code, h.errb)
	}
	var view sessionView
	if err := json.Unmarshal(h.out.Bytes(), &view); err != nil || view.State != session.Sleeping || view.MachineID != "machine-1" {
		t.Fatalf("show exact: %+v %v", view, err)
	}
	if code := h.run(task, "session", "show", "missing"); code != 1 {
		t.Fatalf("missing: %d", code)
	}
}

func TestLivenessCommandUsage(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"lease"}, {"lease", "take"}, {"lease", "renew", "--holder", "session-1"},
		{"lease", "release", "--holder", "session-1", "--fence", "bad"},
		{"lease", "take-over"}, {"lease", "take", "task", "--holder", "session-1"},
		{"session"}, {"session", "show"}, {"session", "list", "extra"},
	} {
		if code := h.run(h.tmp, args...); code != 2 {
			t.Errorf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	for _, args := range [][]string{{"session", "list", "--help"}, {"lease", "take", "--help"}} {
		if code := h.run(h.tmp, args...); code != 0 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	h.env.GridHome = ""
	for _, args := range [][]string{{"session", "list"}, {"lease", "status"}} {
		if code := h.run(h.tmp, args...); code != 1 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
