// Package atomicfile replaces a file so a reader, or a crash, sees the whole old
// content or the whole new content, never a part of it.
package atomicfile

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Write replaces path with data: it writes a temp file in the same folder,
// syncs it, renames it over path and syncs the folder, so the rename survives a
// crash. An existing file keeps its permission bits; a new one gets perm.
func Write(path string, data []byte, perm os.FileMode) error {
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	return replace(f, data, perm, func() error { return os.Rename(tmp, path) }, func() error { return SyncDir(dir) })
}

// WriteRoot is Write confined to root, refusing a symlink or non-regular target.
// The caller checks ancestors for symlinks and holds its destination's lock.
// Opening the parent root pins it across renames; all later filesystem operations
// use that handle. Rename replaces the leaf itself, never its symlink target.
func WriteRoot(root *os.Root, path string, data []byte, perm os.FileMode) error {
	if !filepath.IsLocal(path) {
		return fmt.Errorf("atomic destination must be relative")
	}
	dir, err := root.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	name := filepath.Base(path)
	if st, err := dir.Lstat(name); err == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("atomic destination is not a regular file")
		}
		perm = st.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp := "." + name + "." + rand.Text()
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dir.Remove(tmp)
	return replace(f, data, perm, func() error { return dir.Rename(tmp, name) }, func() error {
		if runtime.GOOS == "windows" {
			return nil // Match SyncDir: NTFS journals the rename.
		}
		d, err := dir.Open(".")
		if err != nil {
			return err
		}
		return errors.Join(d.Sync(), d.Close())
	})
}

// Both entry points use the same write, permission, fsync and commit sequence.
func replace(f *os.File, data []byte, perm os.FileMode, rename, syncDir func() error) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
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
	if err := rename(); err != nil {
		return err
	}
	return syncDir()
}
