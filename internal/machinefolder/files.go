package machinefolder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// The modes of Grid's own paths in the machine folder. Shared mode sets them with
// chmod after the create, never through the umask, so a login's umask cannot
// make a path unreadable to the group.
const (
	SharedDirMode   = os.ModeSetgid | 0o770
	SharedFileMode  = 0o660
	PrivateDirMode  = 0o700
	PrivateFileMode = 0o600
)

// DirMode is the mode of a directory Grid creates in the folder.
func (f Folder) DirMode() os.FileMode {
	if f.Shared() {
		return SharedDirMode
	}
	return PrivateDirMode
}

// FileMode is the mode of a file Grid creates in the folder.
func (f Folder) FileMode() os.FileMode {
	if f.Shared() {
		return SharedFileMode
	}
	return PrivateFileMode
}

// ErrOutside is returned for a path that is not inside the machine folder. Grid
// creates and chmods only inside it.
var ErrOutside = errors.New("path is outside the machine folder")

// Rel returns path relative to the folder, or ErrOutside when path is the folder's
// parent, a sibling, or anywhere else outside it. The folder itself is ".".
func (f Folder) Rel(path string) (string, error) {
	if f.Path == "" {
		return "", errors.New("the machine folder is not known")
	}
	rel, err := filepath.Rel(f.Path, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %s", ErrOutside, path)
	}
	return rel, nil
}

// Join returns the path of parts under the folder; each part is one plain name.
func (f Folder) Join(parts ...string) (string, error) {
	if f.Path == "" {
		return "", errors.New("the machine folder is not known")
	}
	for _, p := range parts {
		if err := validName(p); err != nil {
			return "", err
		}
	}
	return filepath.Join(append([]string{f.Path}, parts...)...), nil
}

// validName refuses what is not one plain file name: empty, a dot name, or one
// holding a separator or a NUL.
func validName(s string) error {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00") {
		return fmt.Errorf("%q is not a plain name", s)
	}
	return nil
}

func (f Folder) inside(path string) bool {
	_, err := f.Rel(path)
	return err == nil
}

// MkdirAll makes dir, which must be inside the folder, and the missing folders
// above it down from the machine folder. Each directory it creates there gets the
// folder's mode by chmod. A directory that already exists is left as it is: it is
// not Grid's to change, and another login may own it. Folders above the machine
// folder are created plain and never chmod.
func (f Folder) MkdirAll(dir string) error {
	dir = filepath.Clean(dir)
	if _, err := f.Rel(dir); err != nil {
		return err
	}
	var missing []string
	for p := dir; ; {
		st, err := os.Stat(p)
		if err == nil {
			if !st.IsDir() {
				return fmt.Errorf("%s is not a folder", p)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		missing = append(missing, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	for i := len(missing) - 1; i >= 0; i-- {
		p := missing[i]
		if err := os.Mkdir(p, 0o700); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue // made by another process or login since the look above
			}
			return err
		}
		if f.inside(p) {
			if err := os.Chmod(p, f.DirMode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteFile writes data to path, inside the folder, by atomic rename of a temp
// file in the same folder: a reader under another login sees the whole old file
// or the whole new one. A new file gets the folder's file mode by chmod; a file
// that exists keeps its mode.
func (f Folder) WriteFile(path string, data []byte) error {
	path = filepath.Clean(path)
	if _, err := f.Rel(path); err != nil {
		return err
	}
	if err := f.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	return atomicfile.Write(path, data, f.FileMode())
}

// OpenCreate opens path (inside the folder) with flag for writing, making it first
// when it is missing and then setting the folder's file mode on the file it made.
// A file that exists is opened as it is, with no chmod.
func (f Folder) OpenCreate(path string, flag int) (*os.File, error) {
	path = filepath.Clean(path)
	if _, err := f.Rel(path); err != nil {
		return nil, err
	}
	if err := f.MkdirAll(filepath.Dir(path)); err != nil {
		return nil, err
	}
	flag &^= os.O_CREATE | os.O_EXCL
	for {
		file, err := os.OpenFile(path, flag|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if err := file.Chmod(f.FileMode()); err != nil {
				file.Close()
				return nil, err
			}
			return file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		file, err = os.OpenFile(path, flag, 0)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed between the two opens
		}
		return file, err
	}
}

// Layout: the parts of the folder.

// LauncherRoot holds one folder per login; see Launcher.
func (f Folder) LauncherRoot() (string, error) { return f.Join("launcher") }

// BrainCache holds the index and backlinks, rebuilt from the brain.
func (f Folder) BrainCache() (string, error) { return f.Join("brain-cache") }

// Worktrees holds the git checkouts per Job.
func (f Folder) Worktrees() (string, error) { return f.Join("worktrees") }

// Videos holds the videos outcome.md links to.
func (f Folder) Videos() (string, error) { return f.Join("videos") }

// StatePath is tasks/<task>/state.json, the cache of a task's board posts.
func (f Folder) StatePath(task string) (string, error) {
	return f.Join("tasks", task, "state.json")
}

// WriteState writes a task's state.json by atomic rename. Readers under any login
// see a whole file.
func (f Folder) WriteState(task string, data []byte) error {
	p, err := f.StatePath(task)
	if err != nil {
		return err
	}
	return f.WriteFile(p, data)
}

// ReadState reads a task's state.json.
func (f Folder) ReadState(task string) ([]byte, error) {
	p, err := f.StatePath(task)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}
