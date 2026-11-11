//go:build !windows

package proctree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunUnixExitCodeIsReturnedAsExitError(t *testing.T) {
	err := Run(exec.Command("sh", "-c", "exit 3"))
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run returned %v (%T), want *exec.ExitError", err, err)
	}
	if exitErr.ExitCode() != 3 {
		t.Fatalf("exit code = %d, want 3", exitErr.ExitCode())
	}
}

func TestRunUnixSuccessReturnsNil(t *testing.T) {
	if err := Run(exec.Command("sh", "-c", "exit 0")); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunUnixStartFailureIsReturned(t *testing.T) {
	if err := Run(exec.Command("/nonexistent/grid-proctree-test")); err == nil {
		t.Fatal("Run of a missing program returned nil")
	}
}

func TestRunUnixCancelledContextReturnsPromptly(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel once the child has written its pid, so the cancel always lands on a
	// running child. The 30 s bound only names what never happened.
	neverStarted := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		bound := time.After(30 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-bound:
				close(neverStarted)
				cancel()
				return
			case <-ticker.C:
				if b, err := os.ReadFile(pidFile); err == nil && len(strings.TrimSpace(string(b))) > 0 {
					cancel()
					return
				}
			}
		}
	}()

	cmd := exec.CommandContext(ctx, "sh", "-c", `echo $$ > "$DIR/pid"; exec sleep 30`)
	cmd.Env = append(os.Environ(), "DIR="+dir)
	err := Run(cmd)
	select {
	case <-neverStarted:
		t.Fatal("child never started")
	default:
	}
	if err == nil {
		t.Fatal("Run of a cancelled sleep returned nil")
	}
	if cmd.ProcessState == nil {
		t.Fatal("cmd.ProcessState is nil after Run")
	}
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		t.Fatalf("child was not signalled: %v", cmd.ProcessState)
	}
	b, rerr := os.ReadFile(pidFile)
	if rerr != nil {
		t.Fatalf("read pid file: %v", rerr)
	}
	pid, perr := strconv.Atoi(strings.TrimSpace(string(b)))
	if perr != nil {
		t.Fatalf("pid file %q: %v", b, perr)
	}
	if kerr := syscall.Kill(pid, 0); !errors.Is(kerr, syscall.ESRCH) {
		t.Fatalf("child %d still exists after cancel: %v", pid, kerr)
	}
}
