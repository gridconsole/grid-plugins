//go:build !windows

package proctree

import (
	"context"
	"errors"
	"os/exec"
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
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	defer cancel()
	started := time.Now()
	err := Run(exec.CommandContext(ctx, "sleep", "30"))
	if err == nil {
		t.Fatal("Run of a cancelled sleep returned nil")
	}
	if took := time.Since(started); took > 2*time.Second {
		t.Fatalf("Run took %v after cancel, want under 2s", took)
	}
}
