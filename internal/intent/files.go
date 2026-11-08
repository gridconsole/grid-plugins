package intent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func (s Store) directory(create bool) (string, error) {
	if s.TaskDir == "" {
		return "", errors.New("task directory is required")
	}
	// Resolve ancestors, including macOS /tmp and /var, but refuse a symlink at
	// intents itself. The task must already exist; intent writes create no task.
	task, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(task, "intents")
	if create {
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("intents must be a real directory")
	}
	return dir, nil
}

func (s Store) locked(create bool, fn func(string) error) error {
	dir, err := s.directory(create)
	if !create && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
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
	return fn(dir)
}

func readJSON(path string, value any) error {
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("intent file must be regular")
	}
	dec := json.NewDecoder(f)
	if err := dec.Decode(value); err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("unexpected data after intent JSON")
	}
	return nil
}

func readClaim(path string, now time.Time) (*Claim, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, errors.New("claim must be a regular file")
	}
	var claim Claim
	err = readJSON(path, &claim)
	if err != nil || !validID(claim.Token) || claim.Until.IsZero() {
		// A process can crash after O_EXCL and before its JSON is synced. Keep that
		// marker exclusive for a full timeout, then allow recovery using its mtime.
		return nil, now.Before(info.ModTime().Add(ClaimTimeout)), nil
	}
	return &claim, now.Before(claim.Until), nil
}

func exclusiveJSON(path string, value any) error {
	f, err := fslock.OpenNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(value); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// Publish a fully synced temporary file with a no-replace hard link. The lock
// serializes normal writers; the link also prevents accidental ID overwrites.
func publish(path string, value any) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".intent-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := json.NewEncoder(f).Encode(value); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
