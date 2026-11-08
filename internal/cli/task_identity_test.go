package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestTaskNewThenQuestionAdd(t *testing.T) {
	h := newHarness(t)
	h.env.Machine = ""
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_WORKER", "")
	if code := h.run(h.home, "task", "new", "Choose the export format", "--project", "personal", "--json"); code != 0 {
		t.Fatalf("new: %d %s", code, h.errb)
	}
	var created struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// Check creation separately: lazy initialization must not hide a missing ID.
	id, err := feed.TaskID(created.Path)
	if err != nil || !feed.IsID(id) {
		t.Errorf("new task identity: %q, %v", id, err)
	}
	q := questionAdd(t, h, created.Path, "member")
	read, err := (feed.Store{TaskDir: created.Path, TaskID: id}).ReadOnly(feed.Filter{})
	if err != nil || len(read.Records) != 1 || read.Records[0].ID != q.Post || read.Records[0].Task != id {
		t.Fatalf("question Feed: %+v, %v", read, err)
	}
}

func TestLegacyTaskIdentityCLIAndMCP(t *testing.T) {
	for _, writer := range []string{"CLI", "MCP"} {
		t.Run(writer, func(t *testing.T) {
			h, dir := feedHarness(t)
			t.Setenv("GRID_FEED_TASK", dir)
			path := filepath.Join(dir, "task.md")
			original := []byte("---\nid: T-07\n---\n")
			h.write(path, string(original))
			for _, verb := range []string{"read", "state", "fold"} {
				if code := h.run(dir, "feed", verb); code == 0 {
					t.Fatalf("%s unexpectedly found an identity", verb)
				}
			}
			for _, tool := range []string{"feed_read", "feed_state"} {
				if out := mcpCall(t, h, tool, `{}`); !strings.Contains(out, `"isError":true`) {
					t.Fatalf("%s unexpectedly found an identity: %s", tool, out)
				}
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("reader mutated task: %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(dir, ".task.lock")); !os.IsNotExist(err) {
				t.Fatalf("reader created task lock: %v", err)
			}
			body := `{"ref":"task","kind":"message","text":"First write"}`
			if writer == "CLI" {
				if code := h.run(dir, "feed", "post", "--type", "post", "--body", body); code != 0 {
					t.Fatalf("post: %d %s", code, h.errb)
				}
			} else if out := mcpCall(t, h, "feed_post", `{"type":"post","body":`+body+`}`); strings.Contains(out, `"isError":true`) {
				t.Fatalf("post: %s", out)
			}
			id, err := feed.TaskID(dir)
			if err != nil || !feed.IsID(id) {
				t.Fatalf("first write identity: %q, %v", id, err)
			}
			read, err := (feed.Store{TaskDir: dir, TaskID: id}).ReadOnly(feed.Filter{})
			if err != nil || len(read.Records) != 1 || read.Records[0].Task != id {
				t.Fatalf("post identity: %+v, %v", read, err)
			}
		})
	}
}
