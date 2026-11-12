// Package localdata provides local identities and safe, atomic data file writes.
package localdata

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// ID returns a ULID with a millisecond timestamp and 80 random bits.
func ID() (string, error) {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(time.Now().UnixMilli())<<16)
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	n := new(big.Int).SetBytes(b[:])
	alphabet := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var out [26]byte
	mask := big.NewInt(31)
	for i := 25; i >= 0; i-- {
		out[i] = alphabet[new(big.Int).And(n, mask).Int64()]
		n.Rsh(n, 5)
	}
	return string(out[:]), nil
}

// ErrUnsafePath identifies a rejected symlink or escape, rather than an I/O error.
var ErrUnsafePath = errors.New("unsafe path")

// Safe checks existing path components below root, refusing symlinks and escapes.
// The caller may choose a symlink-resolved Grid home as root.
func Safe(root string, parts ...string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("no Grid home; set GRID_HOME")
	}
	path := filepath.Join(append([]string{root}, parts...)...)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path is outside the Grid home", ErrUnsafePath)
	}
	p := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: refusing symlink: %s", ErrUnsafePath, p)
		}
	}
	return path, nil
}

// Init creates the format marker without replacing any existing version.
func Init(home string) error {
	if home == "" {
		return fmt.Errorf("no Grid home; set GRID_HOME")
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	path, err := Safe(home, "VERSION")
	if err != nil {
		return err
	}
	// A home an older build wrote has tasks but no VERSION; stamping it would
	// hide it from grid migrate, so refuse and leave the home as it is.
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if old, err := gridhome.Legacy(home); err != nil {
			return err
		} else if old {
			return errors.New(gridhome.FormatProblem(0))
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		// An exclusive creator may not have written the marker yet.
		for i := 0; i < 50; i++ {
			st, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("VERSION is not a regular file")
			}
			if st.Size() != 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		n, err := gridhome.ReadFormat(home)
		if err != nil {
			return err
		}
		if problem := gridhome.FormatProblem(n); problem != "" {
			return errors.New(problem)
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d\n", gridhome.FormatVersion)
	return errors.Join(err, f.Close())
}

// Machine returns this computer's id, creating <home>/machine on first use.
// Creation takes its own lock, because project and signal writers hold different
// ones. A file that is empty or torn from a crash is replaced; one that holds
// something else is an error.
func Machine(home string) (string, error) {
	if err := Init(home); err != nil {
		return "", err
	}
	path, err := Safe(home, "machine")
	if err != nil {
		return "", err
	}
	if id, err := gridhome.ReadMachine(home); id != "" || err != nil {
		return id, err
	}
	unlock, err := Lock(home, ".machine.lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	// Another process may have made the id while this one waited.
	if id, err := gridhome.ReadMachine(home); id != "" || err != nil {
		return id, err
	}
	id, err := ID()
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintln(f, id)
	if err = errors.Join(err, f.Sync(), f.Close()); err != nil {
		return "", err
	}
	return id, atomicfile.SyncDir(home)
}

// Lock serializes local writers with an exclusive flock on a file in the Grid
// home. It waits its turn. The kernel drops the lock when the holder exits, so a
// killed writer leaves nothing to clear; the file stays in place, because
// removing it would let two writers lock different inodes.
func Lock(home, name string) (func(), error) {
	if err := Init(home); err != nil {
		return nil, err
	}
	return LockExisting(home, name)
}

// LockExisting takes the same persistent lock in an existing data directory,
// without creating a Grid home or a format marker there.
func LockExisting(home, name string) (func(), error) {
	path, err := Safe(home, name)
	if err != nil {
		return nil, err
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", name, err)
	}
	return func() { fslock.Unlock(f); _ = f.Close() }, nil
}
