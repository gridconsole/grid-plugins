package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/version"
	"github.com/gridconsole/grid-plugins/internal/worker"
)

// gridVersionAt asks Options.GridVersion for the version of the grid at path.
// ok is false when the version is not read: no GridVersion, or it failed or
// answered nothing. Not reading a version never fails a check.
func gridVersionAt(o Options, path string) (string, bool) {
	if o.GridVersion == nil {
		return "", false
	}
	v, err := o.GridVersion(path)
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

// checkLauncher compares the Grid home's launcher with this grid. It adds
// nothing when the home has no launcher.
func checkLauncher(add func(string, bool, string), o Options) {
	if o.GridHome == "" {
		return
	}
	name := "grid"
	if runtime.GOOS == "windows" {
		name = "grid.exe"
	}
	launcher := filepath.Join(gridhome.FindLauncherDir(os.Getenv, o.Home, o.GridHome), "bin", name)
	if _, err := os.Lstat(launcher); err != nil {
		return
	}
	target, err := filepath.EvalSymlinks(launcher)
	if err != nil {
		add("launcher", false, "launcher points at a target that cannot be found")
		return
	}
	v, ok := gridVersionAt(o, launcher)
	switch {
	case !ok:
		add("launcher", true, "launcher points at "+target+"; version not read")
	case v != version.Version:
		add("launcher", false, fmt.Sprintf("launcher points at %s, version %s; this grid is %s", target, v, version.Version))
	default:
		add("launcher", true, "launcher points at "+target+"; version "+v)
	}
}

// checkWorker reports the version of the worker that holds worker.lock. It adds
// nothing when no worker runs.
func checkWorker(add func(string, bool, string), o Options) {
	entries, err := worker.Store{GridHome: o.GridHome}.List()
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Presence != "reported" {
			continue
		}
		if warn := version.AppWarning("", "", e.Version); warn != "" {
			add("worker", false, warn)
			return
		}
		if e.Version == "" {
			add("worker", true, "running, version not read")
		} else {
			add("worker", true, "running, version "+e.Version)
		}
		return
	}
}
