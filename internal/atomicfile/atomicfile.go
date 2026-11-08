// Package atomicfile replaces a file so a reader, or a crash, sees the whole old
// content or the whole new content, never a part of it.
package atomicfile

import (
	"os"
	"path/filepath"
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
	defer os.Remove(tmp) // a no-op once the rename has moved it
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
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return SyncDir(dir)
}
