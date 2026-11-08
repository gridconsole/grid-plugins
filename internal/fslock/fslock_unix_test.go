//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package fslock

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenNoFollowRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if f, err := OpenNoFollow(link, os.O_RDONLY, 0); err == nil {
		f.Close()
		t.Fatal("followed a symlink")
	}
	f, err := OpenNoFollow(target, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestLocksConflictAcrossDescriptors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, err := OpenNoFollow(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := OpenNoFollow(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := LockShared(a); err != nil {
		t.Fatal(err)
	}
	if err := LockShared(b); err != nil {
		t.Fatalf("shared locks must coexist: %v", err)
	}
	Unlock(b)
	// A non-blocking attempt proves the exclusive lock is really held.
	if err := LockExclusive(a); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(b.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != syscall.EWOULDBLOCK {
		t.Fatalf("second descriptor got the lock: %v", err)
	}
	Unlock(a)
}
