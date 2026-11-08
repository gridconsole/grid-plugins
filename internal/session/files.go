package session

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// realDirectory rejects symlinked components, including sessions/ and .live/.
func realDirectory(dir string, create bool) error {
	if dir == "." || dir == string(filepath.Separator) {
		return nil
	}
	if err := realDirectory(filepath.Dir(dir), create); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) && create {
		if err = os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("session storage path must contain only real directories")
	}
	return nil
}
func readFile(path string) ([]byte, error) {
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
func atomicJSON(path string, value any) error {
	if err := realDirectory(filepath.Dir(path), true); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("session storage file must be regular")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := json.NewEncoder(f).Encode(value); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (s Store) locked(fn func() error) error {
	if s.TaskDir == "" {
		return errors.New("task directory is required")
	}
	dir := filepath.Join(s.TaskDir, "sessions")
	if err := realDirectory(dir, true); err != nil {
		return err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, ".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	return fn()
}
