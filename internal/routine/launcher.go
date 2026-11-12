package routine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// The plugin installs into a versioned folder and every update moves the binary,
// so a timer that names that path stops starting. Timers name this launcher in
// the Grid home instead, and every grid run points it at the binary that runs.

// blLaunchLink makes a hard link on Windows. It is a variable so a test can make
// the link fail and reach the copy that follows a failed link.
var blLaunchLink = os.Link

// LauncherPath is the file routine timers start: bin/grid in the Grid home, or
// bin/grid.exe on Windows.
func LauncherPath(gridHome string) string { return blLaunchPath(gridHome, runtime.GOOS) }

func blLaunchPath(gridHome, goos string) string {
	name := "grid"
	if goos == "windows" {
		name = "grid.exe"
	}
	return filepath.Join(gridHome, "bin", name)
}

// RefreshLauncher makes LauncherPath(gridHome) run exe and says whether it wrote
// anything. On Unix the launcher is a symlink to exe, replaced through a temporary
// link in the same folder and a rename, and left alone when it already points at
// exe. On Windows it is a hard link to exe, or a copy when the link fails, and is
// replaced only when its size or modification time differ from exe. A launcher
// that is the running binary itself, which is how a timer starts it, is left alone.
// The Grid home must already exist: this never creates it.
func RefreshLauncher(gridHome, exe string) (changed bool, err error) {
	if !filepath.IsAbs(gridHome) || !filepath.IsAbs(exe) {
		return false, errors.New("launcher: Grid home and executable must be absolute paths")
	}
	if info, err := os.Stat(gridHome); err != nil || !info.IsDir() {
		return false, fmt.Errorf("launcher: Grid home is not a folder: %s", gridHome)
	}
	// A timer starts the launcher, and some systems then report the launcher's own
	// path as the executable; follow it to the binary it stands for.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	path := LauncherPath(gridHome)
	if filepath.Clean(exe) == path {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		return blLaunchFile(path, exe)
	}
	return blLaunchSymlink(path, exe)
}

// blLaunchTemp is a name in the launcher's folder that no other grid process uses.
func blLaunchTemp(path string) string { return fmt.Sprintf("%s.%d.tmp", path, os.Getpid()) }

// blLaunchSymlink points path at exe. Readlink, not Stat: a link whose target an
// update removed is dangling and must still be read and replaced.
func blLaunchSymlink(path, exe string) (bool, error) {
	if current, err := os.Readlink(path); err == nil && current == exe {
		return false, nil
	}
	tmp := blLaunchTemp(path)
	_ = os.Remove(tmp)
	if err := os.Symlink(exe, tmp); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

// blLaunchFile makes path a hard link to exe, or a copy of it. It is not tied to
// Windows so a test on any host can run it.
func blLaunchFile(path, exe string) (bool, error) {
	want, err := os.Stat(exe)
	if err != nil {
		return false, err
	}
	if have, err := os.Stat(path); err == nil && have.Size() == want.Size() && have.ModTime().Equal(want.ModTime()) {
		return false, nil
	}
	tmp := blLaunchTemp(path)
	_ = os.Remove(tmp)
	if err := blLaunchLink(exe, tmp); err != nil {
		if err := blLaunchCopy(exe, tmp, want); err != nil {
			_ = os.Remove(tmp)
			return false, err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

// blLaunchCopy copies exe to dst with its modification time, so the next refresh
// sees a launcher that matches.
func blLaunchCopy(exe, dst string, info os.FileInfo) error {
	in, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
