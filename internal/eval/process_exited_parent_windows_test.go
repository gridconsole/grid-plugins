//go:build windows

package eval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

const exitedParentHelperEnv = "GRID_EVAL_EXITED_PARENT_HELPER"

// TestWindowsExitedParentHelper is the program the test below starts from this
// test binary. "parent" starts a leaf grandchild, prints both pids and exits at
// once; "leaf" only waits. Without the variable it does nothing, so a normal run
// passes it.
//
// The parent waits before it starts the leaf: the job object is assigned after
// the parent has been created, so a leaf started before that is outside the job
// (a known gap, see internal/proctree). The wait keeps this test about the
// exited parent.
func TestWindowsExitedParentHelper(t *testing.T) {
	switch os.Getenv(exitedParentHelperEnv) {
	case "parent":
		time.Sleep(500 * time.Millisecond)
		leaf := exec.Command(os.Args[0], "-test.run=^TestWindowsExitedParentHelper$")
		leaf.Env = append(os.Environ(), exitedParentHelperEnv+"=leaf")
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

// A command that exits on its own while its background child keeps running must
// not leave that child behind once RunProcess has returned. On base the child is
// still alive 10 s later, because cleanupProcess skips a command whose process
// is already over.
func TestWindowsRunProcessEndsDescendantsOfExitedParent(t *testing.T) {
	sink := newPidSink()
	env := append(os.Environ(), exitedParentHelperEnv+"=parent")
	result := runProcess(context.Background(), []string{os.Args[0], "-test.run=^TestWindowsExitedParentHelper$"}, "", env, "", sink)
	if result.ExitCode != 0 {
		t.Fatalf("the helper parent should exit 0, runProcess reported %+v", result)
	}
	parent, grandchild := sink.pids(t)
	if parent == 0 || grandchild == 0 {
		t.Fatalf("helper reported pids %d %d", parent, grandchild)
	}
	requireGone(t, "grandchild", grandchild)
}
