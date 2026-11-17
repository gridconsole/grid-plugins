// Package gridhome finds the Grid data folder and reads its format version.
package gridhome

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// FormatVersion is the layout version of the data folder this grid reads and writes.
const FormatVersion = 1

// Source says where a resolved value came from.
type Source string

const (
	SourceFlag    Source = "flag"    // a --grid-home or --member flag
	SourceEnv     Source = "env"     // GRID_HOME, or GRID_WHO, GRID_ACTOR, GRID_MEMBER
	SourceConfig  Source = "config"  // the current entry of ~/.grid/config.json
	SourceLegacy  Source = "legacy"  // the v1 top-level keys of that file, migrated in memory
	SourceDefault Source = "default" // ~/.grid itself
)

// Home is the resolved Grid home and what the shared config says about it.
type Home struct {
	// Dir is the data folder: tasks, projects, the brain, settings. "" when no
	// GRID_HOME is set and the user's home folder is unknown.
	Dir    string
	Source Source
	// Machine is the machine folder <userHome>/.grid: config.json and bin/grid.
	Machine string
	// Config is the config file read, "" when there was none.
	Config string
	// ConfigProblem says why the config could not be used; the home is then
	// resolved as if there were no config, never refused.
	ConfigProblem string
	// Workspace is the current entry's name, "" without a config.
	Workspace string
	// Member is the current entry's member, with MemberSource config or legacy; ""
	// when the config names none. GRID_WHO and the flags are read by the commands.
	Member       string
	MemberSource Source
}

// Resolve is the one resolver of the Grid home: GRID_HOME when set (the
// --grid-home flag arrives here as GRID_HOME, and the CLI marks it flag); else the
// data folder of the current workspace in <userHome>/.grid/config.json; else the
// v1 shape of that file migrated in memory; else <userHome>/.grid. A config that
// is missing, unreadable or names no usable folder falls through to the next step:
// it is never a refusal.
func Resolve(getenv func(string) string, userHome string) Home {
	h := Home{Machine: gridconfig.MachineDir(userHome)}
	path := gridconfig.Path(userHome)
	cfg, found, err := gridconfig.Read(path)
	if found {
		h.Config = path
	}
	if err != nil {
		h.ConfigProblem = err.Error()
	}
	dataSource := SourceConfig
	if err == nil && found {
		login := getenv("USER")
		if login == "" {
			login = getenv("USERNAME")
		}
		if cfg.Migrate(userHome, gridconfig.HandleFromLogin(login)) {
			dataSource = SourceLegacy
		}
	}
	data := ""
	if w := cfg.CurrentEntry(); w != nil {
		h.Workspace = w.Name
		if w.Member != "" {
			h.Member, h.MemberSource = w.Member, dataSource
		}
		data = w.DataFolder(userHome)
		if data == "" && h.ConfigProblem == "" {
			h.ConfigProblem = fmt.Sprintf("workspace %q names no usable data folder", w.Name)
		}
	} else if found && err == nil && cfg.Current != "" {
		h.ConfigProblem = fmt.Sprintf("current workspace %q is not in the workspaces list", cfg.Current)
	}
	switch {
	case getenv("GRID_HOME") != "":
		h.Dir, h.Source = filepath.Clean(getenv("GRID_HOME")), SourceEnv
	case data != "":
		h.Dir, h.Source = data, dataSource
	case h.Machine != "":
		h.Dir, h.Source = h.Machine, SourceDefault
	}
	return h
}

// Dir is Resolve(getenv, userHome).Dir, kept for this package's own tests; callers
// use Resolve.
func Dir(getenv func(string) string, userHome string) string {
	return Resolve(getenv, userHome).Dir
}

// LauncherDir is the folder whose bin/grid timers start and SessionStart puts on
// PATH, for the writer: the machine folder when the data folder came from the
// config or sits in it (~/.grid/<name>), else the data folder, as before.
func (h Home) LauncherDir() string {
	if h.Machine != "" && (h.Source == SourceConfig || h.Source == SourceLegacy || filepath.Dir(h.Dir) == h.Machine) {
		return h.Machine
	}
	return h.Dir
}

// FindLauncherDir is the folder to look for bin/grid in: the machine folder
// <userHome>/.grid when its bin/grid is there (a file, or a link to one), else the
// data folder.
func FindLauncherDir(userHome, data string) string {
	if m := gridconfig.MachineDir(userHome); m != "" {
		name := "grid"
		if runtime.GOOS == "windows" {
			name = "grid.exe"
		}
		if st, err := os.Stat(filepath.Join(m, "bin", name)); err == nil && !st.IsDir() {
			return m
		}
	}
	return data
}

// Real makes path absolute and resolves symlinks in its longest existing prefix, so
// paths compare equal whether or not the folder exists yet.
func Real(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	rest := ""
	for p := path; ; {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// Tilde spells a path under home as ~/.... A path that is under home only through
// a link (home is a link, path was resolved by Real) is spelled the same way: when
// the literal prefix does not match, it tries again with Real(home). A path under
// neither spelling stays as it is.
func Tilde(path, home string) string {
	if home == "" {
		return path
	}
	if s, ok := tildeUnder(path, home); ok {
		return s
	}
	if real := Real(home); real != home {
		if s, ok := tildeUnder(path, real); ok {
			return s
		}
	}
	return path
}

// tildeUnder spells path as ~ or ~/... when it is home or under home, literally.
func tildeUnder(path, home string) (string, bool) {
	if path == home {
		return "~", true
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):], true
	}
	return "", false
}

// Machine reads this computer's id from <dir>/machine, or "" before it is made
// and while the file is empty or torn. A file holding some other id (an older
// grid's) is returned as it is. It is the one reader of that file.
func Machine(dir string) string {
	s, err := machineFile(dir)
	if err != nil || torn(s) {
		return ""
	}
	return s
}

// ReadMachine is Machine for a writer: a missing, empty or torn file (a writer
// stopped before the id was complete) gives "" and no error, and a file that holds
// anything but a ULID is an error, so a person's edit is never replaced.
func ReadMachine(dir string) (string, error) {
	s, err := machineFile(dir)
	switch {
	case err != nil:
		return "", err
	case ValidID(s):
		return s, nil
	case torn(s):
		return "", nil
	}
	return "", fmt.Errorf("machine file does not contain a ULID")
}

// machineFile returns the trimmed content of <dir>/machine, "" when it is missing.
func machineFile(dir string) (string, error) {
	path := filepath.Join(dir, "machine")
	st, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() || st.Size() > 128 {
		return "", fmt.Errorf("machine is not a regular ULID file")
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.Trim(string(b), " \t\r\n\x00"), nil
}

// torn reports content a writer left before the whole id was down: nothing, or
// a short run of ULID characters.
func torn(s string) bool { return len(s) < ulidLen && ulidChars(s) }

const ulidLen = 26

// ValidID reports whether s is a canonical ULID.
func ValidID(s string) bool { return len(s) == ulidLen && s[0] <= '7' && ulidChars(s) }

func ulidChars(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			return false
		}
	}
	return true
}

// FormatProblem says why this grid cannot use data of format n, or "" when it can.
func FormatProblem(n int) string {
	switch {
	case n > FormatVersion:
		return fmt.Sprintf("This data is format %d; this grid reads up to %d. Update grid.", n, FormatVersion)
	case n < FormatVersion:
		return fmt.Sprintf("This data is format %d; this grid reads %d. Run: grid migrate", n, FormatVersion)
	}
	return ""
}

// ReadFormat reads the format version from <dir>/VERSION.
func ReadFormat(dir string) (int, error) {
	b, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("VERSION does not hold a number")
	}
	return n, nil
}

// Personal is the project name, and the top-level folder name, of tasks that
// belong to no project.
const Personal = "personal"

// TasksDir is the folder under home that holds a project's task folders:
// personal/tasks for Personal, else projects/<project>/tasks. It is the one
// owner of that layout.
func TasksDir(home, proj string) string {
	if proj == Personal {
		return filepath.Join(home, Personal, "tasks")
	}
	return filepath.Join(home, "projects", proj, "tasks")
}

// Legacy reports whether home, which has no VERSION, was written by an older
// build: it holds a task folder (<tasks dir>/<task>/task.md). Only a task
// creation makes a task folder, and it writes VERSION in the same call, so a task
// folder without VERSION is an older build's (a hidden staging folder, .new-*,
// is not a task folder). The app writes project-roles.json,
// docs/ and brain/ under projects/ before any plugin write, so those say nothing.
// Callers ask only when VERSION is missing; Legacy does not read it. An old home
// with projects but no task is not caught: it holds no task to rewrite, and its
// first write stamps it (a known edge).
func Legacy(home string) (bool, error) {
	dirs, err := TasksDirs(home)
	if err != nil {
		return false, err
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, e := range entries {
			// A hidden folder is a staging folder (.new-*), never a task.
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			_, err := os.Stat(filepath.Join(dir, e.Name(), "task.md"))
			if err == nil {
				return true, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return false, err
			}
		}
	}
	return false, nil
}

// TasksDirs lists the task folders of Personal and of every project folder
// under home, whether or not they exist yet.
func TasksDirs(home string) ([]string, error) {
	dirs := []string{TasksDir(home, Personal)}
	entries, err := os.ReadDir(filepath.Join(home, "projects"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, TasksDir(home, e.Name()))
		}
	}
	return dirs, nil
}
