// Package gridhome finds the Grid data folder and reads its format version.
package gridhome

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// FormatVersion is the layout version of the data folder this grid reads and writes.
const FormatVersion = 1

// Source says where a resolved value came from.
type Source string

const (
	SourceFlag    Source = "flag"    // a --grid-home flag
	SourceEnv     Source = "env"     // GRID_HOME
	SourceConfig  Source = "config"  // the shared config (gridconfig.Path, or the older ~/.grid/config.json)
	SourceLegacy  Source = "legacy"  // the v1 top-level keys of that file, migrated in memory
	SourceDefault Source = "default" // ~/.grid itself, or the OS standard machine folder
	SourceAccount Source = "account" // oauthAccount displayName of the Claude account
	SourceLogin   Source = "login"   // the OS login name
)

// Home is the resolved Grid home and what the shared config says about it.
type Home struct {
	// Dir is the data folder: tasks, projects, accounts, usage, settings, and the
	// brain unless Brain names another folder. "" when no GRID_HOME is set and the
	// user's home folder is unknown.
	Dir    string
	Source Source
	// DataRoot is <userHome>/.grid: the default data folder, the parent of
	// ~/.grid/<name>, and where the older config and launcher sit.
	DataRoot string
	// Machine is the machine folder (locks, caches, worktrees, runner logs,
	// bin/grid) as machinefolder.Resolve finds it: its environment override
	// (MachineSource env), the config's machineFolder (config) or the OS standard
	// place (default).
	// It is set by default, so a caller never decides to use the machine folder by
	// whether it is set: machinefolder.Folder.Active says that.
	Machine       string
	MachineSource Source
	// Config is the config file read, "" when there was none. ConfigCopiedFrom is
	// the older ~/.grid/config.json when this resolve copied it to Config.
	Config           string
	ConfigCopiedFrom string
	// ConfigProblem says why the config could not be used; the home is then
	// resolved as if there were no config, never refused.
	ConfigProblem string
	// Workspace is the config entry whose data folder is Dir: the current entry,
	// or with GRID_HOME the entry that names that folder. "" when none does.
	Workspace string
	// Brain is that entry's brain folder when the config names one, else "": the
	// brain stays in Dir. It is only read here; the brain decides how to use it.
	Brain string
	// Member is the config's member for Workspace (else the current entry), with
	// MemberSource config or legacy; "" when the config names none. It is not the
	// acting member: GRID_WHO and --member are read by the commands, and
	// DefaultMember gives the name to offer when the config has none.
	Member       string
	MemberSource Source
}

// Resolve is the one resolver of the Grid home: GRID_HOME when set (the
// --grid-home flag arrives here as GRID_HOME, and the CLI marks it flag); else the
// data folder of the current workspace in the shared config; else the v1 shape of
// that file migrated in memory; else <userHome>/.grid. A config that is missing,
// unreadable or names no usable folder falls through to the next step: it is never
// a refusal. The config is gridconfig.Path, else the older ~/.grid/config.json,
// which is copied once to gridconfig.Path.
func Resolve(getenv func(string) string, userHome string) Home {
	return resolve(runtime.GOOS, getenv, userHome)
}

func resolve(goos string, getenv func(string) string, userHome string) Home {
	h := Home{DataRoot: gridconfig.DataRoot(userHome)}
	l := gridconfig.Load(gridconfig.PathFor(goos, getenv, userHome), gridconfig.OldPath(userHome))
	cfg, found := l.Config, l.Found
	h.Config, h.ConfigCopiedFrom = l.Path, l.CopiedFrom
	if l.Err != nil {
		h.ConfigProblem = l.Err.Error()
		cfg = gridconfig.Config{}
	}
	// machinefolder.Resolve is the one resolver of the machine folder (the environment
	// override, then the config's machineFolder, then the OS standard place); Home only
	// carries what it found. Its Source strings are this package's.
	folder := machinefolder.Resolve(machinefolder.Env{Getenv: getenv, Home: userHome, GOOS: goos})
	h.Machine, h.MachineSource = folder.Path, Source(folder.Source)
	dataSource := SourceConfig
	if l.Err == nil && found && cfg.Migrate(userHome, gridconfig.HandleFromLogin(login(getenv))) {
		dataSource = SourceLegacy
	}
	data := ""
	cur := cfg.CurrentEntry()
	if cur != nil {
		data = cur.DataFolder(userHome)
		if data == "" && h.ConfigProblem == "" {
			h.ConfigProblem = fmt.Sprintf("workspace %q names no usable data folder", cur.Name)
		}
	} else if found && l.Err == nil && cfg.Current != "" {
		h.ConfigProblem = fmt.Sprintf("current workspace %q is not in the workspaces list", cfg.Current)
	}
	switch {
	case getenv("GRID_HOME") != "":
		h.Dir, h.Source = filepath.Clean(getenv("GRID_HOME")), SourceEnv
	case data != "":
		h.Dir, h.Source = data, dataSource
	case h.DataRoot != "":
		h.Dir, h.Source = h.DataRoot, SourceDefault
	}
	in := cur
	if h.Source == SourceEnv {
		in = entryAt(cfg, h.Dir, userHome)
	}
	if in != nil {
		h.Workspace, h.Brain = in.Name, in.BrainFolder()
	}
	named := in
	if named == nil || named.Member == "" {
		named = cur
	}
	if named != nil && named.Member != "" {
		h.Member, h.MemberSource = named.Member, dataSource
	}
	return h
}

// Dir is Resolve(getenv, userHome).Dir, kept for this package's own tests; callers
// use Resolve.
func Dir(getenv func(string) string, userHome string) string {
	return Resolve(getenv, userHome).Dir
}

// entryAt is the workspace whose data folder is dir, or nil.
func entryAt(cfg gridconfig.Config, dir, userHome string) *gridconfig.Workspace {
	want := Real(dir)
	for i := range cfg.Workspaces {
		if d := cfg.Workspaces[i].DataFolder(userHome); d != "" && Real(d) == want {
			return &cfg.Workspaces[i]
		}
	}
	return nil
}

// login is the OS login name from the environment: USER, else USERNAME, else LOGNAME.
func login(getenv func(string) string) string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// LauncherDirs are the folders whose bin/grid each grid run points at itself: Dir,
// as before (timers made earlier start it); and when Dir is this person's own home
// (~/.grid, or the data folder of a config workspace) also the machine folder,
// which the caller makes when it is missing, and ~/.grid when its bin/grid is
// already there. A GRID_HOME no config names (a test or a second home) gets only
// its own launcher, so the machine folder is not pointed at its binary.
func (h Home) LauncherDirs() []string {
	if h.Dir == "" {
		return nil
	}
	dirs := []string{h.Dir}
	own := h.Workspace != "" || (h.DataRoot != "" && Real(h.Dir) == Real(h.DataRoot))
	if !own {
		return dirs
	}
	if h.Machine != "" && Real(h.Machine) != Real(h.Dir) {
		dirs = append(dirs, h.Machine)
	}
	if h.DataRoot != "" && Real(h.DataRoot) != Real(h.Dir) {
		if _, err := os.Lstat(launcherIn(h.DataRoot)); err == nil {
			dirs = append(dirs, h.DataRoot)
		}
	}
	return dirs
}

// FindLauncherDir is the folder to look for bin/grid in: the machine folder when
// its bin/grid is there (a file, or a link to one), else ~/.grid when its bin/grid
// is there (the older place), else the data folder.
func FindLauncherDir(getenv func(string) string, userHome, data string) string {
	h := Resolve(getenv, userHome)
	for _, dir := range []string{h.Machine, h.DataRoot} {
		if dir == "" {
			continue
		}
		if st, err := os.Stat(launcherIn(dir)); err == nil && !st.IsDir() {
			return dir
		}
	}
	return data
}

// launcherIn is <dir>/bin/grid, or grid.exe on Windows.
func launcherIn(dir string) string {
	name := "grid"
	if runtime.GOOS == "windows" {
		name = "grid.exe"
	}
	return filepath.Join(dir, "bin", name)
}

// DefaultMember is the name in Grid to offer when the config names none: the
// oauthAccount displayName of the Claude account doing setup (the .claude.json in
// CLAUDE_CONFIG_DIR when set, else ~/.claude.json), else the OS login name. The
// name is returned as written, not made a handle. It reads; it never writes, and it
// is kept out of Resolve because .claude.json can be large and hooks resolve often.
func DefaultMember(getenv func(string) string, userHome string) (string, Source) {
	path := ""
	if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		path = filepath.Join(dir, ".claude.json")
	} else if userHome != "" {
		path = filepath.Join(userHome, ".claude.json")
	}
	if b, err := os.ReadFile(path); path != "" && err == nil {
		var d struct {
			OAuthAccount struct {
				DisplayName string `json:"displayName"`
			} `json:"oauthAccount"`
		}
		if json.Unmarshal(b, &d) == nil {
			if name := strings.TrimSpace(d.OAuthAccount.DisplayName); name != "" {
				return name, SourceAccount
			}
		}
	}
	if name := login(getenv); name != "" {
		return name, SourceLogin
	}
	return "", ""
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
	if OldAppMachine(dir) {
		return "", fmt.Errorf("%s is a folder the older Grid app left, not the machine id file; the next grid command that adds a project or a task moves it to %s and makes the id", path, oldAppFolder)
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
