//go:build windows

package proctree

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const procTreeHelperEnv = "GRID_PROCTREE_HELPER"

// TestWindowsProctreeHelper is the program the tests below start from this test
// binary. "parent" starts a leaf grandchild, prints both pids and exits at once;
// "leaf" only waits. Without the variable it does nothing, so a normal run
// passes it.
//
// The parent waits before it starts the leaf. The job is assigned after the
// parent has been created (see the package comment), so a leaf started before
// that would be outside the job; the wait keeps the test about the exited
// parent, not about that known gap.
func TestWindowsProctreeHelper(t *testing.T) {
	switch os.Getenv(procTreeHelperEnv) {
	case "parent":
		time.Sleep(500 * time.Millisecond)
		leaf := exec.Command(os.Args[0], "-test.run=^TestWindowsProctreeHelper$")
		leaf.Env = append(os.Environ(), procTreeHelperEnv+"=leaf")
		if err := leaf.Start(); err != nil {
			fmt.Println("leaf failed:", err)
			os.Exit(3)
		}
		fmt.Printf("pids %d %d\n", os.Getpid(), leaf.Process.Pid)
		os.Exit(0)
	case "leaf":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func helperCmd(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsProctreeHelper$")
	cmd.Env = append(os.Environ(), procTreeHelperEnv+"="+mode)
	return cmd
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

func TestWindowsRunEndsDescendantsOfExitedParent(t *testing.T) {
	var out bytes.Buffer
	cmd := helperCmd("parent")
	cmd.Stdout = &out
	cmd.WaitDelay = 2 * time.Second
	// The parent exits on its own with its leaf still running. After Run the
	// leaf must be gone, which cleanupProcess cannot do for an exited parent.
	if err := Run(cmd); err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("Run: %v", err)
	}
	var parent, leaf int
	if _, err := fmt.Sscanf(out.String(), "pids %d %d", &parent, &leaf); err != nil {
		t.Fatalf("helper output %q: %v", out.String(), err)
	}
	requireGone(t, "leaf", leaf)
}

func TestWindowsAttachJobAndEndJobEndALiveProcess(t *testing.T) {
	cmd := helperCmd("leaf")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := attachJob(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Skipf("this host would not put the process in a job (%v); Run goes on without one", err)
	}
	if job == 0 {
		t.Fatal("attachJob returned no handle and no error")
	}
	pid := cmd.Process.Pid
	endJob(job)
	_ = cmd.Wait()
	requireGone(t, "job member", pid)
}

func TestWindowsAttachJobNeedsAStartedProcess(t *testing.T) {
	job, err := attachJob(helperCmd("leaf"))
	if err == nil || job != 0 {
		t.Fatalf("attachJob of an unstarted command = %d, %v; want 0 and an error", job, err)
	}
}

// The values and layout below are the ones in the Windows SDK headers; a
// mismatch makes SetInformationJobObject fail or read the wrong flags.
func TestWindowsJobObjectConstantsAndLayout(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"JobObjectExtendedLimitInformation", jobObjectExtendedLimitInformation, 9},
		{"JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE", jobObjectLimitKillOnJobClose, 0x2000},
		{"PROCESS_SET_QUOTA", processSetQuota, 0x0100},
		{"PROCESS_TERMINATE", processTerminate, 0x0001},
		{"sizeof IO_COUNTERS", unsafe.Sizeof(ioCounters{}), 48},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("the 144-byte layout is the 64-bit one; GOARCH is %s", runtime.GOARCH)
	}
	var info extendedLimitInformation
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"sizeof JOBOBJECT_EXTENDED_LIMIT_INFORMATION", unsafe.Sizeof(info), 144},
		{"sizeof JOBOBJECT_BASIC_LIMIT_INFORMATION", unsafe.Sizeof(info.BasicLimitInformation), 64},
		{"offset of LimitFlags", unsafe.Offsetof(info.BasicLimitInformation.LimitFlags), 16},
		{"offset of IoInfo", unsafe.Offsetof(info.IoInfo), 64},
		{"offset of ProcessMemoryLimit", unsafe.Offsetof(info.ProcessMemoryLimit), 112},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if extendedLimitInformationSize != 144 {
		t.Errorf("extendedLimitInformationSize = %d, want 144", extendedLimitInformationSize)
	}
}
