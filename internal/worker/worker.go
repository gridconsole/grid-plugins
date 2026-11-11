// Package worker reads the two files the app's grid-worker writes in the Grid
// home. It never starts workers and never keeps the worker lock: the probe takes
// it for an instant and drops it, so listing cannot block a worker.
package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

const (
	// lockFile is the app's lifetime lock: empty, and held with an OS lock for as
	// long as a worker runs. The app decides "running" by trying this lock, and so
	// does List.
	lockFile = "worker.lock"
	// statusFile is what the app writes while it runs and removes when it stops
	// cleanly; a crash leaves it behind.
	statusFile = "worker-status.json"
)

// appStatus is the part of worker-status.json that List shows. The file holds
// more (uptime_seconds, and the control token); nothing is decoded for those, so
// the token never reaches an Entry.
type appStatus struct {
	PID       int       `json:"pid"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
	Address   string    `json:"address"`
}

// Entry is one worker of this Grid home as `grid worker list` shows it.
// State is "running" while another handle holds worker.lock and "stopped" when
// only a status file is left; Presence is "reported" for the first and "stale"
// for the second. The fields from the status file are zero when the lock is held
// before the worker has written the file.
type Entry struct {
	Machine   string    `json:"machine"`
	PID       int       `json:"pid"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
	Address   string    `json:"address"`
	State     string    `json:"state"`
	Presence  string    `json:"presence"`
}

type Store struct {
	GridHome string
}

// read decodes a small regular file of the Grid home into out. A missing or empty
// file is "not there" (false, nil); a symlink, a directory, a big file or bad JSON
// is an error.
func (s Store) read(name string, out any) (bool, error) {
	path, err := localdata.Safe(s.GridHome, name)
	if err != nil {
		return false, err
	}
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() || st.Size() > 64*1024 {
		return false, fmt.Errorf("%s must be a regular file of at most 64 KiB", name)
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return false, nil
	}
	if err = json.Unmarshal(b, out); err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return true, nil
}

// lockHeld tries the worker lock the way the app does: it opens worker.lock
// (never creating it), takes the exclusive lock without waiting and gives it back
// at once. Another handle holding it means a worker runs.
func (s Store) lockHeld() (bool, error) {
	path, err := localdata.Safe(s.GridHome, lockFile)
	if err != nil {
		return false, err
	}
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() {
		return false, fmt.Errorf("%s must be a regular file", lockFile)
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := fslock.TryLockExclusive(f); err != nil {
		if errors.Is(err, fslock.ErrLocked) {
			return true, nil
		}
		return false, err
	}
	fslock.Unlock(f)
	return false, nil
}

// List returns the worker of this Grid home: none when there is no lock held and
// no status file, "running"/"reported" when the lock is held, "stopped"/"stale"
// when the lock is free and a status file is left behind.
func (s Store) List() ([]Entry, error) {
	entries := []Entry{}
	if s.GridHome == "" {
		return entries, nil
	}
	// Read the status first: the app writes it after it takes the lock and removes
	// it before it drops the lock, so a file read here was written under the lock
	// and the probe after it says whether that lock is still held.
	var status appStatus
	hasStatus, err := s.read(statusFile, &status)
	if err != nil {
		return nil, err
	}
	held, err := s.lockHeld()
	if err != nil {
		return nil, err
	}
	if !held && !hasStatus {
		return entries, nil
	}
	entry := Entry{
		Machine:   gridhome.Machine(s.GridHome),
		PID:       status.PID,
		Version:   status.Version,
		StartedAt: status.StartedAt,
		Address:   status.Address,
		State:     "stopped",
		Presence:  "stale",
	}
	if held {
		entry.State, entry.Presence = "running", "reported"
	}
	return append(entries, entry), nil
}
