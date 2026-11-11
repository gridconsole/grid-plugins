package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

var errLaneBusy = &os.LinkError{Op: "rename", Old: "from", New: "to", Err: syscall.Errno(32)}

func laneBusy(err error) bool { return errors.Is(err, syscall.Errno(32)) }

func TestAtomicLaneRetryTwiceThenSucceeds(t *testing.T) {
	calls := 0
	var pauses []time.Duration
	err := retryRename(func() error {
		calls++
		if calls <= 2 {
			return errLaneBusy
		}
		return nil
	}, laneBusy, func(d time.Duration) { pauses = append(pauses, d) })
	if err != nil {
		t.Fatalf("want nil after two retryable failures, got %v", err)
	}
	if calls != 3 || len(pauses) != 2 {
		t.Fatalf("calls %d, pauses %v; want 3 calls and 2 pauses", calls, pauses)
	}
}

func TestAtomicLaneRetryNonRetryableReturnsAtOnce(t *testing.T) {
	calls, paused := 0, 0
	want := errors.New("permanent")
	err := retryRename(func() error { calls++; return want }, laneBusy, func(time.Duration) { paused++ })
	if err != want || calls != 1 || paused != 0 {
		t.Fatalf("err %v, calls %d, pauses %d; want the error after 1 call and no pause", err, calls, paused)
	}
}

func TestAtomicLaneRetryAllFailReturnsLastError(t *testing.T) {
	calls := 0
	var total time.Duration
	err := retryRename(func() error {
		calls++
		return fmt.Errorf("busy %d: %w", calls, errLaneBusy)
	}, laneBusy, func(d time.Duration) { total += d })
	if calls != renameAttempts {
		t.Fatalf("calls %d, want %d", calls, renameAttempts)
	}
	if want := fmt.Sprintf("busy %d", renameAttempts); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err %v, want the last one (%s)", err, want)
	}
	if total <= 0 || total >= time.Second {
		t.Fatalf("pauses total %v, want above 0 and under one second", total)
	}
}

func TestAtomicLaneRenameReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")
	if err := os.WriteFile(from, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rename(from, to); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(to); err != nil || string(got) != "new" {
		t.Fatalf("got %q, %v; want new", got, err)
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("source left behind: %v", got)
	}
}

func TestAtomicLaneUnixRenameDoesNotRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows retries")
	}
	calls := 0
	err := renameRetrying(func() error { calls++; return syscall.Errno(5) })
	if err == nil || calls != 1 {
		t.Fatalf("err %v, calls %d; want the error after 1 call", err, calls)
	}
}

func laneOpenFile(t *testing.T, dir string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(dir, "lane-*")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAtomicLaneReplaceClosedFileReturnsWriteError(t *testing.T) {
	f := laneOpenFile(t, t.TempDir())
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	renamed, synced := false, false
	err := replace(f, []byte("x"), 0o600, func() error { renamed = true; return nil }, func() error { synced = true; return nil })
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("err %v, want the closed-file write error", err)
	}
	if renamed || synced {
		t.Fatalf("rename ran %v, syncDir ran %v; want neither", renamed, synced)
	}
}

func TestAtomicLaneReplaceRenameFailureSkipsSyncDir(t *testing.T) {
	f := laneOpenFile(t, t.TempDir())
	want := errors.New("rename failed")
	synced := false
	err := replace(f, []byte("x"), 0o600, func() error { return want }, func() error { synced = true; return nil })
	if err != want {
		t.Fatalf("err %v, want %v", err, want)
	}
	if synced {
		t.Fatal("syncDir ran after a failed rename")
	}
}

func TestAtomicLaneReplaceSyncDirFailureAfterRename(t *testing.T) {
	f := laneOpenFile(t, t.TempDir())
	want := errors.New("sync failed")
	var order []string
	err := replace(f, []byte("x"), 0o600, func() error { order = append(order, "rename"); return nil }, func() error { order = append(order, "syncDir"); return want })
	if err != want {
		t.Fatalf("err %v, want %v", err, want)
	}
	if got := strings.Join(order, ","); got != "rename,syncDir" {
		t.Fatalf("order %s, want rename,syncDir", got)
	}
}

func TestAtomicLaneReplaceWritesTheData(t *testing.T) {
	dir := t.TempDir()
	f := laneOpenFile(t, dir)
	if err := replace(f, []byte("payload"), 0o600, func() error { return nil }, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(f.Name()); err != nil || string(got) != "payload" {
		t.Fatalf("got %q, %v; want payload", got, err)
	}
}

func TestAtomicLaneWriteOverNonEmptyFolderFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(inner, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("new"), 0o644); err == nil {
		t.Fatal("Write replaced a non-empty folder")
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		t.Fatalf("target is no longer a folder: %v, %v", st, err)
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "keep" {
		t.Fatalf("folder content changed: %q, %v", got, err)
	}
	if got := names(t, dir); len(got) != 1 || got[0] != "target" {
		t.Fatalf("temp file left beside the folder: %v", got)
	}
}

func TestAtomicLaneWriteKeepsExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	path := filepath.Join(t.TempDir(), "note")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v, %v; want 0640", st, err)
	}
}

func TestAtomicLaneWriteRootRefusesEscapeAndFolder(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Mkdir("folder", 0o700); err != nil {
		t.Fatal(err)
	}

	err = WriteRoot(root, "../x", []byte("new"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "must be relative") {
		t.Fatalf("../x: err %v, want the relative-path refusal", err)
	}
	if got := names(t, parent); len(got) != 1 || got[0] != "home" {
		t.Fatalf("wrote beside the root: %v", got)
	}

	err = WriteRoot(root, "folder", []byte("new"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("folder: err %v, want the not-a-regular-file refusal", err)
	}
	if st, err := os.Stat(filepath.Join(home, "folder")); err != nil || !st.IsDir() {
		t.Fatalf("folder is gone: %v, %v", st, err)
	}
	if got := names(t, home); len(got) != 1 || got[0] != "folder" {
		t.Fatalf("temp file left in the root: %v", got)
	}
}
