package cli

import (
	"encoding/json"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestTaskTrashRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, role, agent, verb string
		code                    int
	}{
		{"member", "member", "", "delete", 3}, {"guest", "guest", "", "delete", 3},
		{"agent", "owner", "worker", "delete", 3}, {"planner", "owner", "", "delete", 3},
		{"live_lease", "owner", "", "delete", 3}, {"unknown", "owner", "", "delete", 1},
		{"no_copy", "owner", "", "restore", 3}, {"restore_member", "member", "", "restore", 3},
		{"restore_guest", "guest", "", "restore", 3}, {"restore_agent", "owner", "worker", "restore", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			h.env.Getenv = func(k string) string {
				switch k {
				case "GRID_ROLE":
					return tc.role
				case "GRID_AGENT":
					return tc.agent
				case "GRID_ACTOR":
					return "alex"
				}
				return ""
			}
			if tc.name == "planner" {
				p := filepath.Join(fixture.Path, "task.md")
				h.write(p, strings.Replace(readTaskTestFile(t, p), "state:", "kind: planner\nstate:", 1))
			}
			if tc.name == "live_lease" {
				_, err := (lease.TaskStore{Feed: feed.Store{TaskDir: fixture.Path, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}).TakeTask(feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}, os.Getpid(), "2026-01-01T00:00:00.000Z")
				if err != nil {
					t.Fatal(err)
				}
			}
			id := fixture.ID
			if tc.name == "unknown" {
				id = "T-999"
			}
			if code := h.run(h.home, "task", tc.verb, id, "--project", "personal"); code != tc.code {
				t.Fatalf("exit = %d, want %d: %s", code, tc.code, h.errb)
			}
			if _, err := os.Stat(fixture.Path); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(h.gridHome, "trash")); !os.IsNotExist(err) {
				t.Fatalf("refusal created trash: %v", err)
			}
		})
	}
}

func TestTaskTrashRoundTrip(t *testing.T) {
	for _, state := range []string{"queued", "closed"} {
		t.Run(state, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			p := filepath.Join(fixture.Path, "task.md")
			h.write(p, strings.Replace(readTaskTestFile(t, p), "state: queued", "state: "+state, 1))
			h.write(filepath.Join(fixture.Path, "history.md"), "- Earlier history\n")
			h.write(filepath.Join(fixture.Path, "artifacts", "proof.bin"), "\x00exact bytes\xff")
			before := taskTestSnapshot(t, fixture.Path)
			if code := h.run(h.home, "task", "delete", fixture.ID, "--json"); code != 0 {
				t.Fatalf("delete exit = %d: %s", code, h.errb)
			}
			var result struct{ Project, ID, Folder string }
			if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Project != "personal" || result.ID != fixture.ID {
				t.Fatalf("receipt: %+v", result)
			}
			trash := filepath.Join(h.gridHome, "trash", "personal", result.Folder)
			if _, err := os.Stat(fixture.Path); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			if !strings.Contains(readTaskTestFile(t, filepath.Join(trash, "history.md")), "deleted") {
				t.Fatal("missing delete history")
			}
			if code := h.run(h.home, "task", "restore", fixture.ID, "--project", "personal", "--json"); code != 0 {
				t.Fatalf("restore exit = %d: %s", code, h.errb)
			}
			after := taskTestSnapshot(t, fixture.Path)
			for name, data := range before {
				if name != "/history.md" && string(after[name]) != string(data) {
					t.Fatalf("changed %s", name)
				}
			}
			if !strings.Contains(string(after["/history.md"]), "restored") {
				t.Fatal("missing restore history")
			}
		})
	}
}

func TestTaskTrashLeaseAcquisitionWaitsForMove(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	lock, err := fslock.OpenNoFollow(filepath.Join(fixture.Path, ".task.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	defer fslock.Unlock(lock)
	done := make(chan error, 1)
	go func() {
		_, err := (lease.TaskStore{Feed: feed.Store{TaskDir: fixture.Path}}).TakeTask(feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}, os.Getpid(), "2026-01-01T00:00:00.000Z")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("lease bypassed task lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	dest := filepath.Join(h.gridHome, "moved")
	if err := os.Rename(fixture.Path, dest); err != nil {
		t.Fatal(err)
	}
	fslock.Unlock(lock)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("lease granted after task moved")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lease stuck")
	}
	if _, err := os.Stat(fixture.Path); !os.IsNotExist(err) {
		t.Fatalf("lease recreated task: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "feed")); !os.IsNotExist(err) {
		t.Fatalf("lease modified trash: %v", err)
	}
}

func TestTaskTrashProjectAndIdentity(t *testing.T) {
	h := newHarness(t)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	fixture, err := taskWriter(h.env).New(task.NewOptions{Cwd: h.home, Project: "example", Words: "Example task", Who: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	h.env.Getenv = func(k string) string {
		switch k {
		case "GRID_ACTOR":
			return "operator"
		case "GRID_ROLE":
			return "owner"
		case "GRID_WHO":
			return "ignored"
		}
		return ""
	}
	if code := h.run(h.home, "task", "delete", fixture.ID, "--project", "01EXAMPLE", "--json"); code != 0 {
		t.Fatalf("delete: %d %s", code, h.errb)
	}
	var result task.TrashResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Project != "example" {
		t.Fatalf("result: %+v", result)
	}
	history := readTaskTestFile(t, filepath.Join(h.gridHome, "trash", "example", result.Folder, "history.md"))
	if !strings.Contains(history, `"who":{"id":"operator","role":"owner"}`) || strings.Contains(history, "ignored") {
		t.Fatal(history)
	}
	if code := h.run(h.home, "task", "restore", fixture.ID, "--project", "example"); code != 0 {
		t.Fatalf("restore: %d %s", code, h.errb)
	}
	if h.out.String() != filepath.Base(fixture.Path)+"\n" {
		t.Fatalf("plain output: %s", h.out)
	}
}

func TestTaskTrashSymlinkExit(t *testing.T) {
	for _, verb := range []string{"delete", "restore"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			target := filepath.Join(h.gridHome, "target")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(h.gridHome, "trash")); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip("symlink creation requires Windows Developer Mode or privilege")
				}
				t.Fatal(err)
			}
			if code := h.run(h.home, "task", verb, fixture.ID, "--project", "personal"); code != 2 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
				t.Fatalf("link target changed: %v", err)
			}
		})
	}
}
