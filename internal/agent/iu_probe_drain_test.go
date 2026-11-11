//go:build !windows

package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// iuApDeadline only bounds a wait so a broken build fails by name instead of
// hanging; no assertion depends on how long anything takes.
const iuApDeadline = 60 * time.Second

// iuApProbe is the one place the tests call probeOutput.
func iuApProbe(ctx context.Context, cmd *exec.Cmd) (string, int) {
	return probeOutput(ctx, cmd)
}

// iuApHelperMode returns the arguments after the marker word when the test
// binary was started as a helper, and false in a normal test run.
func iuApHelperMode(word string) ([]string, bool) {
	for i, arg := range os.Args {
		if arg == word {
			return os.Args[i+1:], true
		}
	}
	return nil, false
}

// TestIuApHelper is not a test of its own: TestIuApContextEnds starts the test
// binary on it. The helper starts a sleep in a new session that shares the
// helper's stdout, writes the sleep's pid to the marker and exits 0.
func TestIuApHelper(t *testing.T) {
	args, ok := iuApHelperMode("iu-ap-setsid")
	if !ok {
		return
	}
	path, err := exec.LookPath("sleep")
	if err != nil {
		os.Exit(3)
	}
	sleep := exec.Command(path, "600")
	sleep.Stdout = os.Stdout
	sleep.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := sleep.Start(); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(args[0], []byte(strconv.Itoa(sleep.Process.Pid)), 0o600); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestIuApBackgroundChild(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "hold")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// Open the read end before the probe: the shell's write end then opens at
	// once, and the read end reaches EOF only when every holder of it is gone.
	hold, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hold.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "exec 3>'"+fifo+"'; sleep 30 & echo v1.2.3")
	out, code := iuApProbe(ctx, cmd)
	if out != "v1.2.3\n" || code != 0 {
		t.Fatalf("probeOutput = %q, %d; want \"v1.2.3\\n\" and 0", out, code)
	}
	// The sleep inherited the FIFO. EOF here means the group was ended; it holds
	// the FIFO for 30 s otherwise, so the deadline is far below that.
	if err := hold.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, hold); err != nil {
		t.Fatalf("background sleep still holds the FIFO: %v", err)
	}
}

func TestIuApFullOutput(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), iuApDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", "output")
	out, code := iuApProbe(ctx, cmd)
	if code != 0 || out != strings.Repeat("x", 64*1024) {
		t.Fatalf("probeOutput: code %d, output length %d; want 0 and %d bytes of x", code, len(out), 64*1024)
	}
}

func TestIuApContextEnds(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The marker is a FIFO so the test blocks on the helper's write and polls
	// nothing.
	marker := filepath.Join(t.TempDir(), "marker")
	if err := syscall.Mkfifo(marker, 0o600); err != nil {
		t.Fatal(err)
	}
	type read struct {
		data []byte
		err  error
	}
	markerRead := make(chan read, 1)
	go func() {
		f, err := os.Open(marker)
		if err != nil {
			markerRead <- read{nil, err}
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		markerRead <- read{data, err}
	}()
	// If the helper never starts, this releases the blocked open.
	t.Cleanup(func() {
		if f, err := os.OpenFile(marker, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = f.Close()
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		out  string
		code int
	}
	done := make(chan result, 1)
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestIuApHelper$", "--", "iu-ap-setsid", marker)
	go func() {
		out, code := iuApProbe(ctx, cmd)
		done <- result{out, code}
	}()

	select {
	case m := <-markerRead:
		if m.err != nil {
			t.Fatalf("read marker: %v", m.err)
		}
		pid, err := strconv.Atoi(string(m.data))
		if err != nil {
			t.Fatalf("marker %q is not a pid: %v", m.data, err)
		}
		// The sleep is in its own session and holds the probe's stdout.
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	case r := <-done:
		t.Fatalf("probeOutput returned before the helper wrote its marker: %q, %d", r.out, r.code)
	case <-time.After(iuApDeadline):
		t.Fatal("helper did not write its marker")
	}

	select {
	case r := <-done:
		t.Fatalf("probeOutput returned %q, %d while a process outside the group held stdout and the context was live", r.out, r.code)
	default:
	}
	cancel()
	select {
	case r := <-done:
		if r.code != -1 {
			t.Fatalf("probeOutput after the context ended = %q, %d; want code -1", r.out, r.code)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("context %v", ctx.Err())
		}
	case <-time.After(iuApDeadline):
		t.Fatal("probeOutput did not return after the context ended")
	}
}
