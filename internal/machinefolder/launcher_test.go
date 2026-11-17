package machinefolder_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func tempFolder(t *testing.T) machinefolder.Folder {
	t.Helper()
	root := filepath.Join(t.TempDir(), "grid")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return machinefolder.Folder{Path: root, Source: machinefolder.SourceConfig}
}

type snapshot struct {
	data  []byte
	mtime time.Time
}

func snap(t *testing.T, path string) snapshot {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot{b, st.ModTime()}
}

// Row 3: two launchers under different logins keep separate pid, lock and log
// files, and login A's stale lock is untouched by B.
func TestLaunchersOfTwoLoginsAreSeparate(t *testing.T) {
	f := tempFolder(t)
	a, err := f.Launcher("alice")
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Launcher("bob")
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{a.PIDPath(), b.PIDPath()}, {a.LockPath(), b.LockPath()}, {a.LogPath(), b.LogPath()}} {
		if pair[0] == pair[1] {
			t.Fatalf("both logins share %s", pair[0])
		}
	}
	if want := filepath.Join(f.Path, "launcher", "alice", "pid"); a.PIDPath() != want {
		t.Fatalf("pid path %s, want %s", a.PIDPath(), want)
	}

	// A is running: B starts anyway, and a second A is refused.
	if err := a.Start(101); err != nil {
		t.Fatal(err)
	}
	if err := b.Start(202); err != nil {
		t.Fatalf("bob blocked by alice's launcher: %v", err)
	}
	a2, _ := f.Launcher("alice")
	if err := a2.Start(303); !errors.Is(err, machinefolder.ErrRunning) {
		t.Fatalf("a second launcher of alice: %v, want ErrRunning", err)
	}
	if got, _ := os.ReadFile(a.PIDPath()); strings.TrimSpace(string(got)) != "101" {
		t.Fatalf("alice's pid file is %q", got)
	}
	if got, _ := os.ReadFile(b.PIDPath()); strings.TrimSpace(string(got)) != "202" {
		t.Fatalf("bob's pid file is %q", got)
	}

	// B stops: A's files stay as they were.
	aPID := snap(t, a.PIDPath())
	if err := b.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.PIDPath()); !os.IsNotExist(err) {
		t.Fatalf("bob's pid file stays after Stop: %v", err)
	}
	if got := snap(t, a.PIDPath()); !bytes.Equal(got.data, aPID.data) || !got.mtime.Equal(aPID.mtime) {
		t.Fatal("bob's Stop touched alice's pid file")
	}
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	logins, err := f.Logins()
	if err != nil || strings.Join(logins, ",") != "alice,bob" {
		t.Fatalf("Logins = %v, %v", logins, err)
	}
}

func TestStaleLockOfOneLoginIsLeftAloneByAnother(t *testing.T) {
	f := tempFolder(t)
	a, _ := f.Launcher("alice")
	// What a launcher that died leaves: a pid of a process that is gone, a lock file
	// that nobody holds, both old.
	if err := os.MkdirAll(a.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	for path, text := range map[string]string{a.PIDPath(): "999999\n", a.LockPath(): "stale\n", a.LogPath(): "died here\n"} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]snapshot{}
	for _, p := range []string{a.PIDPath(), a.LockPath(), a.LogPath()} {
		before[p] = snap(t, p)
	}

	b, _ := f.Launcher("bob")
	if err := b.Start(202); err != nil {
		t.Fatalf("bob blocked by alice's stale lock: %v", err)
	}
	lg, err := b.OpenLog()
	if err != nil {
		t.Fatal(err)
	}
	lg.WriteString("bob runs\n")
	lg.Close()
	if err := b.Stop(); err != nil {
		t.Fatal(err)
	}
	for p, was := range before {
		got := snap(t, p)
		if !bytes.Equal(got.data, was.data) || !got.mtime.Equal(was.mtime) {
			t.Errorf("alice's %s was touched by bob's launcher", filepath.Base(p))
		}
	}

	// Alice herself takes her own stale lock over.
	if err := a.Start(404); err != nil {
		t.Fatalf("alice cannot take over her stale lock: %v", err)
	}
	if got, _ := os.ReadFile(a.PIDPath()); strings.TrimSpace(string(got)) != "404" {
		t.Fatalf("alice's pid file is %q", got)
	}
	a.Stop()
}

func TestSafeLoginAndCurrentLogin(t *testing.T) {
	for in, want := range map[string]string{
		"grid2":          "grid2",
		`CORP\bob smith`: "CORP_bob_smith",
		"..hidden":       "hidden",
		"a/b":            "a_b",
		"":               "",
		"...":            "",
	} {
		if got := machinefolder.SafeLogin(in); got != want {
			t.Errorf("SafeLogin(%q) = %q, want %q", in, got, want)
		}
	}
	if got := machinefolder.CurrentLogin(func(string) string { return "" }); got == "" || got != machinefolder.SafeLogin(got) {
		t.Errorf("CurrentLogin = %q, want one plain name", got)
	}
	for _, bad := range []string{"", "../x", "a b", "a/b", ".."} {
		if _, err := tempFolder(t).Launcher(bad); err == nil {
			t.Errorf("Launcher(%q) accepted", bad)
		}
	}
}
