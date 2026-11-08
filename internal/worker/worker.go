// Package worker reads the app-owned local grid-worker presence files. It never
// starts workers or takes their lifetime lock, so listing cannot block a worker.
package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

// Status is the proposed v1 app/CLI presence format. The architecture specifies
// one process and one OS lock per machine, but no filenames or JSON schema.
// The worker writes worker.json atomically, refreshing UpdatedAt every 30s, and
// holds worker.lock for its lifetime. Both carry the same process identity.
// Keep this provisional type here while internal/contract is frozen.
type Status struct {
	Version   int       `json:"version"`
	Machine   string    `json:"machine"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	State     string    `json:"state"`
}

// Lock is JSON written to the stable, OS-locked worker.lock inode by the app.
type Lock struct {
	Version   int       `json:"version"`
	Machine   string    `json:"machine"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func (l Lock) Validate() error {
	if l.Version != 1 || !gridhome.ValidID(l.Machine) || l.PID <= 0 || l.StartedAt.IsZero() {
		return fmt.Errorf("invalid worker lock identity")
	}
	return nil
}
func (s Status) Validate() error {
	if err := (Lock{Version: s.Version, Machine: s.Machine, PID: s.PID, StartedAt: s.StartedAt}).Validate(); err != nil {
		return err
	}
	if s.UpdatedAt.Before(s.StartedAt) || (s.State != "running" && s.State != "stopped") {
		return fmt.Errorf("invalid worker status")
	}
	return nil
}

type Entry struct {
	Status
	Presence string `json:"presence"` // reported, stale, or unknown; never a PID probe
}
type Store struct {
	GridHome string
	Now      func() time.Time
}

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
	if err = json.Unmarshal(b, out); err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return true, nil
}
func (s Store) List() ([]Entry, error) {
	entries := []Entry{}
	if s.GridHome == "" {
		return entries, nil
	}
	var status Status
	var lock Lock
	hasStatus, err := s.read("worker.json", &status)
	if err != nil {
		return nil, err
	}
	hasLock, err := s.read("worker.lock", &lock)
	if err != nil {
		return nil, err
	}
	if !hasStatus && !hasLock {
		return entries, nil
	}
	machine := gridhome.Machine(s.GridHome)
	if hasLock {
		if err := lock.Validate(); err != nil {
			return nil, err
		}
	}
	if hasStatus {
		if err := status.Validate(); err != nil {
			return nil, err
		}
	}
	if !hasStatus {
		if lock.Machine != machine {
			return entries, nil
		}
		return append(entries, Entry{Status: Status{Version: lock.Version, Machine: lock.Machine, PID: lock.PID, StartedAt: lock.StartedAt, State: "unknown"}, Presence: "unknown"}), nil
	}
	if status.Machine != machine {
		return entries, nil
	}
	presence := "stale"
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	if hasLock && lock.Machine == status.Machine && lock.PID == status.PID && lock.StartedAt.Equal(status.StartedAt) && status.State == "running" && !status.UpdatedAt.After(now) && now.Sub(status.UpdatedAt) <= 90*time.Second {
		presence = "reported"
	}
	return append(entries, Entry{Status: status, Presence: presence}), nil
}
