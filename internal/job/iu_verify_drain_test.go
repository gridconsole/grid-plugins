//go:build unix

package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// iuJvSetsidArg selects the helper role of TestIuJvHelper in a child test binary.
const iuJvSetsidArg = "iu-jv-setsid"

// iuJvReadPid returns the pid written to path, or 0 while the file is absent or
// does not yet hold a whole line.
func iuJvReadPid(path string) int {
	b, err := os.ReadFile(path)
	if err != nil || !strings.HasSuffix(string(b), "\n") {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// A shell that exits 0 while a background child still holds its output passes;
// the child is ended with the group.
func TestIuJvBackgroundChild(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "held.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// Opened before the run, so the shell's exec 3> does not block, and
	// non-blocking, so reads go through the poller and honour a deadline.
	f, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r, err := RunVerify(ctx, Job{Verify: Verify{Command: "exec 3>'" + fifo + "'; sleep 30 & echo $! > bg.pid; echo ok"}}, dir)
	if err != nil || !r.Pass || r.ExitCode != 0 || r.Output != "ok\n" {
		t.Fatalf("a shell that exits 0 with a background child should pass: %+v %v", r, err)
	}
	// The background sleep holds the only write end of the FIFO once its shell
	// is gone, so EOF means the sleep is gone too.
	if err := f.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, f); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("background sleep still holds the FIFO")
		}
		t.Fatal(err)
	}
}

// Output larger than a pipe buffer is read to the end and every test line counts.
func TestIuJvFullOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := `i=0; while [ "$i" -lt 3000 ]; do echo "--- PASS: TestA$i (0.00s)"; i=$((i+1)); done`
	r, err := RunVerify(ctx, Job{Verify: Verify{Command: command, MinTests: 3000}}, t.TempDir())
	tests := -1
	if r.Tests != nil {
		tests = *r.Tests
	}
	if err != nil || !r.Pass || tests != 3000 {
		t.Fatalf("all 3000 test lines should be read: pass=%t exit=%d tests=%d reason=%q err=%v", r.Pass, r.ExitCode, tests, r.Reason, err)
	}
}

// A process outside the group that keeps the output open holds RunVerify until
// its context ends; then RunVerify returns the context's error.
func TestIuJvContextEnds(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	t.Cleanup(func() {
		if pid := iuJvReadPid(marker); pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := fmt.Sprintf("'%s' -test.run='^TestIuJvHelper$' -- %s '%s'", exe, iuJvSetsidArg, marker)
	done := make(chan error, 1)
	go func() {
		_, err := RunVerify(ctx, Job{Verify: Verify{Command: command}}, dir)
		done <- err
	}()
	deadline := time.After(time.Minute)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for iuJvReadPid(marker) == 0 {
		select {
		case err := <-done:
			t.Fatalf("RunVerify returned before the helper wrote its marker: %v", err)
		case <-deadline:
			t.Fatal("helper never wrote its marker")
		case <-ticker.C:
		}
	}
	select {
	case err := <-done:
		t.Fatalf("RunVerify returned while a process outside the group held the output: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunVerify after cancel: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("RunVerify did not return after its context ended")
	}
}

// TestIuJvHelper is the child process of TestIuJvContextEnds and does nothing
// in a normal run. With the setsid argument it starts a sleep in its own
// session that shares its stdout, records the sleep's pid in the marker, prints
// ok and exits 0.
func TestIuJvHelper(t *testing.T) {
	i := slices.Index(os.Args, "--")
	if i < 0 || len(os.Args) != i+3 || os.Args[i+1] != iuJvSetsidArg {
		return
	}
	marker := os.Args[i+2]
	sleep := exec.Command("sleep", "600")
	sleep.Stdout = os.Stdout
	sleep.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := sleep.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.WriteFile(marker, []byte(strconv.Itoa(sleep.Process.Pid)+"\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("ok")
	os.Exit(0)
}
