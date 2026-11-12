//go:build !windows

package routine

import (
	"os"
	"path/filepath"
	"testing"
)

// blLaunchResolvesTo fails unless the launcher resolves to the file want.
func blLaunchResolvesTo(t *testing.T, launcher, want string) {
	t.Helper()
	got, err := filepath.EvalSymlinks(launcher)
	if err != nil {
		t.Fatalf("launcher does not resolve: %v", err)
	}
	wantReal, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantReal {
		t.Fatalf("launcher resolves to %s, want %s", got, wantReal)
	}
}

func TestBlLaunchRefreshFollowsTheExecutable(t *testing.T) {
	home := t.TempDir()
	launcher := LauncherPath(home)
	exeA := blLaunchWriteExe(t, "binary A")

	changed, err := RefreshLauncher(home, exeA)
	if err != nil || !changed {
		t.Fatalf("first call: changed %v, err %v", changed, err)
	}
	blLaunchResolvesTo(t, launcher, exeA)
	first, err := os.Readlink(launcher)
	if err != nil {
		t.Fatalf("launcher is not a symlink: %v", err)
	}

	changed, err = RefreshLauncher(home, exeA)
	if err != nil || changed {
		t.Fatalf("second call: changed %v, err %v", changed, err)
	}
	if second, _ := os.Readlink(launcher); second != first {
		t.Fatalf("link changed from %s to %s", first, second)
	}

	// An update: A is gone, so the launcher dangles, and B is the binary now running.
	if err := os.Remove(exeA); err != nil {
		t.Fatal(err)
	}
	exeB := blLaunchWriteExe(t, "binary B")
	changed, err = RefreshLauncher(home, exeB)
	if err != nil || !changed {
		t.Fatalf("call with B: changed %v, err %v", changed, err)
	}
	blLaunchResolvesTo(t, launcher, exeB)

	entries, err := os.ReadDir(filepath.Dir(launcher))
	if err != nil || len(entries) != 1 {
		t.Fatalf("bin folder holds %v (err %v), want the launcher only", entries, err)
	}
}

// A timer starts the launcher, and macOS may then report the launcher as the
// executable: that run must not point the launcher at itself.
func TestBlLaunchRefreshLeavesALauncherThatIsTheExecutable(t *testing.T) {
	home := t.TempDir()
	launcher := LauncherPath(home)
	exe := blLaunchWriteExe(t, "binary")
	if _, err := RefreshLauncher(home, exe); err != nil {
		t.Fatal(err)
	}
	want, _ := os.Readlink(launcher)
	changed, err := RefreshLauncher(home, launcher)
	if err != nil || changed {
		t.Fatalf("launcher as exe: changed %v, err %v", changed, err)
	}
	if got, _ := os.Readlink(launcher); got != want {
		t.Fatalf("launcher now points at %s, was %s", got, want)
	}
	// Dangling, so it cannot be resolved: still left alone.
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if changed, err = RefreshLauncher(home, launcher); err != nil || changed {
		t.Fatalf("dangling launcher as exe: changed %v, err %v", changed, err)
	}
	if got, _ := os.Readlink(launcher); got != want {
		t.Fatalf("dangling launcher now points at %s, was %s", got, want)
	}
}

// The Grid home is never made by this: a machine without one has no routines to start.
func TestBlLaunchRefreshNeedsAnExistingGridHomeAndAbsolutePaths(t *testing.T) {
	exe := blLaunchWriteExe(t, "binary")
	missing := filepath.Join(t.TempDir(), "no-grid-home")
	if changed, err := RefreshLauncher(missing, exe); err == nil || changed {
		t.Fatalf("missing Grid home: changed %v, err %v", changed, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the Grid home was made: %v", err)
	}
	home := t.TempDir()
	if changed, err := RefreshLauncher(home, "grid"); err == nil || changed {
		t.Fatalf("relative exe: changed %v, err %v", changed, err)
	}
	if changed, err := RefreshLauncher("relative-home", exe); err == nil || changed {
		t.Fatalf("relative home: changed %v, err %v", changed, err)
	}
	if _, err := os.Lstat(LauncherPath(home)); !os.IsNotExist(err) {
		t.Fatalf("a launcher was made for a refused call: %v", err)
	}
}
