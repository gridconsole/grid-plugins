//go:build linux || darwin

package localdata

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testenv"
)

// TestLockHelperProcess is the body of the child process the tests below start;
// it does nothing in a normal run.
func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("LOCALDATA_LOCK_HELPER") == "" {
		t.Skip("helper process only")
	}
	if _, err := Lock(os.Getenv("LOCALDATA_HOME"), ".test.lock"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("LOCALDATA_MARKER"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Hour) // hold the lock until the parent kills this process
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// lockWithin fails the test when Lock does not return before the deadline.
func lockWithin(t *testing.T, home string) func() {
	t.Helper()
	type result struct {
		unlock func()
		err    error
	}
	done := make(chan result, 1)
	go func() {
		unlock, err := Lock(home, ".test.lock")
		done <- result{unlock, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.unlock
	case <-time.After(10 * time.Second):
		t.Fatal("Lock is still waiting; a dead writer wedged it")
		return nil
	}
}

func TestKilledWriterDoesNotBlockTheNextWrite(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(testenv.Temp(t, os.Environ()), "LOCALDATA_LOCK_HELPER=1", "LOCALDATA_HOME="+home, "LOCALDATA_MARKER="+marker)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	waitUntil(t, "the holder to take the lock", func() bool { _, err := os.Stat(marker); return err == nil })
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if _, err := os.Stat(filepath.Join(home, ".test.lock")); err != nil {
		t.Fatal("the killed holder should leave its lock file behind")
	}
	lockWithin(t, home)()
}

func TestLeftoverLockFileDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".test.lock"), []byte("pid 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockWithin(t, home)()
}

func TestLockSerializesWriters(t *testing.T) {
	home := t.TempDir()
	var inside atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			unlock, err := Lock(home, ".test.lock")
			if err != nil {
				t.Error(err)
				return
			}
			defer unlock()
			if inside.Add(1) != 1 {
				t.Error("two writers hold the lock at once")
			}
			time.Sleep(2 * time.Millisecond) // long enough for a second writer to overlap
			inside.Add(-1)
		})
	}
	wg.Wait()
}

func TestLockRefusesSymlink(t *testing.T) {
	home := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(home, ".test.lock")); err != nil {
		t.Skip("symlinks not available")
	}
	if _, err := Lock(home, ".test.lock"); err == nil {
		t.Fatal("locked through a symlink")
	}
}
