// Package lease owns the advisory, project-wide merge slot. It does not grant
// task or Job leases. Git remains the authority for whether a branch can land.
package lease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

const Duration = 15 * time.Minute

var (
	ErrHeld      = errors.New("merge lease held")
	ErrNotHolder = errors.New("merge lease holder or fence does not match")
	ErrExpired   = errors.New("merge lease expired; take a new lease")
)

// Record persists even after release, so fences never reset on a normal release.
type Record struct {
	Lease     string     `json:"lease"`
	Project   string     `json:"project"`
	Gen       int64      `json:"gen"`
	RenewedAt *time.Time `json:"renewed_at,omitempty"`
	Holder    *string    `json:"holder"`
	Fence     uint64     `json:"fence"`
	Since     time.Time  `json:"since"`
	Until     time.Time  `json:"until"`
}

type Status struct {
	Record
	Held bool `json:"held"`
}

// HeldError includes the current grant for the CLI's refusal message.
type HeldError struct{ Record Record }

func (e *HeldError) Error() string {
	return fmt.Sprintf("merge lease held by %s since %s", *e.Record.Holder, e.Record.Since.Format(time.RFC3339))
}
func (e *HeldError) Unwrap() error { return ErrHeld }

// Store's directory is a project folder, not a task or repository folder.
// Now is injectable; nil uses the local wall clock. All participants must use
// the same local filesystem and clock. A synced copy is not a distributed lock.
type Store struct {
	ProjectDir string
	Project    string
	Now        func() time.Time
}

func (s Store) project() string {
	if s.Project != "" {
		return s.Project
	}
	return filepath.Base(filepath.Clean(s.ProjectDir))
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func validHolder(holder string) bool {
	return strings.TrimSpace(holder) == holder && holder != "" && !strings.ContainsFunc(holder, unicode.IsControl)
}

// Take grants a new fence when free or expired, including to a previous holder.
// Taking an already-held lease never implicitly renews it.
func (s Store) Take(holder string) (Record, error) {
	if !validHolder(holder) {
		return Record{}, errors.New("a nonempty holder without control characters is required")
	}
	return s.change(func(f *os.File, old Record) (Record, error) {
		now := s.now()
		if old.Holder != nil && now.Before(old.Until) {
			return Record{}, &HeldError{old}
		}
		fence, err := s.nextFence(old.Fence)
		if err != nil {
			return Record{}, err
		}
		rec := Record{Lease: "merge", Project: s.project(), Gen: old.Gen, Holder: &holder, Fence: fence, Since: now, Until: now.Add(Duration)}
		return rec, writeRecord(f, rec)
	})
}

func (s Store) Renew(holder string, fence uint64) (Record, error) {
	return s.owned(holder, fence, false)
}
func (s Store) Release(holder string, fence uint64) (Record, error) {
	return s.owned(holder, fence, true)
}
func (s Store) owned(holder string, fence uint64, release bool) (Record, error) {
	return s.change(func(f *os.File, rec Record) (Record, error) {
		if rec.Holder == nil || *rec.Holder != holder || fence == 0 || rec.Fence != fence {
			return Record{}, ErrNotHolder
		}
		now := s.now()
		if !now.Before(rec.Until) {
			return Record{}, ErrExpired
		}
		if release {
			rec.Holder = nil
		} else {
			rec.RenewedAt = &now
			rec.Until = now.Add(Duration)
		}
		return rec, writeRecord(f, rec)
	})
}

// Status takes a shared lock and never creates directories or files.
func (s Store) Status() (Status, error) {
	f, err := s.open(false)
	if errors.Is(err, os.ErrNotExist) {
		return Status{Record: Record{Lease: "merge"}}, nil
	}
	if err != nil {
		return Status{}, err
	}
	defer f.Close()
	if err := fslock.LockShared(f); err != nil {
		return Status{}, err
	}
	defer fslock.Unlock(f)
	rec, _, _, err := readRecord(f) // a torn tail is ignored here; only a writer repairs it
	if err != nil {
		return Status{}, err
	}
	return Status{Record: rec, Held: rec.Holder != nil && s.now().Before(rec.Until)}, nil
}

func (s Store) change(fn func(*os.File, Record) (Record, error)) (Record, error) {
	f, err := s.open(true)
	if err != nil {
		return Record{}, err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return Record{}, err
	}
	defer fslock.Unlock(f)
	rec, keep, torn, err := readRecord(f)
	if err != nil {
		return Record{}, err
	}
	if torn {
		if err := f.Truncate(keep); err != nil {
			return Record{}, err
		}
		if err := f.Sync(); err != nil {
			return Record{}, err
		}
	}
	rec.Project = s.project()
	return fn(f, rec)
}
func (s Store) open(create bool) (*os.File, error) {
	if s.ProjectDir == "" {
		return nil, errors.New("project directory is required")
	}
	dir := filepath.Join(s.ProjectDir, "lease")
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("lease directory must be a real directory")
	}
	flags := os.O_RDONLY
	if create {
		flags = os.O_RDWR | os.O_CREATE
	}
	return fslock.OpenNoFollow(filepath.Join(dir, "merge.json"), flags, 0o600)
}

// readRecord decodes the first JSON value. rewrite writes the new record at
// offset 0 before it truncates, so a kill in between leaves bytes of the longer
// record it replaced after the new one: keep is where the file should end and
// torn says to cut it there. A file that does not start with a record fails closed.
func readRecord(f *os.File) (rec Record, keep int64, torn bool, err error) {
	b, err := io.ReadAll(f)
	if err != nil {
		return Record{}, 0, false, err
	}
	if len(b) == 0 {
		return Record{Lease: "merge"}, 0, false, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&rec); err != nil {
		return Record{}, 0, false, fmt.Errorf("read merge lease: %w", err)
	}
	end := int(dec.InputOffset())
	if end < len(b) && b[end] == '\n' {
		end++
	}
	torn = len(bytes.TrimSpace(b[end:])) > 0
	if rec.Lease != "merge" || rec.Fence == 0 || (rec.Holder != nil && (!validHolder(*rec.Holder) || rec.Since.IsZero() || !rec.Until.After(rec.Since))) {
		return Record{}, 0, false, errors.New("invalid merge lease record")
	}
	return rec, int64(end), torn, nil
}
func writeRecord(f *os.File, rec Record) error {
	wire := contract.MergeLease{Lease: rec.Lease, Project: rec.Project, Holder: rec.Holder, Fence: int64(rec.Fence), Gen: rec.Gen, Since: contract.FormatTime(rec.Since), Until: contract.FormatTime(rec.Until)}
	if rec.RenewedAt != nil {
		stamp := contract.FormatTime(*rec.RenewedAt)
		wire.RenewedAt = &stamp
	}
	b, err := json.Marshal(wire)
	if err != nil {
		return err
	}
	return rewrite(f, append(b, '\n'))
}

// Never unlink or rename a locked inode, or truncate it before writing. A kill
// before the truncate leaves a torn tail that the next writer cuts off.
func rewrite(f *os.File, b []byte) error {
	if _, err := f.WriteAt(b, 0); err != nil {
		return err
	}
	if err := f.Truncate(int64(len(b))); err != nil {
		return err
	}
	return f.Sync()
}
func (s Store) nextFence(previous uint64) (uint64, error) {
	f, err := fslock.OpenNoFollow(filepath.Join(s.ProjectDir, "lease", "seq"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return 0, err
	}
	defer fslock.Unlock(f)
	b, err := io.ReadAll(f)
	if err != nil {
		return 0, err
	}
	var n uint64
	if len(b) != 0 {
		n, err = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("read lease fence: %w", err)
		}
	}
	// A missing or reset counter must not wedge the lease or reissue a fence.
	n = max(n, previous)
	if n >= math.MaxInt64 {
		return 0, errors.New("lease fence counter exhausted")
	}
	n++
	return n, rewrite(f, []byte(strconv.FormatUint(n, 10)+"\n"))
}
