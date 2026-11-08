// Package lease owns the advisory, project-wide merge slot. It does not grant
// task or Job leases. Git remains the authority for whether a branch can land.
package lease

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Duration = 15 * time.Minute

var (
	ErrHeld      = errors.New("merge lease held")
	ErrNotHolder = errors.New("merge lease holder or fence does not match")
	ErrExpired   = errors.New("merge lease expired; take a new lease")
)

// Record persists even after release, so fences never reset on a normal release.
type Record struct {
	Lease  string    `json:"lease"`
	Holder *string   `json:"holder"`
	Fence  uint64    `json:"fence"`
	Since  time.Time `json:"since"`
	Until  time.Time `json:"until"`
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
	Now        func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func validHolder(holder string) bool {
	return strings.TrimSpace(holder) == holder && holder != "" && !strings.ContainsAny(holder, "\r\n\x00\t")
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
		rec := Record{Lease: "merge", Holder: &holder, Fence: fence, Since: now, Until: now.Add(Duration)}
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
	if err := flock(f, syscall.LOCK_SH); err != nil {
		return Status{}, err
	}
	defer unlock(f)
	rec, err := readRecord(f)
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
	if err := flock(f, syscall.LOCK_EX); err != nil {
		return Record{}, err
	}
	defer unlock(f)
	rec, err := readRecord(f)
	if err != nil {
		return Record{}, err
	}
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
	return os.OpenFile(filepath.Join(dir, "merge.json"), flags|syscall.O_NOFOLLOW, 0o600)
}
func readRecord(f *os.File) (Record, error) {
	b, err := io.ReadAll(f)
	if err != nil {
		return Record{}, err
	}
	if len(b) == 0 {
		return Record{Lease: "merge"}, nil
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return Record{}, fmt.Errorf("read merge lease: %w", err)
	}
	if rec.Lease != "merge" || rec.Fence == 0 || (rec.Holder != nil && (!validHolder(*rec.Holder) || rec.Since.IsZero() || !rec.Until.After(rec.Since))) {
		return Record{}, errors.New("invalid merge lease record")
	}
	return rec, nil
}
func writeRecord(f *os.File, rec Record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return rewrite(f, append(b, '\n'))
}

// Never unlink or rename a locked inode, or truncate it before writing. A torn
// nonempty record fails closed; an empty new file has no grant to protect.
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
	f, err := os.OpenFile(filepath.Join(s.ProjectDir, "lease", "seq"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := flock(f, syscall.LOCK_EX); err != nil {
		return 0, err
	}
	defer unlock(f)
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
	if n < previous {
		return 0, errors.New("lease fence counter moved backwards")
	}
	if n == ^uint64(0) {
		return 0, errors.New("lease fence counter exhausted")
	}
	n++
	return n, rewrite(f, []byte(strconv.FormatUint(n, 10)+"\n"))
}
func flock(f *os.File, mode int) error {
	for {
		err := syscall.Flock(int(f.Fd()), mode)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
func unlock(f *os.File) { _ = flock(f, syscall.LOCK_UN) }
