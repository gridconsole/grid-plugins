//go:build unix

package machinefolder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// Row 3: two logins on one shared folder each take their own start lock. Holding
// one never blocks the other, and both files are 0660 in 2770 folders whatever the umask.
func TestStartLockTwoLoginsShareOneMachineFolder(t *testing.T) {
	withUmask(t, 0o077)
	f := sharedFolder(t)
	machine := f.Path
	var held []*os.File
	for _, login := range []string{"alice", "bob"} {
		path, err := f.StartLockPath(filepath.Join(t.TempDir(), ".grid"), login)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(machine, "launcher", login, "lock"); path != want {
			t.Fatalf("%s: StartLockPath = %q, want %q", login, path, want)
		}
		lock, err := f.OpenStartLock(path)
		if err != nil {
			t.Fatalf("%s: %v", login, err)
		}
		defer lock.Close()
		if err := fslock.TryLockExclusive(lock); err != nil {
			t.Fatalf("%s: could not take its own lock while the other login holds theirs: %v", login, err)
		}
		held = append(held, lock)
		if got := modeOf(t, path); got != machinefolder.SharedFileMode {
			t.Errorf("%s: lock mode %v, want %v", login, got, os.FileMode(machinefolder.SharedFileMode))
		}
		if got := modeOf(t, filepath.Dir(path)); got != machinefolder.SharedDirMode {
			t.Errorf("%s: folder mode %v, want %v", login, got, machinefolder.SharedDirMode)
		}
	}
	// The same login a second time does wait: that is what the lock is for.
	again, err := f.OpenStartLock(filepath.Join(machine, "launcher", "alice", "lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if err := fslock.TryLockExclusive(again); err == nil {
		t.Error("a second grid open of one login took the lock while the first held it")
	}
	for _, l := range held {
		fslock.Unlock(l)
	}
}

// Row 2: with no machine folder the lock is <grid home>/open-start.lock, 0600, as before.
func TestStartLockUnconfiguredIsTheOldFile(t *testing.T) {
	withUmask(t, 0o022)
	grid := filepath.Join(t.TempDir(), ".grid")
	if err := os.MkdirAll(grid, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]machinefolder.Folder{
		"zero folder":     {},
		"default private": {Path: filepath.Join(t.TempDir(), "x"), Source: machinefolder.SourceDefault},
	} {
		path, err := f.StartLockPath(grid, "alice")
		if err != nil || path != grid+"/open-start.lock" {
			t.Fatalf("%s: StartLockPath = %q, %v", name, path, err)
		}
		lock, err := f.OpenStartLock(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		lock.Close()
		if got := modeOf(t, path); got != 0o600 {
			t.Errorf("%s: lock mode %v, want 0600", name, got)
		}
		os.Remove(path)
	}
	if _, err := os.Stat(filepath.Join(grid, "launcher")); err == nil {
		t.Error("an unconfigured start made a launcher folder")
	}
}
