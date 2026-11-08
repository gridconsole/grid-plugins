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

// Safe checks existing path components below root, refusing symlinks and escapes.
// The caller may choose a symlink-resolved Grid home as root.
func Safe(root string, parts ...string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("no Grid home; set GRID_HOME")
	}
	path := filepath.Join(append([]string{root}, parts...)...)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside the Grid home")
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
			return "", fmt.Errorf("refusing symlink: %s", p)
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

// Machine creates the machine identity with O_EXCL or reads the existing ULID.
func Machine(home string) (string, error) {
	if err := Init(home); err != nil {
		return "", err
	}
	path, err := Safe(home, "machine")
	if err != nil {
		return "", err
	}
	id, err := ID()
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		// Another process can be between its exclusive open and its write.
		for i := 0; i < 50; i++ {
			st, err := os.Lstat(path)
			if err != nil {
				return "", err
			}
			if !st.Mode().IsRegular() || st.Size() > 128 {
				return "", fmt.Errorf("machine is not a regular ULID file")
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			existing := strings.TrimSpace(string(b))
			if existing != "" {
				if !ValidID(existing) {
					return "", fmt.Errorf("machine file does not contain a ULID")
				}
				return existing, nil
			}
			time.Sleep(10 * time.Millisecond)
		}
		return "", fmt.Errorf("machine file is empty")
	}
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintln(f, id)
	err = errors.Join(err, f.Close())
	if err != nil {
		return "", err
	}
	return id, nil
}

// ValidID reports whether s is a canonical ULID.
func ValidID(s string) bool {
	if len(s) != 26 || s[0] > '7' {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			return false
		}
	}
	return true
}

// Lock serializes local writers. A leftover lock fails closed; it is never stolen.
func Lock(home, name string) (func(), error) {
	if err := Init(home); err != nil {
		return nil, err
	}
	path, err := Safe(home, name)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 200; i++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if err := f.Close(); err != nil {
				return nil, err
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, fmt.Errorf("data is locked by %s; if no writer is running, remove the leftover lock", name)
}

// Write replaces a file by rename, so readers see a complete old or new file.
func Write(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
