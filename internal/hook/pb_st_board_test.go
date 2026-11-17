package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// pbStHookBoard turns the task of a jobFixture into a per-post board: it makes board/,
// points the hook's machine folder at a folder of its own, and returns the Store that
// writes the board. The task has no feed/ folder.
func pbStHookBoard(t *testing.T, j *jobFixture) feed.Store {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(j.dir, "board"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(j.dir, "feed")); err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	j.env[machinefolder.EnvDir] = folder
	t.Setenv(machinefolder.EnvDir, folder) // jobHasResult has no options: the process environment
	return feed.Store{
		TaskDir: j.dir, TaskID: j.task,
		Machine: machinefolder.Folder{Path: folder, Source: machinefolder.SourceEnv},
		Board:   &feed.BoardHome{Rel: "projects/demo/tasks/T-1", By: peMsgMachine + ":tester"},
	}
}

// pbStResult posts J-1's result, attempt 1, as the worker, with the body of the
// contract's result fixture.
func pbStResult(t *testing.T, s feed.Store) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "result", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: "worker@" + peMsgMachine}
	if _, err := s.Post(feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: fx.Body}, w); err != nil {
		t.Fatalf("post result: %v", err)
	}
}

// The PostToolUse message delivery reads a task's board/ (hooks, brain-board.md Q3): a
// message posted to a board task reaches the agent of its Job.
func TestPbStBoardPeMsgDelivers(t *testing.T) {
	j := jobSetup(t)
	s := pbStHookBoard(t, j)
	if _, err := s.Post(feed.Input{Type: "post", Body: peMsgBody("To J-1: read the board")}, peMsgPerson("owner", "owner")); err != nil {
		t.Fatal(err)
	}
	if got, want := peMsgContext(t, peMsgCall(t, j)), "Message from owner: To J-1: read the board"; got != want {
		t.Fatalf("additionalContext = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(j.dir, "feed")); !os.IsNotExist(err) {
		t.Fatalf("the hook made feed/ in a board task: %v", err)
	}
}

// The stop check asks the board, not feed/, whether the Job posted its result.
func TestPbStBoardJobHasResult(t *testing.T) {
	j := jobSetup(t)
	s := pbStHookBoard(t, j)
	if posted, err := jobHasResult(j.dir, "J-1", "1"); err != nil || posted {
		t.Fatalf("before the result: %v %v", posted, err)
	}
	pbStResult(t, s)
	if posted, err := jobHasResult(j.dir, "J-1", "1"); err != nil || !posted {
		t.Fatalf("after the result: %v %v", posted, err)
	}
	if posted, err := jobHasResult(j.dir, "J-2", "1"); err != nil || posted {
		t.Fatalf("another Job: %v %v", posted, err)
	}
}

// The history snapshot of a Bash call leaves board/ out, as it leaves feed/ out: a post
// that lands during the call is not a file the agent edited.
func TestPbStBoardSnapshotSkipsBoard(t *testing.T) {
	j := jobSetup(t)
	s := pbStHookBoard(t, j)
	if _, err := s.Post(feed.Input{Type: "post", Body: peMsgBody("To J-1: a post")}, peMsgPerson("owner", "owner")); err != nil {
		t.Fatal(err)
	}
	got, err := snapshot(j.dir)
	if err != nil {
		t.Fatal(err)
	}
	for name := range got {
		if filepath.Dir(name) == "board" {
			t.Fatalf("snapshot lists %s", name)
		}
	}
}
