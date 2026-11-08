package cli

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskLeaseInvalidMemberCreatesNoLeaseDirectory(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	for _, member := range []string{"bad/member", "bad member", "bad\tmember"} {
		t.Run(member, func(t *testing.T) {
			h, path := feedHarness(t)
			t.Setenv("GRID_MEMBER", member)
			code := h.run(path, taskTakeArgs(pid, start)...)
			_, err := os.Lstat(filepath.Join(path, "feed", "lease"))
			if code != 2 || !os.IsNotExist(err) {
				t.Fatalf("take exit=%d, lease directory error=%v: %s", code, err, h.errb)
			}
		})
	}
}

// The take pauses after ErrTaskHeld. Release either demonstrably contends on
// task-take.lock or finishes in the gap (the regression); no sleeps order work.
func TestTaskLeaseReleaseBetweenHeldCheckAndAdoption(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	deadPID, deadStart := deadTaskProcess(t)
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
	writer := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	first, err := store.TakeTask(writer, deadPID, deadStart)
	if err != nil {
		t.Fatal(err)
	}
	releaseOut, releaseErr := &bytes.Buffer{}, &bytes.Buffer{}
	releaseEnv := h.env
	releaseEnv.Cwd, releaseEnv.Stdout, releaseEnv.Stderr = path, releaseOut, releaseErr
	attempted := make(chan error, 1)
	released := make(chan int, 1)
	beforeTaskLeaseLock = func(action string, lock *os.File) {
		if action == "release" {
			err := fslock.TryLockExclusive(lock)
			if err == nil {
				fslock.Unlock(lock)
			}
			attempted <- err
		}
	}
	var early, reached bool
	beforeTaskAdopt = func() {
		reached = true
		go func() {
			released <- Run([]string{"lease", "release", "task", "--epoch", fmt.Sprintf("0.%d", first.Fence)}, releaseEnv)
		}()
		select {
		case err := <-attempted:
			if !errors.Is(err, fslock.ErrLocked) {
				t.Errorf("release did not contend on take lock: %v", err)
			}
		case code := <-released:
			early = true
			t.Errorf("release crossed held-check/adoption gap: exit %d %s", code, releaseErr)
		case <-time.After(5 * time.Second):
			t.Fatal("release never reached the lock")
		}
	}
	t.Cleanup(func() { beforeTaskAdopt = nil; beforeTaskLeaseLock = nil })
	code := h.run(path, taskTakeArgs(pid, start)...)
	if !reached {
		t.Fatalf("take never reached adoption: %d %s", code, h.errb)
	}
	if !early {
		select {
		case code := <-released:
			if code != 1 {
				t.Errorf("old release: %d %s", code, releaseErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("release stayed blocked after take")
		}
	}
	if code != 0 {
		t.Fatalf("adoption: %d %s", code, h.errb)
	}
	current, err := store.Read("task")
	if err != nil || current.Holder == nil || current.Fence != first.Fence+1 || current.PID == nil || *current.PID != pid {
		t.Fatalf("lost adopted tenure: %+v %v", current, err)
	}
}
