package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
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
				_, err := (lease.TaskStore{Feed: feed.Store{TaskDir: fixture.Path}}).TakeTask(feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}, os.Getpid(), "2026-01-01T00:00:00Z")
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
				if name != "history.md" && string(after[name]) != string(data) {
					t.Fatalf("changed %s", name)
				}
			}
			if !strings.Contains(string(after["history.md"]), "restored") {
				t.Fatal("missing restore history")
			}
		})
	}
}
