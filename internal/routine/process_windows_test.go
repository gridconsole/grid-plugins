//go:build windows

package routine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const treeHelperEnv = "GRID_ROUTINE_TREE_HELPER"

// TestWindowsTreeHelper is the program the tree tests start from this test
// binary. "parent" starts a grandchild and reports both pids; "leaf" only waits.
// Without the variable it does nothing, so a normal run passes it.
func TestWindowsTreeHelper(t *testing.T) {
	switch os.Getenv(treeHelperEnv) {
	case "parent":
		grandchild := exec.Command(os.Args[0], "-test.run=^TestWindowsTreeHelper$")
		grandchild.Env = append(os.Environ(), treeHelperEnv+"=leaf")
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

// A routine that is stopped (its context ends) loses the agent and the agent's own children.
func TestWindowsRoutineStopKillsProcessTree(t *testing.T) {
	sink := newPidSink()
	var stop context.CancelFunc
	p := ProcessExecutor{
		GridBinary: os.Args[0],
		Stdout:     sink,
		withTimeout: func(ctx context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
			ctx, stop = context.WithCancel(ctx)
			return ctx, stop
		},
		// Keep the real command setup (process group, Cancel, cleanup) and run the helper instead of grid.
		runCommand: func(cmd *exec.Cmd) error {
			cmd.Args = []string{cmd.Path, "-test.run=^TestWindowsTreeHelper$"}
			cmd.Env = append(cmd.Env, treeHelperEnv+"=parent")
			go func() {
				select {
				case <-sink.done:
				case <-time.After(30 * time.Second):
				}
				stop()
			}()
			return cmd.Run()
		},
	}
	err := p.Execute(context.Background(), Store{}, Entry{Run: "grid example"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("execute: %v", err)
	}
	parent, grandchild := sink.pids(t)
	requireGone(t, "child", parent)
	requireGone(t, "grandchild", grandchild)
}

func TestWindowsCleanupKillsProcessTree(t *testing.T) {
	sink := newPidSink()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsTreeHelper$")
	cmd.Env = append(os.Environ(), treeHelperEnv+"=parent")
	cmd.Stdout = sink
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	parent, grandchild := sink.pids(t)
	cleanupProcess(cmd)
	_ = cmd.Wait()
	requireGone(t, "child", parent)
	requireGone(t, "grandchild", grandchild)
}

func TestWindowsConfigureProcessKeepsAttributes(t *testing.T) {
	cmd := exec.Command("x")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	configureProcess(cmd)
	if !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 || cmd.Cancel == nil {
		t.Fatalf("configureProcess: %+v cancel=%v", cmd.SysProcAttr, cmd.Cancel != nil)
	}
}
