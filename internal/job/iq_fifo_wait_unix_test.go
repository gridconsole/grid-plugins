//go:build unix

package job

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// iqFifoHangCtx ends shortly before the test deadline: one tenth of the time
// left, at most one minute, earlier than the deadline. With no test deadline it never ends.
func iqFifoHangCtx(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		return ctx
	}
	margin := time.Until(deadline) / 10
	if margin > time.Minute {
		margin = time.Minute
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-margin))
	t.Cleanup(cancel)
	return ctx
}

func TestIqFifoLateResolve(t *testing.T) {
	for _, kind := range []string{"repo:", "prototype:", "task:"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "pipe.md")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			f := &Files{Task: root, Repo: root, Prototype: root, Folders: []string{root}}
			hang := iqFifoHangCtx(t)
			waiting := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				// Resolve starts only after the waiting side has signalled, however late.
				<-waiting
				if err := hang.Err(); err != nil {
					t.Errorf("hang context ended before Resolve started: %v", err)
				}
				_, err := f.Resolve(kind + "pipe.md#L1")
				done <- err
			}()
			close(waiting)
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("want regular-file refusal, got %v", err)
				}
			case <-hang.Done():
				// Release a blocked reader so a failing test leaves no goroutine behind.
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

func TestIqFifoHangDeadline(t *testing.T) {
	testDeadline, ok := t.Deadline()
	if !ok {
		t.Skip("no test deadline")
	}
	deadline, ok := iqFifoHangCtx(t).Deadline()
	if !ok {
		t.Fatal("hang context has no deadline although the test has one")
	}
	if deadline.After(testDeadline) {
		t.Errorf("hang deadline %v is after the test deadline %v", deadline, testDeadline)
	}
	if deadline.Before(testDeadline.Add(-time.Minute)) {
		t.Errorf("hang deadline %v is more than a minute before the test deadline %v", deadline, testDeadline)
	}
}
