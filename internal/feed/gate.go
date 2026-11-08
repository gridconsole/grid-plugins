package feed

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func gateID(r Record) string {
	if r.Type != "question" {
		return ""
	}
	return str(object(r.Body["gate"])["id"])
}

func (s Store) gateLock() (*os.File, error) {
	dir := filepath.Join(s.Dir(), ".cache")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "gate.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
