package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

// Spawn is one Job in Assigned at the end of a pass, with the facts a worker
// needs to start it: the Job lease it was granted and the assign control that
// granted it (the id that spawned/<J>.json keeps as "after").
type Spawn struct {
	Job     string
	Attempt int
	To      string
	Fence   int64
	Gen     int64
	Assign  string
	Role    string // the Job's role, from which a stream picks its model and effort
}
type Output struct {
	Rows     []Row   `json:"jobs"`
	Wake     *Wake   `json:"wake,omitempty"`
	Assigned int     `json:"-"`
	Spawns   []Spawn `json:"-"`
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
	// AccountRoom, when set, says whether an account has room and whether the
	// account numbers are known at all; a Job parked on usage_limit is offered
	// again by yaUsageReady only when its reset time has passed or there is room.
	AccountRoom func() (room, known bool)
	Incident    func(string, []string) error
	// Diagnostics receives publication notices; nil discards them.
	Diagnostics io.Writer
	// pause is the wait between two passes of Next; nil means the one-second tick.
	pause func(context.Context) error
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

	PlanPublicationBlocked bool `json:"plan_publication_blocked,omitempty"`
}

func (s Store) locked(fn func() error) error {
	// Check before creating the lock, and again while the pass is serialized.
	return s.Feed.WithLease(s.Writer, nil, func() error { return s.exclusive(fn) })
}

// exclusive serializes fn with a holder pass without asking for a lease: a
// person's unleased write takes the same lock a tick takes, and nothing more.
// The lock file lives in the Feed folder, which a task has only once something
// was posted, so a person's first Job on a fresh task finds it made (mode 0700)
// before the lock is opened. The task folder itself must already exist.
func (s Store) exclusive(fn func() error) error {
	info, err := os.Stat(s.Feed.TaskDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("task is not a folder")
	}
	if err := feed.CheckFilesystem(s.Feed.Dir()); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Feed.Dir(), 0o700); err != nil {
		return err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(s.Feed.Dir(), ".orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	// The wait owns f from here and the Feed's Context ends it: a failed or
	// cancelled lock closes f, so the close is deferred only once the lock is held.
	if err := fslock.LockExclusiveContext(s.Feed.Context, f); err != nil {
		return err
	}
	defer f.Close()
	defer fslock.Unlock(f)
	return fn()
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
	pause := s.pause
	if pause == nil {
		pause = tickPause
	}
	for {
		out, err := s.Tick()
		if err != nil || !wait || out.Assigned > 0 || out.Wake != nil {
			return out, err
		}
		if err := pause(ctx); err != nil {
			return out, err
		}
	}
}

// tickPause is the default wait between two passes of Next: one second, or
// the context's error as soon as it is done.
func tickPause(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
