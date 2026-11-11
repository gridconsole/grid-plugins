//go:build linux || darwin

package ports

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// iqPsSurvivor leaves a process in the shell's group that never ends by itself
// (2147483647 s is far beyond any go test timeout) and holds the pipe open. The
// leader exits after a short sleep; only a signal to the group ends the child.
const iqPsSurvivor = "sleep 2147483647 & sleep 0.3"

// iqPsHangCtx ends at t.Deadline() less one tenth of the time left, at most one
// minute, so a hung wait fails from the test before the go test timeout panic.
// Without a test deadline it never ends. It is a wait's only deadline.
func iqPsHangCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if deadline, ok := t.Deadline(); ok {
		ctx, cancel = context.WithDeadline(context.Background(), iqPsHangEnd(deadline, time.Now()))
	}
	t.Cleanup(cancel)
	return ctx
}

// iqPsHangEnd is deadline less one tenth of the time left from now, at most one minute.
func iqPsHangEnd(deadline, now time.Time) time.Time {
	margin := deadline.Sub(now) / 10
	if margin > time.Minute {
		margin = time.Minute
	}
	return deadline.Add(-margin)
}

// iqPsOrphanGroup runs script in its own process group, waits for the leader to
// exit, and returns the group's record with a channel that receives the result
// of reading the pipe to EOF. Its survivors are killed at cleanup.
func iqPsOrphanGroup(t *testing.T, script string) (Record, <-chan error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = writer
	if err := cmd.Start(); err != nil {
		writer.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() })
	writer.Close()
	r := Record{PID: cmd.Process.Pid, Group: true, PIDStart: processid.Start(cmd.Process.Pid)}
	if r.PIDStart == "" {
		t.Fatal("missing live leader start token")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if Alive(r.PID, false) || !Alive(r.PID, true) {
		t.Fatal("expected a live group with its leader gone")
	}
	// EOF proves the surviving child has stopped, even if an orphaned
	// zombie still makes kill(0) report that the group exists.
	exited := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, reader); exited <- err }()
	return r, exited
}

func TestIqPsHangDeadline(t *testing.T) {
	deadline, ok := t.Deadline()
	if !ok {
		t.Skip("no test deadline")
	}
	end, ok := iqPsHangCtx(t).Deadline()
	if !ok {
		t.Fatal("hang context has no deadline under a test deadline")
	}
	if end.Before(deadline.Add(-time.Minute)) || end.After(deadline) {
		t.Fatalf("hang deadline %s outside [%s, %s]", end, deadline.Add(-time.Minute), deadline)
	}
}

func TestIqPsHangEnd(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, tc := range []struct{ left, want time.Duration }{
		{10 * time.Second, 9 * time.Second},
		{time.Hour, time.Hour - time.Minute},
	} {
		if got := iqPsHangEnd(now.Add(tc.left), now); got != now.Add(tc.want) {
			t.Fatalf("left %s: ends %s after now, want %s", tc.left, got.Sub(now), tc.want)
		}
	}
}

// A stopped survivor holds SIGINT and SIGTERM pending; only SIGKILL ends it.
func TestIqPsLateEOF(t *testing.T) {
	r, exited := iqPsOrphanGroup(t, iqPsSurvivor)
	hang := iqPsHangCtx(t)
	// The leader is gone, so this stops only the survivor.
	if err := syscall.Kill(-r.PID, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	if err := StopRecord(r, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-hang.Done():
		t.Fatal("group child survived shutdown")
	}
	if err := hang.Err(); err != nil {
		t.Fatalf("hang context ended: %v", err)
	}
}
