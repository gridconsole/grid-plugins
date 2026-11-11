package task

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

func runningOptions(id string) WriteOptions {
	return WriteOptions{ID: id, Note: "J-1 assigned", Who: contract.Author{ID: "worker", Role: "cli"}}
}

// readHistory returns history.md, or nil when the task has none.
func readHistory(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return data
}

// A queued task moves to running with the receipt Change writes for action status: the author,
// the note naming the Job, and nothing else in task.md changes. A second call writes no receipt.
func TestRunningFromQueuedMovesQueuedOnce(t *testing.T) {
	s, task := writeFixture(t)
	path := filepath.Join(task.Path, "task.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "state: queued") {
		t.Fatalf("a new task is not queued: %s", before)
	}
	moved, event, err := s.RunningFromQueued(runningOptions(task.ID))
	if err != nil || !moved {
		t.Fatalf("moved %v, %v", moved, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(string(before), "state: queued", "state: running", 1); string(after) != want {
		t.Fatalf("task.md changed more than the state:\n%s\nwant:\n%s", after, want)
	}
	if got, err := Read(task.Path); err != nil || got.State != "running" {
		t.Fatalf("%+v: %v", got, err)
	}
	if event.What != "status" || event.FromState != "queued" || event.ToState != "running" || event.Note != "J-1 assigned" ||
		event.Who != (contract.Author{ID: "worker", Role: "cli"}) || event.Task != task.ID {
		t.Fatalf("receipt: %+v", event)
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 1 {
		t.Fatalf("want one receipt, got %v: %v", lines, err)
	}
	var recorded Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[0], "- ")), &recorded); err != nil || recorded != event {
		t.Fatalf("recorded %+v, %v; want %+v", recorded, err, event)
	}
	history := readHistory(t, task.Path)
	// The second spawn of the same task finds it running: no move, no receipt.
	again := runningOptions(task.ID)
	again.Note = "J-2 assigned"
	moved, event, err = s.RunningFromQueued(again)
	if err != nil || moved || event != (Event{}) {
		t.Fatalf("second call: moved %v, %+v, %v", moved, event, err)
	}
	if got := readHistory(t, task.Path); !bytes.Equal(got, history) {
		t.Fatalf("a second call wrote a receipt:\n%s", got)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, after) {
		t.Fatal("a second call changed task.md")
	}
}

// Only queued moves. The table lets waiting, stuck and paused go to running too, and a person may
// have set one of them a moment before; every other state is not queued either. Each is left
// byte for byte, with no receipt and no error.
func TestRunningFromQueuedLeavesEveryOtherState(t *testing.T) {
	for _, state := range []string{"running", "waiting", "stuck", "reworking", "pausing", "paused", "closed"} {
		t.Run(state, func(t *testing.T) {
			s, task := writeFixture(t)
			path := filepath.Join(task.Path, "task.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			write(t, path, strings.Replace(string(data), "state: queued", "state: "+state, 1))
			write(t, filepath.Join(task.Path, "history.md"), "- legacy entry\n")
			task, err = Read(task.Path)
			if err != nil || task.State != state {
				t.Fatalf("setup: %+v: %v", task, err)
			}
			before, _ := os.ReadFile(path)
			history := readHistory(t, task.Path)
			moved, event, err := s.RunningFromQueued(runningOptions(task.ID))
			if err != nil || moved || event != (Event{}) {
				t.Fatalf("moved %v, %+v, %v", moved, event, err)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, before) {
				t.Fatalf("task.md changed:\n%s\nwas:\n%s", got, before)
			}
			if got := readHistory(t, task.Path); !bytes.Equal(got, history) {
				t.Fatalf("a receipt was written:\n%s", got)
			}
			if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
				t.Fatalf("a journal was left: %v", err)
			}
		})
	}
}
