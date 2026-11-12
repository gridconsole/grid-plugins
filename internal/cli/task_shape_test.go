package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// grid task new, show, list and a folding verb (rename) work against a
// migrated home (VERSION 1, id ULID with display T-NN) and a legacy one (no
// VERSION, id T-NN with ulid). New tasks get the migrated shape in both.
func TestTaskCommandsMigratedAndLegacyShape(t *testing.T) {
	const stable = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, tc := range []struct {
		name, ids string
		version   bool
	}{
		{"migrated", "id: " + stable + "\ndisplay: T-01\n", true},
		{"legacy", "id: T-01\nulid: " + stable + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			if !tc.version {
				if err := os.Remove(filepath.Join(h.gridHome, "VERSION")); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(fixture.Path, "task.md")
			h.write(path, "---\n"+tc.ids+"title: Old title\nproject: "+fixture.Project+"\nsize: small\nworkflow: \"\"\nstate: queued\ncreated: '2026-01-02T03:04:05.000Z'\ncontract: \"1\"\n---\n")

			if code := h.run(h.home, "task", "show", "T-01", "--json"); code != 0 {
				t.Fatalf("show: %d %s", code, h.errb)
			}
			var shown struct{ ID, Title string }
			if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil || shown.ID != "T-01" || shown.Title != "Old title" {
				t.Fatalf("show: %s %v", h.out, err)
			}

			if code := h.run(h.home, "task", "rename", "T-01", "--title", "New title"); code != 0 {
				t.Fatalf("rename: %d %s", code, h.errb)
			}
			text := readTaskTestFile(t, path)
			if !strings.HasPrefix(text, "---\n"+tc.ids) || !strings.Contains(text, "title: \"New title\"\n") {
				t.Fatalf("rename changed identity or missed title:\n%s", text)
			}

			if !tc.version {
				// An old home is no longer stamped by the first write; it is migrated
				// first. Show and rename above ran on it unversioned; the new task is
				// the first write, so the home is given VERSION 1 here.
				if err := os.WriteFile(filepath.Join(h.gridHome, "VERSION"), []byte("1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if code := h.run(h.home, "task", "new", "Second task", "--json"); code != 0 {
				t.Fatalf("new: %d %s", code, h.errb)
			}
			var created struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &created); err != nil || created.ID != "T-02" {
				t.Fatalf("new: %s %v", h.out, err)
			}
			id, err := feed.TaskID(created.Path)
			if err != nil {
				t.Fatal(err)
			}
			text = readTaskTestFile(t, filepath.Join(created.Path, "task.md"))
			if !strings.HasPrefix(text, "---\nid: "+id+"\ndisplay: T-02\n") || strings.Contains(text, "ulid:") || strings.Contains(text, "id: T-02") {
				t.Fatalf("new task is not in the migrated shape:\n%s", text)
			}
			if code := h.run(h.home, "task", "show", "T-02", "--json"); code != 0 {
				t.Fatalf("show new: %d %s", code, h.errb)
			}

			if code := h.run(h.home, "task", "list", "--json"); code != 0 {
				t.Fatalf("list: %d %s", code, h.errb)
			}
			var listed []struct {
				ID, Title  string
				Unreadable bool
			}
			if err := json.Unmarshal(h.out.Bytes(), &listed); err != nil || len(listed) != 2 ||
				listed[0].ID != "T-01" || listed[0].Title != "New title" || listed[0].Unreadable ||
				listed[1].ID != "T-02" || listed[1].Unreadable {
				t.Fatalf("list: %s %v", h.out, err)
			}
		})
	}
}
