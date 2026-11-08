package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
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
	if code := h.run(task, "lease", "take", "merge", "--holder", "worker@01K9J000000000000000000001"); code != 0 || h.out.String() != "merge lease taken, fence 1\n" {
		t.Fatalf("take: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "lease", "take", "merge", "--project", "example", "--holder", "worker@01K9J000000000000000000002"); code != 1 || !strings.HasPrefix(h.errb.String(), "merge lease held by worker@01K9J000000000000000000001 since ") {
		t.Fatalf("refusal: %d %s", code, h.errb)
	}
	if code := h.run(task, "lease", "renew", "merge", "--holder", "worker@01K9J000000000000000000001", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("renew: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || !status.Held || status.Fence != 1 {
		t.Fatalf("renew: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "release", "--holder", "worker@01K9J000000000000000000002", "--fence", "1"); code != 1 {
		t.Fatalf("wrong owner: %d", code)
	}
	if code := h.run(task, "lease", "release", "merge", "--holder", "worker@01K9J000000000000000000001", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("release: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || status.Held || status.Holder != nil {
		t.Fatalf("release: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "take", "--holder", "worker@01K9J000000000000000000002"); code != 0 || h.out.String() != "merge lease taken, fence 2\n" {
		t.Fatalf("retake: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(task, "lease", "release", "--holder", "worker@01K9J000000000000000000001", "--fence", "1"); code != 1 {
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

func TestSessionCommandsReachAPersonalTask(t *testing.T) {
	h := newHarness(t)
	task := filepath.Join(h.gridHome, "personal", "tasks", "T-01-note")
	h.mkdir(task)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := session.Store{TaskDir: task, Now: func() time.Time { return now }}
	record := session.Record{Key: session.Key{SessionID: "session-1", MachineID: "machine-1"}, PID: 42, PIDStart: "start-1"}
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		cwd  string
		args []string
	}{
		{"inside the task", filepath.Join(task, "artifacts"), []string{"session", "list", "--json"}},
		{"project personal", h.tmp, []string{"session", "list", "--project", "personal", "--json"}},
		{"project Personal from the Grid home", h.gridHome, []string{"session", "list", "--project", "Personal", "--json"}},
	} {
		h.out.Reset()
		if code := h.run(tc.cwd, tc.args...); code != 0 {
			t.Fatalf("%s: %d %s", tc.name, code, h.errb)
		}
		var views []sessionView
		if err := json.Unmarshal(h.out.Bytes(), &views); err != nil || len(views) != 1 || views[0].SessionID != "session-1" || views[0].Task != "T-01-note" {
			t.Fatalf("%s: %+v %v %s", tc.name, views, err, h.out)
		}
	}
	h.out.Reset()
	if code := h.run(h.tmp, "session", "show", "session-1", "--project", "personal", "--task", "T-01", "--json"); code != 0 {
		t.Fatalf("show by id: %d %s", code, h.errb)
	}
	var view sessionView
	if err := json.Unmarshal(h.out.Bytes(), &view); err != nil || view.SessionID != "session-1" {
		t.Fatalf("show by id: %+v %v", view, err)
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

func TestLeaseCommandWritesContract(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "lease", "take", "--holder", "worker@01K9J000000000000000000002"); code != 0 {
		t.Fatalf("take: %d %s", code, h.errb)
	}
	data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "example", "lease", "merge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("merge-lease", data); err != nil {
		t.Fatal(err)
	}
	var r contract.MergeLease
	if err := json.Unmarshal(data, &r); err != nil || r.Project != "example" {
		t.Fatalf("project: %+v %v", r, err)
	}
}

func TestLeaseRejectsInvalidHolderBeforeWriting(t *testing.T) {
	for _, holder := range []string{"foo", "worker@bad", "worker/name@01K9J000000000000000000002", "worker name@01K9J000000000000000000002", "worker@81K9J000000000000000000002"} {
		t.Run(holder, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			for _, action := range []string{"take", "renew", "release"} {
				if code := h.run(task, "lease", action, "--holder", holder, "--fence", "1"); code != 2 || !strings.Contains(h.errb.String(), "--holder must be <member>@<machine ULID>") {
					t.Fatalf("%s: exit %d: %s", action, code, h.errb)
				}
			}
			if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "example", "lease")); !os.IsNotExist(err) {
				t.Fatalf("invalid holder created lease files: %v", err)
			}
		})
	}
}

func TestLeaseCommandPreservesStoredGeneration(t *testing.T) {
	for _, held := range []bool{false, true} {
		name := "released"
		if held {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			holder := "worker@01K9J000000000000000000002"
			r := contract.MergeLease{Lease: "merge", Project: "example", Gen: 7, Fence: 4, Since: "2000-01-01T00:00:00.000Z", Until: "2000-01-01T00:15:00.000Z"}
			if held {
				r.Holder = &holder
			}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := contract.Validate("lease", data); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(h.gridHome, "projects", "example", "lease", "merge.json")
			h.write(path, string(data))
			for _, action := range []string{"take", "renew", "release"} {
				if code := h.run(task, "lease", action, "--holder", holder, "--fence", "5"); code != 0 {
					t.Fatalf("%s: exit %d: %s", action, code, h.errb)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := contract.Validate("lease", data); err != nil {
					t.Fatalf("%s: %v", action, err)
				}
				if err := json.Unmarshal(data, &r); err != nil || r.Gen != 7 || r.Fence != 5 {
					t.Fatalf("%s lost generation or fence: %+v %v", action, r, err)
				}
			}
		})
	}
}
