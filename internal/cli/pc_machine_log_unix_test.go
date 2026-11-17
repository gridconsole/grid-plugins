//go:build unix

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// pcStart runs startDetached for one login and waits until the program's output is
// in its log.
func pcStart(t *testing.T, env Env, name, word string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	if err := startDetached(env, name, sh, []string{"-c", "echo " + word}); err != nil {
		t.Fatal(err)
	}
	path, err := env.MachineFolder.LogPath(env.GridHome, env.Login, name)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, _ := os.ReadFile(path); strings.Contains(string(b), word) {
			return
		}
	}
	t.Fatalf("%s never held %q", path, word)
}

// Row 3: two logins on one shared folder each get their own launcher folder and log;
// neither one's start touches the other's files; the shared modes come from chmod.
func TestStartDetachedTwoLoginsShareOneMachineFolder(t *testing.T) {
	old := syscall.Umask(0o077) // the modes must come from chmod, not from the umask
	defer syscall.Umask(old)
	machine := filepath.Join(t.TempDir(), "machine")
	folder := machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv, SharedGroup: "grid"}
	alice := Env{GridHome: filepath.Join(t.TempDir(), "alice-grid"), Login: "alice", MachineFolder: folder}
	bob := Env{GridHome: filepath.Join(t.TempDir(), "bob-grid"), Login: "bob", MachineFolder: folder}
	for _, e := range []Env{alice, bob} {
		if err := os.MkdirAll(e.GridHome, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	pcStart(t, alice, "grid-server", "hello-alice")
	aliceLog := filepath.Join(machine, "launcher", "alice", "grid-server.log")
	before, err := os.ReadFile(aliceLog)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(aliceLog)
	if err != nil {
		t.Fatal(err)
	}

	pcStart(t, bob, "grid-server", "hello-bob")
	pcStart(t, bob, "grid-worker", "hello-worker")

	after, err := os.ReadFile(aliceLog)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, _ := os.Stat(aliceLog)
	if !bytes.Equal(before, after) || !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Errorf("alice's log changed when bob started: %q -> %q", before, after)
	}
	bobLog, err := os.ReadFile(filepath.Join(machine, "launcher", "bob", "grid-server.log"))
	if err != nil || strings.Contains(string(bobLog), "alice") || !strings.Contains(string(bobLog), "hello-bob") {
		t.Errorf("bob's log = %q, %v", bobLog, err)
	}
	for _, e := range []Env{alice, bob} {
		if _, err := os.Stat(filepath.Join(e.GridHome, "logs")); err == nil {
			t.Errorf("%s: a log folder was made in the Grid home although the machine folder is on", e.Login)
		}
	}
	for _, dir := range []string{machine, filepath.Join(machine, "launcher"), filepath.Join(machine, "launcher", "alice"), filepath.Join(machine, "launcher", "bob")} {
		st, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Mode() & (os.ModePerm | os.ModeSetgid); got != machinefolder.SharedDirMode && dir != machine {
			t.Errorf("%s mode %v, want %v", dir, got, machinefolder.SharedDirMode)
		}
	}
	for _, f := range []string{aliceLog, filepath.Join(machine, "launcher", "bob", "grid-worker.log")} {
		if st, err := os.Stat(f); err != nil || st.Mode().Perm() != machinefolder.SharedFileMode {
			t.Errorf("%s: %v %v, want %v", f, st, err, os.FileMode(machinefolder.SharedFileMode))
		}
	}
}

// Row 2: with no machine folder, grid open writes <Grid home>/logs/<name>.log, in a
// 0700 folder, as a 0600 file, exactly as before.
func TestStartDetachedUnconfiguredKeepsTheOldLog(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	env := Env{GridHome: filepath.Join(t.TempDir(), ".grid")}
	if err := os.MkdirAll(env.GridHome, 0o700); err != nil {
		t.Fatal(err)
	}
	pcStart(t, env, "grid-server", "hello-old")
	log := filepath.Join(env.GridHome, "logs", "grid-server.log")
	if st, err := os.Stat(log); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("%s: %v %v, want 0600", log, st, err)
	}
	if st, err := os.Stat(filepath.Dir(log)); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("logs folder: %v %v, want 0700", st, err)
	}
}
