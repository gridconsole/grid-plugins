//go:build windows

package land

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const landExitedParentHelperEnv = "GRID_LAND_EXITED_PARENT_HELPER"

// TestWindowsLandExitedParentHelper is the program the test below starts from
// this test binary. "parent" starts a leaf grandchild, prints both pids and exits
// at once; "leaf" only waits. Without the variable it does nothing, so a normal
// run passes it.
//
// The parent waits before it starts the leaf: the job object is assigned after
// the parent has been created, so a leaf started before that is outside the job
// (a known gap, see internal/proctree). The wait keeps this test about the
// exited parent.
func TestWindowsLandExitedParentHelper(t *testing.T) {
	switch os.Getenv(landExitedParentHelperEnv) {
	case "parent":
		time.Sleep(500 * time.Millisecond)
		leaf := exec.Command(os.Args[0], "-test.run=^TestWindowsLandExitedParentHelper$")
		leaf.Env = append(os.Environ(), landExitedParentHelperEnv+"=leaf")
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

// A land command that exits on its own while its background child keeps running
// must not leave that child behind once runProcess has returned. On base the
// child is still alive 10 s later, because cleanupProcess skips a command whose
// process is already over. The command has exited by the time runProcess returns,
// so its stdout in the result holds both pids.
func TestWindowsRunProcessEndsDescendantsOfExitedParent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	env := append(os.Environ(), landExitedParentHelperEnv+"=parent")
	result := runProcess(ctx, []string{os.Args[0], "-test.run=^TestWindowsLandExitedParentHelper$"}, "", env)
	if result.ExitCode != 0 {
		t.Fatalf("the helper parent should exit 0, runProcess reported %+v", result)
	}
	var parent, grandchild int
	for _, line := range strings.Split(result.Stdout, "\n") {
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "pids %d %d", &parent, &grandchild); err == nil {
			break
		}
	}
	if parent == 0 || grandchild == 0 {
		t.Fatalf("helper stdout %q holds no pids", result.Stdout)
	}
	requireGone(t, "grandchild", grandchild)
}
