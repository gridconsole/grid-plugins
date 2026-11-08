package feed

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTaskID(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		invalid    bool
	}{
		{"display", "---\nid: T-01\n---\nbody\n", false},
		{"CRLF", "---\r\nid: T-01\r\n---\r\nbody\r\n", false},
		{"empty", "---\nid: T-01\nulid: ''\n---\n", false},
		{"existing id", "---\nid: '" + testTask + "'\n---\n", false},
		{"existing ulid", "---\nid: T-01\nulid: " + testTask + "\n---\n", false},
		{"invalid ulid", "---\nid: T-01\nulid: broken\n---\n", true},
		{"conflicting", "---\nid: " + testTask + "\nulid: 01ARZ3NDEKTSV4RRFFQ69G5FAX\n---\n", true},
		{"unclosed", "---\nid: T-01\n", true},
		{"no frontmatter", "body\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "task.md")
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			old, oldErr := TaskID(dir)
			id, err := EnsureTaskID(dir)
			if (err != nil) != tc.invalid {
				t.Fatalf("id %q, error %v", id, err)
			}
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.invalid || oldErr == nil {
				if !bytes.Equal(first, []byte(tc.text)) {
					t.Fatal("existing or invalid identity was changed")
				}
			}
			if tc.invalid {
				return
			}
			if !IsID(id) || oldErr == nil && id != old {
				t.Fatalf("identity %q, previous %q", id, old)
			}
			again, err := EnsureTaskID(dir)
			if err != nil || again != id {
				t.Fatalf("second identity %q, want %q: %v", again, id, err)
			}
			second, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("second write changed task: %q, %v", second, err)
			}
		})
	}
}
