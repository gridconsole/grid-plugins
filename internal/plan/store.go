package plan

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// ErrNoPlan reports a task folder with no plan.md.
var ErrNoPlan = errors.New("plan.md is missing")

// Store owns only plan.md, plan-revisions/<n>.md and the persistent .plan.lock.
// TaskDir is an existing task folder selected by internal/where or the caller.
type Store struct{ TaskDir string }

func (s Store) directory() (string, error) {
	if s.TaskDir == "" {
		return "", errors.New("task folder is required; use --task")
	}
	// Resolve the task root, including macOS's symlinked temporary directories.
	dir, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("task path is not a directory")
	}
	return dir, nil
}

func childDirectory(dir, name string, create bool) (string, error) {
	path := filepath.Join(dir, name)
	if create {
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s must be a real directory", name)
	}
	return path, nil
}

func regular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", filepath.Base(path))
	}
	return nil
}

func readRegular(path string) ([]byte, error) {
	if err := regular(path); err != nil {
		return nil, err
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("plan storage must contain regular files")
	}
	return io.ReadAll(f)
}

func locked(dir string, fn func() error) error {
	path := filepath.Join(dir, ".plan.lock")
	if err := regular(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("plan lock must be a regular file")
	}
	if err := fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	return fn()
}

// atomicWrite places and syncs a complete temporary file beside its destination
// before renaming it. Cooperative writers hold .plan.lock for the entire write.
func atomicWrite(path string, data []byte) error {
	if err := regular(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".plan-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Read reads a single atomic snapshot of the current plan without creating files.
func (s Store) Read() (string, error) {
	dir, err := s.directory()
	if err != nil {
		return "", err
	}
	b, err := readRegular(filepath.Join(dir, "plan.md"))
	return string(b), err
}

// WithHash calls fn with the hash of the current plan.md while holding
// .plan.lock, serializing cooperative plan writers with what fn records.
// A caller that also holds another lock (a task's .task.lock) takes it first.
func (s Store) WithHash(fn func(hash string) error) error {
	return s.WithDigest(func(hash, _ string) error { return fn(hash) })
}

// WithDigest calls fn with the short hash and full digest of one plan snapshot
// under .plan.lock, using the same lock order and missing-plan rules as WithHash.
func (s Store) WithDigest(fn func(hash, digest string) error) error {
	dir, err := s.directory()
	if err != nil {
		return err
	}
	return locked(dir, func() error {
		b, err := readRegular(filepath.Join(dir, "plan.md"))
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoPlan
		}
		if err != nil {
			return err
		}
		digest := Digest(string(b))
		return fn(Hash(string(b)), digest)
	})
}

// Items projects the current plan and optional legacy state files without writes.
func (s Store) Items() ([]Item, error) {
	text, err := s.Read()
	if err != nil {
		return nil, err
	}
	items, err := Items(text)
	if err != nil {
		return nil, err
	}
	dir, err := s.directory()
	if err != nil {
		return nil, err
	}
	blocks, err := readRegular(filepath.Join(dir, "items.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	briefDir, err := childDirectory(dir, "subagents", false)
	if errors.Is(err, os.ErrNotExist) {
		return WithLegacyState(items, string(blocks), nil), nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(briefDir)
	if err != nil {
		return nil, err
	}
	var briefs []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		b, err := readRegular(filepath.Join(briefDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		briefs = append(briefs, string(b))
	}
	return WithLegacyState(items, string(blocks), briefs), nil
}
