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
)

// Check is one fact: its name, whether it holds, and one line of detail.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Report is every check; OK is true when all hold.
type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// Options sets what to check. Label is how the Grid home is named in messages
// (for example ~/.grid); GitVersion defaults to running git --version.
type Options struct {
	GridHome   string
	Label      string
	GitVersion func() (string, error)
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
		}
	}

	info, err := os.Stat(o.GridHome)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		add("grid home", false, label+" does not exist")
	case err != nil:
		add("grid home", false, label+" cannot be read")
	case !info.IsDir():
		add("grid home", false, label+" is not a folder")
	default:
		add("grid home", true, label)
		if writable(o.GridHome) {
			add("grid home writable", true, label)
		} else {
			add("grid home writable", false, "cannot write in "+label)
		}
		checkFormat(add, o.GridHome, label)
	}

	if v, err := gitVersion(); err != nil {
		add("git", false, "git was not found on PATH")
	} else {
		add("git", true, v)
	}
	return rep
}

func checkFormat(add func(string, bool, string), dir, label string) {
	n, err := gridhome.ReadFormat(dir)
	switch {
	case err != nil:
		add("format version", false, "cannot read "+filepath.ToSlash(filepath.Join(label, "VERSION")))
	case gridhome.FormatProblem(n) != "":
		add("format version", false, gridhome.FormatProblem(n))
	default:
		add("format version", true, strconv.Itoa(n))
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
