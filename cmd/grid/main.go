// Command grid is the Grid command line: every write to task folders, the Feed and the brain.
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

func main() {
	env := cli.OSEnv()
	launchDir := env.GridHome
	if env.Resolved.Dir == env.GridHome {
		// The machine folder (~/.grid) when the data folder is the config's or sits in it.
		launchDir = env.Resolved.LauncherDir()
	}
	blLaunchRefresh(launchDir)
	os.Exit(cli.Run(os.Args[1:], env))
}

// blLaunchRefresh points the launcher in the Grid home at this binary, which is
// the file routine timers start. It does nothing without a Grid home or for a
// binary under the temporary folder, where go run builds. An error is dropped: a
// grid command must not fail or print because the launcher could not be written.
func blLaunchRefresh(gridHome string) {
	if gridHome == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil || blLaunchUnderTemp(exe) {
		return
	}
	_, _ = routine.RefreshLauncher(gridHome, exe)
}

// blLaunchUnderTemp reports whether path lies under os.TempDir, as the folder is
// named and as it resolves (macOS names /var/folders, which is a link).
func blLaunchUnderTemp(path string) bool {
	temp := os.TempDir()
	if blLaunchWithin(path, temp) {
		return true
	}
	realTemp, err := filepath.EvalSymlinks(temp)
	if err != nil {
		return false
	}
	realPath, err := filepath.EvalSymlinks(path)
	return err == nil && blLaunchWithin(realPath, realTemp)
}

func blLaunchWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
