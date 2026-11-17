package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// pbStBoardFixture is the fixture of server_test.go on a per-post board: the task has
// a board/ folder, and its caches are in a machine folder of their own.
func pbStBoardFixture(t *testing.T) (Server, feed.Store) {
	t.Helper()
	s, _, _ := fixture(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "board"), 0o700); err != nil {
		t.Fatal(err)
	}
	store := feed.Store{
		TaskDir: dir, TaskID: taskID,
		Machine: machinefolder.Folder{Path: t.TempDir(), Source: machinefolder.SourceEnv},
		Board:   &feed.BoardHome{Rel: "tasks/T-07", By: machineID + ":tester"},
	}
	s.Resolve = func(task, project string) (feed.Store, error) { return store, nil }
	return s, store
}

// feed_post on a board answers with by and the file, writes board/<seq>-task-post.md
// and no feed/ folder; feed_read answers the Q3 record shape (brain-board.md Q3): by and
// file, worker as an alias of by, and no lease, fence, end or gen.
func TestPbStBoardPostRead(t *testing.T) {
	s, store := pbStBoardFixture(t)
	post := request("feed_post", `{"type":"post","body":{"ref":"task","kind":"message","text":"example"}}`)
	out := result(t, last(t, exchange(t, s, prelude+post)))
	if out.IsError {
		t.Fatalf("post: %+v", out)
	}
	text := out.Content[0].Text
	for _, want := range []string{`"by":"` + machineID + `:tester"`, `"file":"000001-task-post.md"`, `"worker":"` + machineID + `:tester"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("post reply lacks %s: %s", want, text)
		}
	}
	for _, gone := range []string{`"lease"`, `"fence"`, `"end"`, `"gen"`} {
		if strings.Contains(text, gone) {
			t.Fatalf("post reply has %s: %s", gone, text)
		}
	}
	if _, err := os.Stat(filepath.Join(store.TaskDir, "board", "000001-task-post.md")); err != nil {
		t.Fatalf("post file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.TaskDir, "feed")); !os.IsNotExist(err) {
		t.Fatalf("feed/ in a board task: %v", err)
	}

	out = result(t, last(t, exchange(t, s, prelude+request("feed_read", `{}`))))
	if out.IsError {
		t.Fatalf("read: %+v", out)
	}
	text = out.Content[0].Text
	for _, want := range []string{`"by":"` + machineID + `:tester"`, `"file":"000001-task-post.md"`, `"text":"example"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("read lacks %s: %s", want, text)
		}
	}
	for _, gone := range []string{`"lease"`, `"fence"`, `"end"`, `"gen"`} {
		if strings.Contains(text, gone) {
			t.Fatalf("read has %s: %s", gone, text)
		}
	}
}

// feed_handover is a Feed lease record; a board has none, so it is refused and nothing
// is written.
func TestPbStBoardHandoverRefused(t *testing.T) {
	s, store := pbStBoardFixture(t)
	out := result(t, last(t, exchange(t, s, prelude+request("feed_handover", `{"to":"member@`+machineID+`"}`))))
	if !out.IsError {
		t.Fatalf("handover on a board: %+v", out)
	}
	if _, err := os.Stat(filepath.Join(store.TaskDir, "feed")); !os.IsNotExist(err) {
		t.Fatalf("feed/ in a board task: %v", err)
	}
}
