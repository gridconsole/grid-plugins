package machinefolder

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
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

// StateFile is the file a task's fold is published to: <feed folder>/state.json,
// and tasks/<task id>/state.json in the machine folder when Active.
func (f Folder) StateFile(feedDir, taskID string) (string, error) {
	if !f.Active() {
		return filepath.Join(feedDir, "state.json"), nil
	}
	return f.StatePath(taskID)
}

// WriteStateFile writes the fold at a StateFile path by atomic rename. Not Active
// it is 0600, as before; Active it is WriteFile, so a shared folder gets 0660 and a
// reader under any login sees the whole old file or the whole new one.
func (f Folder) WriteStateFile(path string, data []byte) error {
	if !f.Active() {
		return atomicfile.Write(path, data, 0o600)
	}
	return f.WriteFile(path, data)
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
