// Package safefile shares bounded reads and atomic writes below an os.Root.
package safefile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Reject symlink aliases as well as escapes: a home knowledge link into a
// project would otherwise mix the private and shared namespaces.
func NoSymlinks(root *os.Root, path string) error {
	if !filepath.IsLocal(path) {
		return fmt.Errorf("brain path must stay within its level")
	}
	prefix := ""
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)
		info, err := root.Lstat(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("brain paths cannot be symbolic links")
		}
	}
	return nil
}

func Read(root *os.Root, path string) ([]byte, error) {
	if err := NoSymlinks(root, path); err != nil {
		return nil, err
	}
	info, err := root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("brain files must be regular files of at most 1 MiB")
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("brain files must be regular files of at most 1 MiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err == nil && len(b) > 1<<20 {
		err = fmt.Errorf("brain file exceeds 1 MiB")
	}
	return b, err
}

func AtomicWrite(root *os.Root, path string, data []byte) error {
	if err := NoSymlinks(root, path); err != nil {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), ".brain-"+id+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(tmp, path)
}

func randomID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
