package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/session"
)

type Row struct {
	ID    string          `json:"id"`
	Role  string          `json:"role"`
	State contract.Status `json:"state"`
	Why   string          `json:"why"`
}
type Wake struct {
	Job    string   `json:"job"`
	Event  string   `json:"event"`
	N      int      `json:"n"`
	Events []string `json:"events,omitempty"`
}
type Output struct {
	Rows     []Row `json:"jobs"`
	Wake     *Wake `json:"wake,omitempty"`
	Assigned int   `json:"-"`
}

// Store acts only as Writer, the current task lease holder. Probe must compare
// both pid and start time. Without a probe, a readable heartbeat supplies its
// expiry; AfterWake suppresses expiry until supervision has checked processes.
// UsageReady must positively confirm capacity after a usage_limit exit.
type Store struct {
	taskProgress string
	progressMemo map[string]string
	AfterWake    bool
	WorktreesDir string
	Feed         feed.Store
	Writer       feed.Writer
	Assignee     string
	Owner        string
	Now          func() time.Time
	Probe        func(session.Live) session.ProcessState
	UsageReady   func(string) bool
	Incident     func(string, []string) error
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

type jobMemory struct {
	ObservedFence int64     `json:"observed_fence,omitempty"`
	LastCheck     time.Time `json:"last_check,omitempty"`
	WakePending   bool      `json:"wake_pending,omitempty"`
	Failure       string    `json:"failure"`
	Failures      []string  `json:"failures"`
	Progress      string    `json:"progress"`
	ParkProgress  string    `json:"park_progress"`
	Liveness      string    `json:"liveness,omitempty"`
}
type progressFile struct {
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Hash  string `json:"hash"`
}
type memory struct {
	OfflineHosts map[string]bool         `json:"offline_hosts,omitempty"`
	Files        map[string]progressFile `json:"files,omitempty"`
	WakeVersion  int                     `json:"wake_version"`
	WakeCounts   map[string]int          `json:"wake_counts"`
	Active       *Wake                   `json:"active,omitempty"`
	Overflow     bool                    `json:"overflow"`
	Jobs         map[string]*jobMemory   `json:"jobs"`
	Delivered    map[string]bool         `json:"delivered"`
	Wakes        []time.Time             `json:"wakes"`
	Pending      []Wake                  `json:"pending"`
}

func (s Store) locked(fn func() error) error {
	// Check before creating the lock, and again while the pass is serialized.
	return s.Feed.WithLease(s.Writer, nil, func() error {
		f, err := fslock.OpenNoFollow(filepath.Join(s.Feed.Dir(), ".orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := fslock.LockExclusive(f); err != nil {
			return err
		}
		defer fslock.Unlock(f)
		return fn()
	})
}
func (s Store) readMemory() (memory, error) {
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}, Wakes: []time.Time{}, Pending: []Wake{}}
	b, err := os.ReadFile(filepath.Join(s.Feed.Dir(), ".orchestrator.json"))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	if err == nil && (m.Jobs == nil || m.Delivered == nil) {
		err = errors.New("invalid orchestrator state")
	}
	return m, err
}
func (s Store) saveMemory(m memory) error {
	return writeJSON(filepath.Join(s.Feed.Dir(), ".orchestrator.json"), m)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, b) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicfile.Write(path, b, 0o600)
}
func (s Store) control(j *contract.Job, status contract.Status, reason, re string) error {
	body := map[string]any{"target": j.ID, "reason": reason}
	if status != "" {
		body["status"] = status
	}
	if status == contract.Rejected || status == contract.Cancelled {
		body["close_reason"] = reason
	}
	_, err := s.Feed.Post(feed.Input{Type: "control", Body: body, Re: re}, s.Writer)
	return err
}

// Next polls feed/ and lease/ through a single pass every second. This is a
// portable file watch; the unconditional rescan is more frequent than 30 s.
func (s Store) Next(ctx context.Context, wait bool) (Output, error) {
	for {
		out, err := s.Tick()
		if err != nil || !wait || out.Assigned > 0 || out.Wake != nil {
			return out, err
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return out, ctx.Err()
		case <-timer.C:
		}
	}
}
