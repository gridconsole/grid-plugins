//go:build unix

package machinefolder_test

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// ownGroup is the name of the group the test itself runs in, so a shared folder
// made here is owned by a group the test's login belongs to.
func ownGroup(t *testing.T) string {
	t.Helper()
	g, err := user.LookupGroupId(strconv.Itoa(os.Getgid()))
	if err != nil {
		return "gid" + strconv.Itoa(os.Getgid())
	}
	return g.Name
}

// sharedFolder makes a folder in a temp dir, in the test's own group, 2770, as the
// one-time setup commands leave a real shared folder.
func sharedFolder(t *testing.T) machinefolder.Folder {
	t.Helper()
	root := filepath.Join(t.TempDir(), "grid")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, machinefolder.SharedDirMode); err != nil {
		t.Fatal(err)
	}
	return machinefolder.Folder{Path: root, Source: machinefolder.SourceConfig, SharedGroup: ownGroup(t)}
}

func privateFolder(t *testing.T) machinefolder.Folder {
	t.Helper()
	root := filepath.Join(t.TempDir(), "grid")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return machinefolder.Folder{Path: root, Source: machinefolder.SourceConfig}
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode() & (os.ModePerm | os.ModeSetgid)
}

func withUmask(t *testing.T, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}

// Row 2: in shared mode every directory Grid makes is 2770 and every file 0660,
// whatever the login's umask; in private mode they are 0700 and 0600.
func TestSharedModeDirs2770Files0660(t *testing.T) {
	for _, mask := range []int{0o022, 0o077, 0} {
		withUmask(t, mask)
		f := sharedFolder(t)
		deep := filepath.Join(f.Path, "worktrees", "J-01", "src")
		if err := f.MkdirAll(deep); err != nil {
			t.Fatal(err)
		}
		for _, d := range []string{"worktrees", "worktrees/J-01", "worktrees/J-01/src"} {
			if got := modeOf(t, filepath.Join(f.Path, d)); got != os.ModeSetgid|0o770 {
				t.Errorf("umask %04o: dir %s mode %v, want setgid 770", mask, d, got)
			}
		}
		if err := f.WriteState("T-01", []byte(`{"posts":[]}`)); err != nil {
			t.Fatal(err)
		}
		for _, d := range []string{"tasks", "tasks/T-01"} {
			if got := modeOf(t, filepath.Join(f.Path, d)); got != os.ModeSetgid|0o770 {
				t.Errorf("umask %04o: dir %s mode %v, want setgid 770", mask, d, got)
			}
		}
		sp, _ := f.StatePath("T-01")
		if got := modeOf(t, sp); got != 0o660 {
			t.Errorf("umask %04o: state.json mode %v, want 0660", mask, got)
		}
		l, err := f.Launcher("alice")
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Start(os.Getpid()); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{l.PIDPath(), l.LockPath()} {
			if got := modeOf(t, p); got != 0o660 {
				t.Errorf("umask %04o: %s mode %v, want 0660", mask, filepath.Base(p), got)
			}
		}
		lg, err := l.OpenLog()
		if err != nil {
			t.Fatal(err)
		}
		lg.Close()
		if got := modeOf(t, l.LogPath()); got != 0o660 {
			t.Errorf("umask %04o: log mode %v, want 0660", mask, got)
		}
		if got := modeOf(t, l.Dir()); got != os.ModeSetgid|0o770 {
			t.Errorf("umask %04o: launcher dir mode %v, want setgid 770", mask, got)
		}
		l.Stop()
	}
}

func TestPrivateModeDirs0700Files0600(t *testing.T) {
	f := privateFolder(t)
	if f.Shared() || f.DirMode() != 0o700 || f.FileMode() != 0o600 {
		t.Fatalf("private mode: %v %v", f.DirMode(), f.FileMode())
	}
	if err := f.WriteState("T-01", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"tasks", "tasks/T-01"} {
		if got := modeOf(t, filepath.Join(f.Path, d)); got != 0o700 {
			t.Errorf("dir %s mode %v, want 0700", d, got)
		}
	}
	sp, _ := f.StatePath("T-01")
	if got := modeOf(t, sp); got != 0o600 {
		t.Errorf("state.json mode %v, want 0600", got)
	}
	l, _ := f.Launcher("alice")
	if err := l.Start(1); err != nil {
		t.Fatal(err)
	}
	defer l.Stop()
	for _, p := range []string{l.PIDPath(), l.LockPath()} {
		if got := modeOf(t, p); got != 0o600 {
			t.Errorf("%s mode %v, want 0600", filepath.Base(p), got)
		}
	}
}

// Grid creates and chmods only paths it made, only inside the folder.
func TestNeverChmodsOutsideOrExisting(t *testing.T) {
	f := sharedFolder(t)
	parent := filepath.Dir(f.Path)
	before := modeOf(t, parent)

	// A directory that already exists keeps its mode: it is not Grid's to change.
	mine := filepath.Join(f.Path, "brain-cache")
	if err := os.Mkdir(mine, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := f.MkdirAll(filepath.Join(mine, "index")); err != nil {
		t.Fatal(err)
	}
	// (The parent is setgid, so the OS gives the folder its group bit; the permission
	// bits are the ones Grid must not touch.)
	if got := modeOf(t, mine).Perm(); got != 0o750 {
		t.Errorf("an existing directory changed to %v", got)
	}
	if got := modeOf(t, filepath.Join(mine, "index")); got != os.ModeSetgid|0o770 {
		t.Errorf("a new directory under it is %v, want setgid 770", got)
	}
	// An existing file keeps its mode through a rewrite.
	existing := filepath.Join(f.Path, "tasks-note.txt")
	if err := os.WriteFile(existing, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFile(existing, []byte("y")); err != nil {
		t.Fatal(err)
	}
	if got := modeOf(t, existing); got != 0o640 {
		t.Errorf("an existing file changed to %v", got)
	}

	// A path outside the folder is refused and nothing is made or changed.
	outside := filepath.Join(parent, "sibling")
	for name, err := range map[string]error{
		"MkdirAll parent":  f.MkdirAll(parent),
		"MkdirAll sibling": f.MkdirAll(outside),
		"WriteFile":        f.WriteFile(filepath.Join(parent, "x.txt"), []byte("x")),
		"WriteFile dotdot": f.WriteFile(filepath.Join(f.Path, "..", "x.txt"), []byte("x")),
	} {
		if err == nil {
			t.Errorf("%s: no error for a path outside the folder", name)
		}
	}
	if _, err := os.Stat(outside); err == nil {
		t.Error("a folder was made outside the machine folder")
	}
	if got := modeOf(t, parent); got != before {
		t.Errorf("the folder above the machine folder changed from %v to %v", before, got)
	}
	if _, err := f.Join("tasks", "..", "x"); err == nil {
		t.Error("Join accepted a .. part")
	}
	if _, err := f.Launcher("../bob"); err == nil {
		t.Error("Launcher accepted a login with a separator")
	}
}

// A folder Grid makes above the machine folder (the machine folder itself, when it is
// missing in private mode) is made plain and not chmod to 2770.
func TestMissingRootInPrivateModeIs0700(t *testing.T) {
	root := filepath.Join(t.TempDir(), "a", "b", "grid")
	f := machinefolder.Folder{Path: root}
	if err := f.MkdirAll(filepath.Join(root, "videos")); err != nil {
		t.Fatal(err)
	}
	if got := modeOf(t, filepath.Join(root, "videos")); got != 0o700 {
		t.Errorf("videos mode %v", got)
	}
}
