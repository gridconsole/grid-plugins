package feed

import (
	"os"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// WithLease holds the checked tenure against takeover for a mechanical pass.
// Callers serialize passes separately; individual posts still check their lease.
func (s Store) WithLease(w Writer, attempt *int, fn func() error) error {
	if w.Lease == "" || w.Fence < 1 {
		return ErrStaleLease
	}
	f, err := s.leaseLock(w, attempt)
	if err != nil {
		return err
	}
	defer f.Close()
	defer fslock.Unlock(f)
	return fn()
}

// Sync flushes a tenure or lease commit using the Feed's platform durability rule.
func Sync(f *os.File) error { return syncFile(f) }
