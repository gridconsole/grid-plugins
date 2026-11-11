//go:build unix

package job

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSourceFIFORefused(t *testing.T) {
	for _, kind := range []string{"repo:", "prototype:", "task:"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "pipe.md")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			f := &Files{Task: root, Repo: root, Prototype: root, Folders: []string{root}}
			hang := iqFifoHangCtx(t)
			done := make(chan error, 1)
			go func() { _, err := f.Resolve(kind + "pipe.md#L1"); done <- err }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("want regular-file refusal, got %v", err)
				}
			case <-hang.Done():
				// Release the old blocking reader so a failing test leaves no goroutine behind.
				writer, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
				if err == nil {
					writer.Close()
				}
				select {
				case <-done:
				case <-iqFifoHangCtx(t).Done():
					t.Error("reader did not exit after cleanup")
				}
				t.Fatal("Resolve blocked on FIFO")
			}
		})
	}
}
