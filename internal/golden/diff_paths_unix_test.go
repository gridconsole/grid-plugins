//go:build unix

package golden

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestGdSafePathThroughFile(t *testing.T) {
	box := t.TempDir()
	if err := os.WriteFile(filepath.Join(box, "gdfile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := safePath(box, filepath.Join(box, "gdfile", "x"))
	if err == nil || !strings.HasPrefix(err.Error(), "lstat ") {
		t.Fatalf("safePath error = %v, want one starting %q", err, "lstat ")
	}
}

func TestGdTakeSnapshotRefusesFifo(t *testing.T) {
	box := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(box, "gdpipe"), 0o600); err != nil {
		t.Skipf("mkfifo not available: %v", err)
	}
	_, err := takeSnapshot(box)
	if err == nil || !strings.HasPrefix(err.Error(), "unsupported fixture file: ") {
		t.Fatalf("takeSnapshot error = %v, want one starting %q", err, "unsupported fixture file: ")
	}
}
