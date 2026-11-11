//go:build windows

package land

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const (
	landTreeHelperEnv  = "GRID_LAND_TREE_HELPER"
	landTreePidFileEnv = "GRID_LAND_TREE_PIDFILE"
)

// TestWindowsLandTreeHelper is the program the tree tests start from this test
// binary. "parent" starts a grandchild and writes both pids to the file named by
// the environment; "leaf" only waits. runProcess keeps output until the process
// ends, so the pids travel through a file. Without the variable it does nothing,
// so a normal run passes it.
func TestWindowsLandTreeHelper(t *testing.T) {
	switch os.Getenv(landTreeHelperEnv) {
	case "parent":
		grandchild := exec.Command(os.Args[0], "-test.run=^TestWindowsLandTreeHelper$")
		grandchild.Env = append(os.Environ(), landTreeHelperEnv+"=leaf")
		if err := grandchild.Start(); err != nil {
			fmt.Println("grandchild failed:", err)
			os.Exit(3)
		}
		file := os.Getenv(landTreePidFileEnv)
		// Rename makes the file appear with both pids in it.
		if err := os.WriteFile(file+".tmp", []byte(fmt.Sprintf("pids %d %d\n", os.Getpid(), grandchild.Process.Pid)), 0o600); err != nil {
			os.Exit(4)
		}
		if err := os.Rename(file+".tmp", file); err != nil {
			os.Exit(5)
		}
		time.Sleep(time.Minute)
		os.Exit(0)
	case "leaf":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func landTreeArgv() []string {
	return []string{os.Args[0], "-test.run=^TestWindowsLandTreeHelper$"}
}

func landTreeEnv(pidFile string) []string {
	return append(os.Environ(), landTreeHelperEnv+"=parent", landTreePidFileEnv+"="+pidFile)
}

// treePids waits for the helper parent to write its pid file.
func treePids(t *testing.T, pidFile string) (parent, grandchild int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if data, err := os.ReadFile(pidFile); err == nil {
			if _, err := fmt.Sscanf(string(data), "pids %d %d", &parent, &grandchild); err != nil {
				t.Fatalf("helper pid file %q: %v", data, err)
			}
			return parent, grandchild
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never reported its pids")
		}
		time.Sleep(50 * time.Millisecond)
	}
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

func TestWindowsCancelKillsProcessTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pidFile := filepath.Join(t.TempDir(), "pids")
	done := make(chan struct {
		status, errText string
	}, 1)
	go func() {
		result := runProcess(ctx, landTreeArgv(), "", landTreeEnv(pidFile))
		done <- struct{ status, errText string }{result.Status, result.Error}
	}()
	parent, grandchild := treePids(t, pidFile)
	cancel()
	select {
	case result := <-done:
		if result.status == "passed" || result.errText != context.Canceled.Error() {
			t.Fatalf("cancelled run reported status %q error %q", result.status, result.errText)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runProcess did not return within 10 s of cancel")
	}
	requireGone(t, "child", parent)
	requireGone(t, "grandchild", grandchild)
}

func TestWindowsCleanupKillsProcessTree(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pids")
	cmd := exec.Command(landTreeArgv()[0], landTreeArgv()[1:]...)
	cmd.Env = landTreeEnv(pidFile)
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	parent, grandchild := treePids(t, pidFile)
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

func TestWindowsCleanupSkipsUnstartedAndFinishedCommands(t *testing.T) {
	cleanupProcess(exec.Command("x"))
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	configureProcess(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	// ProcessState is set, so the pid is never used for a kill.
	cleanupProcess(cmd)
}
