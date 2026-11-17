package machinefolder

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Where Grid's per-machine files go. A Folder that is Active sends them to the
// machine folder; any other Folder, including the zero one, leaves each file where
// Grid has always put it, with the same path, the same modes and the same calls.
// The functions below hold both paths, so no caller joins them itself.

// Active reports that the person asked for a machine folder: GRID_MACHINE_DIR or
// machine_folder named a path, or shared_group is set. The OS standard place in
// private mode is not active, and neither is a folder whose setting could not be
// used (Folder.Problem): both leave every file where it was and grid doctor
// shows the problem.
func (f Folder) Active() bool {
	return f.Path != "" && (f.Source != SourceDefault || f.Shared())
}

// LogPath is the log of the program name that grid open starts for login:
// <grid home>/logs/<name>.log, and launcher/<login>/<name>.log when Active. Each
// login has its own folder under launcher/, so two logins never share a log.
func (f Folder) LogPath(gridHome, login, name string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	if !f.Active() {
		return filepath.Join(gridHome, "logs", name+".log"), nil
	}
	l, err := f.Launcher(login)
	if err != nil {
		return "", err
	}
	return filepath.Join(l.Dir(), name+".log"), nil
}

// OpenLog opens a log path from LogPath for appending, making it when missing.
// When not Active it makes the folder 0700 and the file 0600 as grid open always
// has; when Active it uses OpenCreate, so a shared folder gets 2770 and 0660.
func (f Folder) OpenLog(path string) (*os.File, error) {
	if !f.Active() {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	}
	return f.OpenCreate(path, os.O_WRONLY|os.O_APPEND)
}

// StartLockPath is the lock that makes two grid open calls start the app one after
// the other: <grid home>/open-start.lock, and launcher/<login>/lock when Active.
// The lock is the login's own, so a login never waits on, or breaks, another's.
func (f Folder) StartLockPath(gridHome, login string) (string, error) {
	if !f.Active() {
		return filepath.Join(gridHome, "open-start.lock"), nil
	}
	l, err := f.Launcher(login)
	if err != nil {
		return "", err
	}
	return l.LockPath(), nil
}

// OpenStartLock opens a path from StartLockPath for locking, making it when missing.
// Not Active it is the open grid open always made: no symlink as the last element,
// 0600. Active the file is made through OpenCreate (2770 folder, 0660 file in a
// shared folder) and then opened the same no-symlink way.
func (f Folder) OpenStartLock(path string) (*os.File, error) {
	if !f.Active() {
		return fslock.OpenNoFollow(path, os.O_RDWR|os.O_CREATE, 0o600)
	}
	made, err := f.OpenCreate(path, os.O_RDWR)
	if err != nil {
		return nil, err
	}
	if err := made.Close(); err != nil {
		return nil, err
	}
	return fslock.OpenNoFollow(path, os.O_RDWR, 0)
}

// RoutineLogDir is the folder the OS timer of a routine writes its stdout and stderr
// to when Active: launcher/<login>/routines/<label>. Each login has its own, so two
// logins that schedule the same routine never share a log. Not Active the log stays in
// <grid home>/logs/routines/<label>, which the routine package still makes as before.
func (f Folder) RoutineLogDir(login, label string) (string, error) {
	if err := validName(label); err != nil {
		return "", err
	}
	l, err := f.Launcher(login)
	if err != nil {
		return "", err
	}
	return filepath.Join(l.Dir(), "routines", label), nil
}

// StateFile is the file a task's fold is published to: <feed folder>/state.json,
// and tasks/<task id>/state.json in the machine folder when Active. A feed with no
// task id has no name to put under tasks/, so it keeps <feed folder>/state.json.
func (f Folder) StateFile(feedDir, taskID string) (string, error) {
	if !f.Active() || taskID == "" {
		return filepath.Join(feedDir, "state.json"), nil
	}
	return f.StatePath(taskID)
}

// WriteStateFile writes the fold at a StateFile path by atomic rename. Outside the
// machine folder (not Active, or a feed with no task id) it is 0600, as before; in it
// it is WriteFile, so a shared folder gets 0660 and a reader under any login sees the
// whole old file or the whole new one.
func (f Folder) WriteStateFile(path string, data []byte) error {
	if !f.Active() || !f.inside(path) {
		return atomicfile.Write(path, data, 0o600)
	}
	return f.WriteFile(path, data)
}

// WorktreeParts is the read side of the worktrees folders: path split into the folder
// names below <machine folder>/worktrees. It holds none when the Folder is not Active
// or path is not below worktrees/, so a caller that asks it leaves every other path
// as it was. The caller passes path and the folder's Path in the same spelling
// (both resolved through symlinks).
func (f Folder) WorktreeParts(path string) ([]string, bool) {
	if !f.Active() {
		return nil, false
	}
	root, err := f.Worktrees()
	if err != nil {
		return nil, false
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, false
	}
	return strings.Split(rel, string(filepath.Separator)), true
}

// WorktreesDir holds the git worktrees of one project's Jobs:
// <grid home>/projects/<project>/worktrees, and worktrees/<project> in the machine
// folder when Active.
func (f Folder) WorktreesDir(gridHome, project string) (string, error) {
	if !f.Active() {
		return filepath.Join(gridHome, "projects", project, "worktrees"), nil
	}
	return f.Join("worktrees", project)
}
