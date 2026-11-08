package feed_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func TestConcurrentFirstWritesKeepOneTaskULID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")
	original := []byte("---\nid: T-01\n# Keep this comment\nextra: [one, two]\n---\n\n## Outcome\nKeep this body.\n")
	if err := os.WriteFile(path, original, 0640); err != nil {
		t.Fatal(err)
	}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@01ARZ3NDEKTSV4RRFFQ69G5FAW"}
	s := question.Store{TaskDir: dir, Writer: w}
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("reader changed task: %q, %v", got, err)
	}
	// Both entry points must reach the allocation lock before either proceeds.
	var arrived atomic.Int32
	ready := make(chan struct{})
	var checked atomic.Int32
	feed.SetTaskIDHooks(t, func() {
		if arrived.Add(1) == 2 {
			close(ready)
		}
		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			t.Error("both writers did not reach the task lock")
		}
	}, func() {
		// A barrier alone cannot force overlapping reads without the lock.
		// Probe from a separate descriptor while metadata is protected: this
		// detects a missing lock on every execution, regardless of scheduling.
		probe, err := os.OpenFile(filepath.Join(dir, ".task.lock"), os.O_RDWR, 0)
		if err != nil {
			t.Error(err)
			return
		}
		defer probe.Close()
		err = fslock.TryLockExclusive(probe)
		if err == nil {
			fslock.Unlock(probe)
		}
		if !errors.Is(err, fslock.ErrLocked) {
			t.Errorf("task metadata read without the task lock: probe = %v", err)
		}
		checked.Add(1)
	})
	input := question.Input{Question: "Continue?", Who: "owner", Default: "yes", Applies: time.Now().Add(time.Hour)}
	errs := make(chan error, 2)
	go func() {
		_, err := s.Add(input)
		errs <- err
	}()
	go func() {
		_, err := (feed.Store{TaskDir: dir}).Post(feed.Input{Type: "question", Body: map[string]any{"question": "Also continue?", "who": "owner"}}, w)
		errs <- err
	}()
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if arrived.Load() < 2 || checked.Load() < 2 {
		t.Fatal("both writers must exercise the allocation hooks")
	}
	if t.Failed() {
		return
	}
	id, err := feed.TaskID(dir)
	if err != nil || !feed.IsID(id) {
		t.Fatalf("task identity: %q, %v", id, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || bytes.Count(got, []byte("ulid: ")) != 1 || !bytes.Equal(bytes.Replace(got, []byte("ulid: "+id+"\n"), nil, 1), original) {
		t.Fatalf("task content changed beyond identity: %q, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("task permissions changed: %v, %v", info, err)
	}
	if _, err := s.Add(input); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, got) {
		t.Fatalf("later write changed identity: %q, %v", after, err)
	}
	read, err := (feed.Store{TaskDir: dir, TaskID: id}).ReadOnly(feed.Filter{})
	if err != nil || len(read.Records) != 3 {
		t.Fatalf("Feed: %+v, %v", read, err)
	}
	for _, entry := range read.Records {
		if entry.Task != id || entry.Verdict != "counted" {
			t.Fatalf("writer used a different task identity: %+v", entry)
		}
	}
}
