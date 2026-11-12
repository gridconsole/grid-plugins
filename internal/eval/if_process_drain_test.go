//go:build linux || darwin

package eval

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This file needs named pipes (syscall.Mkfifo) and a new session
// (SysProcAttr.Setsid), so it builds on linux and darwin only. The timeout case
// of TestFailuresAreSaved lives here for the same reason and registers itself
// through ifEdTimeoutCase (runner_test.go).

// ifEdWait only bounds a wait, so a hang fails by name with what it waited for.
// No test result depends on a time passing.
const ifEdWait = 20 * time.Second

const (
	ifEdHelperEnv = "GRID_EVAL_IFED_HELPER"
	ifEdFifoEnv   = "GRID_EVAL_IFED_FIFO"
)

func init() { ifEdTimeoutCase = ifEdTimeout }

func ifEdMakeFIFO(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestIfEdHelperProcess is the program TestIfEdContextEnds runs from this test
// binary, chosen by an environment variable; without it the test does nothing.
// "parent" starts a "holder" in a new session, with the parent's stdout, says so
// on stdout and exits. The holder reads the FIFO the test holds open until the
// test closes it, so it ends with the test and never outlives it.
func TestIfEdHelperProcess(t *testing.T) {
	switch os.Getenv(ifEdHelperEnv) {
	case "parent":
		holder := exec.Command(os.Args[0], "-test.run=^TestIfEdHelperProcess$")
		holder.Env = append(os.Environ(), ifEdHelperEnv+"=holder")
		holder.Stdout = os.Stdout
		holder.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := holder.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "holder failed:", err)
			os.Exit(3)
		}
		fmt.Println("started", holder.Process.Pid)
		os.Exit(0)
	case "holder":
		// Open without waiting for a writer, then read blocking: with no writer
		// left the read gives EOF at once, so the holder ends in every order of
		// events, also when the test closed its end before this open.
		fd, err := syscall.Open(os.Getenv(ifEdFifoEnv), syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			if syscall.SetNonblock(fd, false) == nil {
				buf := make([]byte, 4096)
				for {
					n, err := syscall.Read(fd, buf)
					if err == syscall.EINTR {
						continue
					}
					if n <= 0 || err != nil {
						break
					}
				}
			}
			_ = syscall.Close(fd)
		}
		os.Exit(0)
	}
}

// ifEdPidGone reports that no live process has the pid (a zombie counts as
// gone: it only waits for its reaper).
func ifEdPidGone(pid int) bool {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return true
	}
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		return bytes.Contains(data, []byte(") Z"))
	}
	return false
}

// The holder of TestIfEdContextEnds must end by itself when the FIFO has no
// writer any more before it opens it (a test that failed or ended early). This
// runs the "parent" helper with the FIFO never opened, reads the holder's pid
// from the line beside "started" and waits for that process to be gone.
func TestIfEdHolderEndsWithoutWriter(t *testing.T) {
	fifo := ifEdMakeFIFO(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestIfEdHelperProcess$")
	cmd.Env = append(os.Environ(), ifEdHelperEnv+"=parent", ifEdFifoEnv+"="+fifo)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var pid int
	lineCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		lineCh <- line
	}()
	select {
	case line := <-lineCh:
		if _, err := fmt.Sscanf(line, "started %d", &pid); err != nil || pid <= 0 {
			t.Fatalf("first line %q is not %q", line, "started <pid>")
		}
	case <-time.After(ifEdWait):
		_ = cmd.Process.Kill()
		t.Fatalf("no first line from the parent within %v", ifEdWait)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	defer func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(ifEdWait)
	for !ifEdPidGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("the holder (pid %d) was still alive %v after its FIFO had no writer", pid, ifEdWait)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ifEdLineSink is the tee of runProcess: it closes first when the first line
// of stdout has arrived.
type ifEdLineSink struct {
	text  bytes.Buffer
	first chan struct{}
	once  sync.Once
}

func (s *ifEdLineSink) Write(p []byte) (int, error) {
	s.text.Write(p)
	if bytes.IndexByte(s.text.Bytes(), '\n') >= 0 {
		s.once.Do(func() { close(s.first) })
	}
	return len(p), nil
}

// A background child of the command must be gone when RunProcess returns, and
// the output must still be complete. The event that proves it: the shell opens
// the FIFO for writing before it forks, so the background child holds a write
// end too, and the FIFO reads EOF only when both are gone.
func TestIfEdBackgroundChild(t *testing.T) {
	fifo := ifEdMakeFIFO(t)
	reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	env := append(os.Environ(), ifEdFifoEnv+"="+fifo)
	r := RunProcess(ctx, []string{"sh", "-c", `exec 3>"$` + ifEdFifoEnv + `"; sleep 30 & echo $!; echo ok`}, t.TempDir(), env, "")
	if r.Status != "passed" || r.ExitCode != 0 || r.Error != "" {
		t.Fatalf("a command that exited 0 was not passed: status %q, exit %d, error %q, stderr %q", r.Status, r.ExitCode, r.Error, r.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(r.Stdout), "\n")
	if pid, err := strconv.Atoi(lines[0]); len(lines) != 2 || err != nil || pid <= 0 || lines[1] != "ok" {
		t.Fatalf("stdout is not the PID line and ok: %q", r.Stdout)
	}
	if err := reader.SetReadDeadline(time.Now().Add(ifEdWait)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, reader); errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("the background child still held the FIFO %v after RunProcess returned", ifEdWait)
	} else if err != nil {
		t.Fatal(err)
	}
}

// The drain reads to EOF: a child that writes the most the result can hold and
// exits 0 gives exactly those bytes, not cut short and not marked truncated.
func TestIfEdFullOutput(t *testing.T) {
	size := 64 * 1024
	if outputLimit < size {
		size = outputLimit
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r := RunProcess(ctx, []string{"sh", "-c", fmt.Sprintf(`head -c %d /dev/zero | tr '\0' x`, size)}, t.TempDir(), os.Environ(), "")
	if r.Status != "passed" || r.ExitCode != 0 || r.Error != "" {
		t.Fatalf("status %q, exit %d, error %q, stderr %q", r.Status, r.ExitCode, r.Error, r.Stderr)
	}
	if r.Truncated || r.Stdout != strings.Repeat("x", size) {
		t.Fatalf("stdout holds %d of %d bytes (truncated %v)", len(r.Stdout), size, r.Truncated)
	}
}

// A descendant that left the group and holds stdout is cut off when the context
// ends, and not before: nothing but the context decides.
func TestIfEdContextEnds(t *testing.T) {
	fifo := ifEdMakeFIFO(t)
	hold, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hold.Close() }) // the holder reads EOF and exits
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := &ifEdLineSink{first: make(chan struct{})}
	env := append(os.Environ(), ifEdHelperEnv+"=parent", ifEdFifoEnv+"="+fifo)
	done := make(chan ProcessResult, 1)
	go func() {
		done <- runProcess(ctx, []string{os.Args[0], "-test.run=^TestIfEdHelperProcess$"}, "", env, "", sink)
	}()
	select {
	case <-sink.first:
	case r := <-done:
		t.Fatalf("runProcess returned before the child's first line: %+v", r)
	case <-time.After(ifEdWait):
		t.Fatalf("no first line from the child within %v", ifEdWait)
	}
	select {
	case r := <-done:
		t.Fatalf("runProcess returned before the context was cancelled, while a descendant holds stdout: %+v", r)
	default:
	}
	cancel()
	select {
	case r := <-done:
		if r.Error != "context canceled" {
			t.Fatalf("error %q, want %q: %+v", r.Error, "context canceled", r)
		}
	case <-time.After(ifEdWait):
		t.Fatalf("runProcess did not return within %v after the context was cancelled", ifEdWait)
	}
}

// ifEdCtx is a parent context the test ends by hand: after end, Done is closed
// and Err is context.DeadlineExceeded, as if a deadline had fallen. Runner.Run
// passes that Err on through context.WithTimeout.
type ifEdCtx struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newIfEdCtx() *ifEdCtx {
	return &ifEdCtx{Context: context.Background(), done: make(chan struct{})}
}

func (c *ifEdCtx) Done() <-chan struct{} { return c.done }

func (c *ifEdCtx) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *ifEdCtx) end() { c.once.Do(func() { close(c.done) }) }

// ifEdTimeout is the timeout case of TestFailuresAreSaved. The deadline falls
// when the test has read "started" from a named pipe the agent wrote to, so the
// agent has really begun, however slow setup was; Runner.Timeout stays at
// testRunner's backstop. Every assertion of the old case is kept.
func ifEdTimeout(t *testing.T) {
	task, runner := sampleTask(t), testRunner(t)
	task.Setup = "true"
	fifo := ifEdMakeFIFO(t)
	runner.Agent = []string{"sh", "-c", "echo started > '" + fifo + "'; exec sleep 600"}
	parent := newIfEdCtx()
	started := make(chan struct{})
	reading := make(chan struct{})
	go func() {
		defer close(reading)
		f, err := os.Open(fifo) // waits for the agent to open it for writing
		if err != nil {
			return
		}
		defer func() { _ = f.Close() }()
		if data, _ := io.ReadAll(f); string(data) == "started\n" {
			close(started)
			parent.end()
		}
	}()
	t.Cleanup(func() {
		// A reader still waiting in open (the agent never got that far) is let
		// through, then sees EOF at once.
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
		<-reading
	})
	type ran struct {
		result Result
		err    error
	}
	done := make(chan ran, 1)
	go func() {
		result, err := runner.Run(parent, task)
		done <- ran{result, err}
	}()
	var got ran
	select {
	case got = <-done:
	case <-time.After(ifEdWait):
		t.Fatalf("Runner.Run did not return within %v after the parent context ended with DeadlineExceeded", ifEdWait)
	}
	select {
	case <-started:
	default:
		t.Fatalf("the run ended before the agent wrote started: %+v, %v", got.result, got.err)
	}
	r := checkSavedRun(t, runner, got.result, got.err)
	if r.Status != "error" || r.Summary.Skipped != len(task.Checks) || r.Summary.CheckScore != nil {
		t.Fatalf("bad failure record: %+v", r)
	}
	if r.AgentResult.ExitCode == 0 || !strings.Contains(r.AgentResult.Error, "deadline") {
		t.Fatalf("timeout did not stop the command: %+v", r)
	}
}

// The runner's own timer still ends a task: with a Timeout that has already
// passed, setup is the step that reports the deadline, the agent is skipped and
// the record is saved (checkSavedRun loads it with LoadResult).
func TestIfEdRunnerTimerEndsTask(t *testing.T) {
	task, runner := sampleTask(t), testRunner(t)
	runner.Timeout = time.Nanosecond
	runner.Agent = []string{"sh", "-c", "exit 0"}
	result, err := runner.Run(context.Background(), task)
	r := checkSavedRun(t, runner, result, err)
	if r.Status != "error" || !strings.Contains(r.Setup.Error, "deadline") || r.AgentResult.Status != "skipped" {
		t.Fatalf("the runner's timer did not end the task: %+v", r)
	}
}
