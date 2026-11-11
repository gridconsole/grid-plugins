//go:build windows

package atomicfile

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type laneRead struct {
	data []byte
	err  error
}

func laneHeldFile(t *testing.T) (dir, path string, old []byte, h *os.File) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "task.md")
	old = bytes.Repeat([]byte("old-bytes "), 8192)
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := os.Open(path) // Go opens without FILE_SHARE_DELETE, like the app reading the file
	if err != nil {
		t.Fatal(err)
	}
	return dir, path, old, h
}

// (a) A reader that lets go after 50 ms is well inside the retry time.
func TestWinLaneWriteWaitsForReaderToClose(t *testing.T) {
	dir, path, old, h := laneHeldFile(t)
	defer h.Close()
	newData := bytes.Repeat([]byte("new-bytes! "), 9000)

	done := make(chan laneRead, 1)
	start := time.Now()
	go func() {
		data, err := io.ReadAll(h)
		time.Sleep(time.Until(start.Add(50 * time.Millisecond)))
		if cerr := h.Close(); err == nil {
			err = cerr
		}
		done <- laneRead{data, err}
	}()

	if err := Write(path, newData, 0o644); err != nil {
		t.Fatalf("Write with a reader closing after 50 ms: %v", err)
	}
	t.Logf("Write returned after %v", time.Since(start))
	r := <-done
	if r.err != nil || !bytes.Equal(r.data, old) {
		t.Fatalf("reader saw %d bytes, err %v; want the %d old bytes in full", len(r.data), r.err, len(old))
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, newData) {
		t.Fatalf("file holds %d bytes, err %v; want the %d new bytes in full", len(got), err, len(newData))
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("temp file left beside the target: %v", got)
	}
}

// (b) A reader that never lets go cannot be outlasted: success or one of the
// three sharing errors, and then the old file is whole and nothing is left over.
func TestWinLaneWriteWithReaderHeldToTheEnd(t *testing.T) {
	dir, path, old, h := laneHeldFile(t)
	defer h.Close()
	newData := bytes.Repeat([]byte("new-bytes! "), 9000)

	err := Write(path, newData, 0o644)
	switch {
	case err == nil:
		t.Log("Write returned nil: Go's share mode let the rename through")
		if got, rerr := os.ReadFile(path); rerr != nil || !bytes.Equal(got, newData) {
			t.Fatalf("file holds %d bytes, err %v; want the %d new bytes in full", len(got), rerr, len(newData))
		}
	case errors.Is(err, syscall.Errno(5)) || errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33)):
		t.Logf("Write returned the sharing error: %v", err)
		if got, rerr := os.ReadFile(path); rerr != nil || !bytes.Equal(got, old) {
			t.Fatalf("file holds %d bytes, err %v; want the %d old bytes in full", len(got), rerr, len(old))
		}
	default:
		t.Fatalf("Write returned %v; want nil or ERROR_ACCESS_DENIED, ERROR_SHARING_VIOLATION or ERROR_LOCK_VIOLATION", err)
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("temp file left beside the target: %v", got)
	}
}
