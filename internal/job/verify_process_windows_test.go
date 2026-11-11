//go:build windows

package job

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const verifyTreeHelperEnv = "GRID_JOB_VERIFY_TREE_HELPER"

// TestWindowsVerifyTreeHelper is the program the tree tests start from this test
// binary. "parent" starts a grandchild and reports both pids; "leaf" only waits.
// Without the variable it does nothing, so a normal run passes it.
func TestWindowsVerifyTreeHelper(t *testing.T) {
	switch os.Getenv(verifyTreeHelperEnv) {
	case "parent":
		grandchild := exec.Command(os.Args[0], "-test.run=^TestWindowsVerifyTreeHelper$")
		grandchild.Env = append(os.Environ(), verifyTreeHelperEnv+"=leaf")
		if err := grandchild.Start(); err != nil {
			fmt.Println("grandchild failed:", err)
			os.Exit(3)
		}
		fmt.Printf("pids %d %d\n", os.Getpid(), grandchild.Process.Pid)
		time.Sleep(time.Minute)
		os.Exit(0)
	case "leaf":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

// verifyTreeCommand is the cmd.exe command line of the tree: the shell starts
// the helper parent, which starts the grandchild. The test name has no cmd.exe
// metacharacters, so it needs no quoting beyond the binary path.
func verifyTreeCommand() string {
	return `"` + os.Args[0] + `" -test.run=TestWindowsVerifyTreeHelper`
}

// verifyTreeCmd builds the command through verifyShell as verify.go does, with the
// same WaitDelay, and points its output at sink.
func verifyTreeCmd(ctx context.Context, sink *pidSink) *exec.Cmd {
	cmd := verifyShell(ctx, verifyTreeCommand())
	cmd.Env = append(os.Environ(), verifyTreeHelperEnv+"=parent")
	cmd.WaitDelay = 250 * time.Millisecond
	cmd.Stdout, cmd.Stderr = sink, sink
	return cmd
}

// pidSink collects the helper's output and signals once its first line is in.
type pidSink struct {
	mu   sync.Mutex
	text strings.Builder
	done chan struct{}
	once sync.Once
}

func newPidSink() *pidSink { return &pidSink{done: make(chan struct{})} }

func (s *pidSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.text.Write(p)
	if strings.Contains(s.text.String(), "\n") {
		s.once.Do(func() { close(s.done) })
	}
	return len(p), nil
}

func (s *pidSink) pids(t *testing.T) (parent, grandchild int) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(30 * time.Second):
		t.Fatal("helper never reported its pids")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Sscanf(s.text.String(), "pids %d %d", &parent, &grandchild); err != nil {
		t.Fatalf("helper output %q: %v", s.text.String(), err)
	}
	return parent, grandchild
}

func processGone(pid int) bool {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	event, err := syscall.WaitForSingleObject(h, 0)
	return err == nil && event == syscall.WAIT_OBJECT_0
}

func requireGone(t *testing.T, name string, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("%s (pid %d) is still running", name, pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestWindowsVerifyCancelKillsProcessTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := newPidSink()
	cmd := verifyTreeCmd(ctx, sink)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	shell := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	parent, grandchild := sink.pids(t)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled verification reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("verification did not return within 10 s of cancel")
	}
	cleanupVerifyProcess(cmd)
	requireGone(t, "shell", shell)
	requireGone(t, "child", parent)
	requireGone(t, "grandchild", grandchild)
}

func TestWindowsVerifyCleanupKillsProcessTree(t *testing.T) {
	sink := newPidSink()
	cmd := verifyTreeCmd(context.Background(), sink)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	shell := cmd.Process.Pid
	parent, grandchild := sink.pids(t)
	cleanupVerifyProcess(cmd)
	_ = cmd.Wait()
	requireGone(t, "shell", shell)
	requireGone(t, "child", parent)
	requireGone(t, "grandchild", grandchild)
}

func TestWindowsVerifyShellKeepsCommandLineAndStartsProcessGroup(t *testing.T) {
	const command = `echo "a b" && exit 0`
	cmd := verifyShell(context.Background(), command)
	if cmd.SysProcAttr == nil {
		t.Fatal("verifyShell set no SysProcAttr")
	}
	if want := windowsVerifyCommandLine(command); cmd.SysProcAttr.CmdLine != want {
		t.Fatalf("CmdLine = %q, want %q", cmd.SysProcAttr.CmdLine, want)
	}
	if cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 || cmd.Cancel == nil {
		t.Fatalf("verifyShell: %+v cancel=%v", cmd.SysProcAttr, cmd.Cancel != nil)
	}
}

func TestWindowsVerifyCleanupSkipsUnstartedAndFinishedCommands(t *testing.T) {
	cleanupVerifyProcess(verifyShell(context.Background(), "exit 0"))
	cmd := verifyShell(context.Background(), "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	// ProcessState is set, so the pid is never used for a kill.
	cleanupVerifyProcess(cmd)
}
