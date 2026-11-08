//go:build windows

package fslock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const lockTestTimeout = 10 * time.Second

func openTestFile(t *testing.T, path string, flags int) *os.File {
	t.Helper()
	f, err := OpenNoFollow(path, flags, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func awaitLock(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(lockTestTimeout):
		t.Fatal("lock did not complete")
	}
}

func requireBlocked(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("lock returned while conflicting lock was held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestWindowsLockCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name        string
		first, next func(*os.File) error
		blocks      bool
	}{
		{"exclusive/exclusive", LockExclusive, LockExclusive, true},
		{"shared/shared", LockShared, LockShared, false},
		{"shared/exclusive", LockShared, LockExclusive, true},
		{"exclusive/shared", LockExclusive, LockShared, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "lock")
			a := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
			// Read-only handles must support shared locks, as lease readers do.
			flags := os.O_RDWR
			if tc.name == "shared/shared" || tc.name == "exclusive/shared" {
				flags = os.O_RDONLY
			}
			b := openTestFile(t, path, flags)
			if err := tc.first(a); err != nil {
				t.Fatal(err)
			}
			// Release the holder before cleanup closes a possibly waiting b.
			t.Cleanup(func() { Unlock(a) })
			result := make(chan error, 1)
			started := make(chan struct{})
			go func() {
				close(started)
				result <- tc.next(b)
			}()
			<-started
			if tc.blocks {
				requireBlocked(t, result)
				Unlock(a)
			}
			awaitLock(t, result)
			Unlock(b)
		})
	}
}

func TestWindowsTryLockAndRelease(t *testing.T) {
	for _, hold := range []struct {
		name string
		lock func(*os.File) error
	}{{"shared", LockShared}, {"exclusive", LockExclusive}} {
		for _, release := range []string{"unlock", "close"} {
			t.Run(hold.name+"/"+release, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "lock")
				a := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
				b := openTestFile(t, path, os.O_RDWR)
				if err := hold.lock(a); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { Unlock(a) })
				result := make(chan error, 1)
				go func() { result <- TryLockExclusive(b) }()
				select {
				case err := <-result:
					if !errors.Is(err, ErrLocked) {
						t.Fatalf("contended TryLockExclusive: %v", err)
					}
				case <-time.After(lockTestTimeout):
					t.Fatal("TryLockExclusive blocked")
				}
				if release == "close" {
					if err := a.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					Unlock(a)
				}
				if err := TryLockExclusive(b); err != nil {
					t.Fatalf("after %s: %v", release, err)
				}
				Unlock(b)
				Unlock(b) // best effort even when no lock remains
			})
		}
	}
}

func TestWindowsLockedContentRemainsAccessible(t *testing.T) {
	for _, lock := range []struct {
		name string
		take func(*os.File) error
	}{{"shared", LockShared}, {"exclusive", LockExclusive}} {
		t.Run(lock.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "content")
			a := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
			if _, err := a.WriteString("before"); err != nil {
				t.Fatal(err)
			}
			if err := lock.take(a); err != nil {
				t.Fatal(err)
			}
			defer Unlock(a)
			b := openTestFile(t, path, os.O_RDWR)
			if content, err := io.ReadAll(b); err != nil || string(content) != "before" {
				t.Fatalf("read while locked: %q, %v", content, err)
			}
			if _, err := b.WriteAt([]byte("after!"), 0); err != nil {
				t.Fatalf("write while locked: %v", err)
			}
			if content, err := os.ReadFile(path); err != nil || string(content) != "after!" {
				t.Fatalf("content: %q, %v", content, err)
			}
			if info, err := a.Stat(); err != nil || info.Size() != 6 {
				t.Fatalf("locking must not extend the file: %v, %v", info, err)
			}
		})
	}
}

func TestWindowsPendingLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Keep this deliberately asynchronous test handle out of os.NewFile's
	// runtime completion port; production callers use synchronous handles.
	t.Cleanup(func() { _ = syscall.CloseHandle(h) })
	if err := LockExclusive(a); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Unlock(a) })
	result := make(chan error, 1)
	go func() { result <- lockHandle(uintptr(h), lockfileExclusiveLock, false) }()
	requireBlocked(t, result)
	Unlock(a)
	awaitLock(t, result)
	if err := TryLockExclusive(a); !errors.Is(err, ErrLocked) {
		t.Fatalf("pending lock was not granted: %v", err)
	}
	if err := lockHandle(uintptr(h), 0, true); err != nil {
		t.Fatal(err)
	}
	if err := TryLockExclusive(a); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsLockProcess(t *testing.T) {
	if os.Getenv("GRID_FSLOCK_CHILD") == "1" {
		f, err := OpenNoFollow(os.Getenv("GRID_FSLOCK_PATH"), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := LockExclusive(f); err != nil {
			t.Fatal(err)
		}
		fmt.Println("locked")
		// The parent kills us while the handle is still open.
		for {
			time.Sleep(time.Hour)
		}
	}
	path := filepath.Join(t.TempDir(), "lock")
	f := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestWindowsLockProcess$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "GRID_FSLOCK_CHILD=1", "GRID_FSLOCK_PATH="+path)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		if !waited {
			_ = cmd.Wait()
		}
	})
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(out).ReadString('\n')
		if err == nil && line != "locked\n" {
			err = fmt.Errorf("unexpected child output: %q", line)
		}
		ready <- err
	}()
	awaitLock(t, ready)
	if err := TryLockExclusive(f); !errors.Is(err, ErrLocked) {
		t.Fatalf("child did not exclude parent: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("killed child exit: %v", err)
	}
	// Windows may release a terminated process's locks asynchronously.
	deadline := time.Now().Add(lockTestTimeout)
	for {
		err := TryLockExclusive(f)
		if err == nil {
			Unlock(f)
			break
		}
		if !errors.Is(err, ErrLocked) || time.Now().After(deadline) {
			t.Fatalf("lock not released after child death: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWindowsOpenFlags(t *testing.T) {
	// Includes every flag combination used by Windows OpenNoFollow callers.
	for _, tc := range []struct {
		name  string
		flags int
	}{
		{"read", os.O_RDONLY},
		{"write", os.O_WRONLY},
		{"read-write", os.O_RDWR},
		{"lock-create", os.O_RDWR | os.O_CREATE},
		{"feed-append-create", os.O_RDWR | os.O_APPEND | os.O_CREATE},
		{"history-append", os.O_RDWR | os.O_APPEND},
		{"log-truncate-create", os.O_WRONLY | os.O_CREATE | os.O_TRUNC},
		{"intent-exclusive-create", os.O_WRONLY | os.O_CREATE | os.O_EXCL},
		{"write-append", os.O_WRONLY | os.O_APPEND},
		{"truncate", os.O_RDWR | os.O_TRUNC},
		{"truncate-append", os.O_RDWR | os.O_TRUNC | os.O_APPEND},
		{"create-truncate-append", os.O_RDWR | os.O_CREATE | os.O_TRUNC | os.O_APPEND},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%v", tc.name, existing), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "content")
				want := ""
				if existing {
					want = "old"
					if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				f, err := OpenNoFollow(path, tc.flags, 0o600)
				if f != nil {
					t.Cleanup(func() { _ = f.Close() })
				}
				if existing && tc.flags&os.O_EXCL != 0 {
					if !errors.Is(err, os.ErrExist) {
						t.Fatalf("existing O_EXCL: %v", err)
					}
					return
				}
				if !existing && tc.flags&os.O_CREATE == 0 {
					if !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("missing file: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if tc.flags&os.O_TRUNC != 0 {
					want = ""
				}
				if tc.flags&os.O_WRONLY == 0 {
					if got, err := io.ReadAll(f); err != nil || string(got) != want {
						t.Fatalf("read: %q, %v; want %q", got, err, want)
					}
				}
				if tc.flags&(os.O_WRONLY|os.O_RDWR) != 0 {
					if _, err := f.Seek(0, io.SeekStart); err != nil {
						t.Fatal(err)
					}
					if _, err := f.WriteString("new"); err != nil {
						t.Fatal(err)
					}
					if tc.flags&os.O_APPEND != 0 {
						want += "new"
						// Even after seeking backwards, each write must append.
						if _, err := f.Seek(0, io.SeekStart); err != nil {
							t.Fatal(err)
						}
						if _, err := f.WriteString("!"); err != nil {
							t.Fatal(err)
						}
						want += "!"
					} else {
						want = "new"
					}
				} else if _, err := f.WriteString("forbidden"); err == nil {
					t.Fatal("read-only file accepted a write")
				}
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Fatalf("content: %q, %v; want %q", got, err, want)
				}
			})
		}
	}
}

func TestWindowsOpenAllowsRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	f := openTestFile(t, path, os.O_CREATE|os.O_RDWR)
	if err := LockExclusive(f); err != nil {
		t.Fatal(err)
	}
	defer Unlock(f)
	if err := os.Rename(path, filepath.Join(dir, "moved")); err != nil {
		t.Fatalf("rename open locked file: %v", err)
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(replacement, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filepath.Join(dir, "moved")); err != nil {
		t.Fatalf("replace open locked file: %v", err)
	}
}

func requireReparseRefused(t *testing.T, path string, flags int) {
	t.Helper()
	f, err := OpenNoFollow(path, flags, 0o600)
	if f != nil {
		_ = f.Close()
		t.Fatal("returned a handle to a reparse point")
	}
	if !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("reparse error: %v; want ELOOP", err)
	}
	var pe *os.PathError
	if !errors.As(err, &pe) || pe.Path != path || pe.Op != "open" {
		t.Fatalf("missing open path context: %v", err)
	}
}

func TestWindowsOpenRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, syscall.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skipf("file symlink creation needs Windows privilege or Developer Mode: %v", err)
		}
		t.Fatal(err)
	}
	for _, flags := range []int{os.O_RDONLY, os.O_RDWR | os.O_CREATE, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, os.O_RDWR | os.O_APPEND} {
		requireReparseRefused(t, link, flags)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "preserve" {
		t.Fatalf("refused open changed target: %q, %v", got, err)
	}
	if _, err := os.Readlink(link); err != nil {
		t.Fatalf("refused truncation changed symlink: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	requireReparseRefused(t, link, os.O_RDONLY) // dangling link, too
	if err := os.Remove(link); err != nil {
		t.Fatalf("refused open leaked a handle: %v", err)
	}
}

func TestWindowsOpenRefusesJunction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	cmdPath, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Skipf("cmd.exe is unavailable for mklink /J: %v", err)
	}
	// Fixed relative names avoid cmd quoting and metacharacters in TempDir.
	cmd := exec.Command(cmdPath, "/d", "/c", "mklink", "/J", "junction", "target")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("mklink /J is unavailable: %v: %s", err, output)
	}
	junction := filepath.Join(dir, "junction")
	for _, flags := range []int{os.O_RDONLY, os.O_RDWR | os.O_CREATE, os.O_WRONLY | os.O_TRUNC} {
		requireReparseRefused(t, junction, flags)
	}
	if err := os.Remove(junction); err != nil {
		t.Fatalf("refused junction open leaked a handle: %v", err)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("junction target changed: %v, %v", info, err)
	}
}

func TestWindowsClosedFileLockError(t *testing.T) {
	f := openTestFile(t, filepath.Join(t.TempDir(), "lock"), os.O_CREATE|os.O_RDWR)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, lock := range []func(*os.File) error{LockShared, LockExclusive, TryLockExclusive} {
		if err := lock(f); err == nil || errors.Is(err, ErrLocked) {
			t.Fatalf("closed handle error was lost or mapped to contention: %v", err)
		}
	}
	Unlock(f)
}
