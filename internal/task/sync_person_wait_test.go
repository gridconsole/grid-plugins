package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setState(t *testing.T, s Store, task Task, extra, state string) Task {
	t.Helper()
	path := filepath.Join(task.Path, "task.md")
	data, _ := os.ReadFile(path)
	write(t, path, strings.Replace(string(data), "state: queued", "state: "+state+extra, 1))
	got, err := Read(task.Path)
	if err != nil || got.State != state {
		t.Fatalf("setup: %+v: %v", got, err)
	}
	return got
}

// A running task whose open Jobs all wait on a person shows waiting, and shows running again
// when an agent Job is open; a second call with the same answer writes nothing.
func TestSyncPersonWaitMovesBothWays(t *testing.T) {
	s, task := writeFixture(t)
	task = setState(t, s, task, "", "running")
	moved, event, err := s.SyncPersonWait(runningOptions(task.ID), true)
	if err != nil || !moved || event.ToState != "waiting" {
		t.Fatalf("to waiting: %v %+v %v", moved, event, err)
	}
	if got, _ := Read(task.Path); got.State != "waiting" {
		t.Fatalf("state %q", got.State)
	}
	if moved, _, err := s.SyncPersonWait(runningOptions(task.ID), true); err != nil || moved {
		t.Fatalf("again: %v %v", moved, err)
	}
	moved, event, err = s.SyncPersonWait(runningOptions(task.ID), false)
	if err != nil || !moved || event.ToState != "running" {
		t.Fatalf("to running: %v %+v %v", moved, event, err)
	}
}

// A task waiting at step ship is the delivery's wait, not a person Job's: it stays.
func TestSyncPersonWaitLeavesShipWait(t *testing.T) {
	s, task := writeFixture(t)
	task = setState(t, s, task, "\nstep: ship", "waiting")
	if moved, _, err := s.SyncPersonWait(runningOptions(task.ID), false); err != nil || moved {
		t.Fatalf("moved %v %v", moved, err)
	}
	if got, _ := Read(task.Path); got.State != "waiting" {
		t.Fatalf("state %q", got.State)
	}
}
