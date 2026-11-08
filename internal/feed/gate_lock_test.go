package feed

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func TestGateLockCoversCheckAndAppend(t *testing.T) {
	s := testStore(t)
	// Precreate the probe path so removing gateLock fails on lock ownership,
	// not merely on its missing directory or file.
	if err := os.MkdirAll(filepath.Join(s.Dir(), ".cache"), 0700); err != nil {
		t.Fatal(err)
	}
	probe, err := fslock.OpenNoFollow(filepath.Join(s.Dir(), ".cache", "gate.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	checked, resume := make(chan struct{}), make(chan struct{})
	s.afterGateCheck = func() { close(checked); <-resume }
	first := make(chan error, 1)
	go func() { _, err := s.Post(gatedQuestion("Q-1"), testWriter()); first <- err }()
	released := false
	defer func() {
		if !released {
			close(resume)
		}
	}()
	select {
	case <-checked:
	case err := <-first:
		t.Fatalf("post did not reach gate hook: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("post did not reach gate hook")
	}
	// The writer is paused after checking Q-1 and has not appended it. Probe
	// the actual file lock, avoiding a sleep or a scheduling-based assertion.
	err = fslock.TryLockExclusive(probe)
	if !errors.Is(err, fslock.ErrLocked) {
		t.Errorf("gate lock not held between check and append: %v", err)
	}
	fslock.Unlock(probe)

	other := Store{TaskDir: s.TaskDir, TaskID: s.TaskID}
	w := testWriter()
	w.Author.ID = "other"
	w.Worker = "other@" + testMachine
	second := make(chan error, 1)
	go func() { _, err := other.Post(gatedQuestion("Q-1"), w); second <- err }()
	close(resume)
	released = true
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("second claim must be rejected: %v", err)
	}
	records, err := s.Read(Filter{})
	if err != nil || len(records.Records) != 1 {
		t.Fatalf("records=%d err=%v", len(records.Records), err)
	}
}
