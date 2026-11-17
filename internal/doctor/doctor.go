// Package doctor checks that this machine can run grid.
package doctor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/migrate"
)

// Check is one fact: its name, whether it passes, and one line of detail.
// A skipped check has OK true and a detail beginning with "skipped:".
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Report is every check; OK is true when all hold.
type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// Options sets what to check. Label is how the Grid home is named in messages
// (for example ~/.grid); GitVersion defaults to running git --version.
type Options struct {
	Home         string // user home for the same permission anchors as setup
	SettingsPath string // Claude settings file; supplied by the caller, never guessed
	GridHome     string
	Projects     string // absolute GRID_PROJECTS as setup resolves it; empty selects <GridHome>/projects
	Label        string
	GitVersion   func() (string, error)
	HooksFile    string
	LookPath     func(string) (string, error)
	// GridVersion answers the version of the grid at path; nil means the version
	// is not read, which never fails a check on its own.
	GridVersion func(path string) (string, error)
	// Resolved is how the Grid home was found. A zero Source adds no source row.
	Resolved gridhome.Home
}

// Run runs the checks. The home checks that depend on the folder are skipped
// when the folder is missing.
func Run(o Options) Report {
	label := o.Label
	if label == "" {
		label = o.GridHome
	}
	gitVersion := o.GitVersion
	if gitVersion == nil {
		gitVersion = gitx.Version
	}
	rep := Report{OK: true, Checks: []Check{}}
	add := func(name string, ok bool, detail string) {
		rep.Checks = append(rep.Checks, Check{Name: name, OK: ok, Detail: detail})
		if !ok {
			rep.OK = false
			rep.Checks[len(rep.Checks)-1].Fix = fixHint(name, detail)
		}
	}

	homeAvailable := false
	info, err := os.Stat(o.GridHome)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		add("grid home", false, label+" does not exist")
	case err != nil:
		add("grid home", false, label+" cannot be read")
	case !info.IsDir():
		add("grid home", false, label+" is not a folder")
	default:
		homeAvailable = true
		add("grid home", true, label)
		if writable(o.GridHome) {
			add("grid home writable", true, label)
		} else {
			add("grid home writable", false, "cannot write in "+label)
		}
		checkFormat(add, o.GridHome, label)
		checkMachine(add, o.GridHome, label)
	}
	checkHomeSource(add, o)

	// The journal sits beside the home, so this row shows when the home is absent too:
	// between migrate's two renames the home does not exist, and grid migrate puts it back.
	if migrate.Pending(o.GridHome) {
		add("migration", false, "a migration of "+label+" stopped part way")
	} else {
		add("migration", true, "no interrupted migration")
	}

	if v, err := gitVersion(); err != nil {
		add("git", false, "git was not found on PATH")
	} else {
		add("git", true, v)
	}
	checkInstallation(add, o)
	if homeAvailable {
		checkAllowBlock(add, o)
		checkDenyBlock(add, o)
	} else {
		add("allow block", true, "skipped: Grid home is unavailable")
		add("deny block", true, "skipped: Grid home is unavailable")
	}
	checkSandbox(add, o)
	return rep
}

func checkFormat(add func(string, bool, string), dir, label string) {
	n, err := gridhome.ReadFormat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist) && freshHome(dir):
		// grid setup writes the Claude settings and its record, never VERSION: the
		// first project or task does, so a new home has nothing for the person to fix.
		add("format version", true, "new home: VERSION is written by the first project or task")
	case err != nil:
		add("format version", false, "cannot read "+filepath.ToSlash(filepath.Join(label, "VERSION")))
	case gridhome.FormatProblem(n) != "":
		add("format version", false, gridhome.FormatProblem(n))
	default:
		add("format version", true, strconv.Itoa(n))
	}
}

// freshHome reports a home only the plugin hooks (sessions/) or grid setup (setup/)
// have written to: it holds no project and no personal task, so there is nothing
// for grid migrate to do.
func freshHome(dir string) bool {
	made := false
	for _, name := range []string{"sessions", "setup"} {
		if s, err := os.Stat(filepath.Join(dir, name)); err == nil && s.IsDir() {
			made = true
		}
	}
	if !made {
		return false
	}
	for _, name := range []string{"projects", "personal"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	return true
}

// checkMachine reports the older desktop app's folder where the machine id file
// belongs. It only reports: the next grid command that adds a project or a task moves
// the folder aside (gridhome.SetAsideOldMachine), so the row shows only while it is there.
func checkMachine(add func(string, bool, string), dir, label string) {
	if gridhome.OldAppMachine(dir) {
		add("machine id", false, filepath.ToSlash(filepath.Join(label, "machine"))+" is a folder from the older Grid app, not the machine id file")
	}
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	return os.Remove(name) == nil
}
