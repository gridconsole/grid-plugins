package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func pbStWriteBoardFile(t *testing.T, dir, name, text string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The quiet-task digest of a task on a per-post board is the sorted names of
// board/*.md (brain-board.md Q2): a new post makes the key differ, so the task is read;
// a rewritten mod time, or a file in board/.tmp, does not.
func TestPbStQuietTaskBoardKey(t *testing.T) {
	dir := t.TempDir()
	pbStWriteBoardFile(t, dir, "board/000001-task-post.md", "one")
	store := feed.Store{TaskDir: dir}
	first := ohSqFeedKey(store)
	if !first.ok {
		t.Fatal("no key for a board task")
	}
	if again := ohSqFeedKey(store); again != first {
		t.Fatal("the key changed with nothing written")
	}

	// A post is immutable: its size and mod time are not in the key.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "board", "000001-task-post.md"), later, later); err != nil {
		t.Fatal(err)
	}
	if got := ohSqFeedKey(store); got != first {
		t.Fatal("a mod time changed the board key")
	}

	// A file in board/.tmp is a post being written, not a post.
	pbStWriteBoardFile(t, dir, "board/.tmp/000002-task-post.md", "two")
	if got := ohSqFeedKey(store); got != first {
		t.Fatal("board/.tmp changed the board key")
	}

	// A post that lands makes the key differ, so the next pass reads the task.
	pbStWriteBoardFile(t, dir, "board/000002-task-post.md", "two")
	if got := ohSqFeedKey(store); got == first {
		t.Fatal("a new post left the board key as it was")
	}
}
