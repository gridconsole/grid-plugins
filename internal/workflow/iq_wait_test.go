package workflow

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// iqWfHangCtx ends at the test binary's own deadline less a tenth of the time
// left (at most one minute earlier), so a hung child fails the test with its
// own message before go test panics. With no deadline (-timeout 0) it never
// ends. It catches a hang; it does not time the event.
func iqWfHangCtx(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		return ctx
	}
	early := time.Until(deadline) / 10
	if early > time.Minute {
		early = time.Minute
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-early))
	t.Cleanup(cancel)
	return ctx
}

// iqWfRun starts cmd (built with exec.CommandContext on ctx), closes the
// waiting channel just before it blocks in cmd.Wait, and returns the combined
// output and the Wait error. An optional afterStart hook runs synchronously
// right after Start with the process and the waiting channel; it must not block.
func iqWfRun(t *testing.T, ctx context.Context, cmd *exec.Cmd, afterStart ...func(p *os.Process, waiting <-chan struct{})) ([]byte, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	waiting := make(chan struct{})
	for _, hook := range afterStart {
		hook(cmd.Process, waiting)
	}
	close(waiting)
	err := cmd.Wait()
	return out.Bytes(), err
}

// The hang context is the test's own deadline: values only, no elapsed time.
func TestIqWfHangDeadline(t *testing.T) {
	test, ok := t.Deadline()
	got, has := iqWfHangCtx(t).Deadline()
	if !ok {
		if has {
			t.Fatalf("test has no deadline but the hang context ends at %v", got)
		}
		return
	}
	if !has {
		t.Fatal("hang context has no deadline although the test has one")
	}
	if got.Before(test.Add(-time.Minute)) || got.After(test) {
		t.Fatalf("hang deadline %v outside [%v, %v]", got, test.Add(-time.Minute), test)
	}
}
