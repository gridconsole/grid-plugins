//go:build windows

package job

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const verifyExitedParentHelperEnv = "GRID_JOB_VERIFY_EXITED_PARENT_HELPER"

// TestWindowsVerifyExitedParentHelper is the program the test below starts from
// this test binary. "parent" starts a leaf grandchild, prints both pids and exits
// at once; "leaf" only waits. Without the variable it does nothing, so a normal
// run passes it.
//
// The parent waits before it starts the leaf: the job object is assigned after
// the shell has been created, so a leaf started before that is outside the job
// (a known gap, see internal/proctree). The wait keeps this test about the
// exited parent.
func TestWindowsVerifyExitedParentHelper(t *testing.T) {
	switch os.Getenv(verifyExitedParentHelperEnv) {
	case "parent":
		time.Sleep(500 * time.Millisecond)
		leaf := exec.Command(os.Args[0], "-test.run=^TestWindowsVerifyExitedParentHelper$")
		leaf.Env = append(os.Environ(), verifyExitedParentHelperEnv+"=leaf")
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

// A verification command whose shell exits while a background child keeps
// running must not leave that child behind once RunVerify has returned. On base
// the child is still alive 10 s later, because cleanupVerifyProcess skips a
// command whose process is already over. The pids come from the helper's output
// in the result, so the test goes through verifyShell and the run path of
// RunVerify and nothing else.
func TestWindowsRunVerifyEndsDescendantsOfExitedParent(t *testing.T) {
	t.Setenv(verifyExitedParentHelperEnv, "parent")
	// No cmd.exe metacharacters in the test name, so no quoting beyond the path.
	command := `"` + os.Args[0] + `" -test.run=TestWindowsVerifyExitedParentHelper`
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// The error is not asserted: a child that kept the output open for longer
	// than WaitDelay makes Wait report that, which is not what this test is for.
	r, _ := RunVerify(ctx, Job{Verify: Verify{Command: command}}, t.TempDir())
	if r.ExitCode != 0 {
		t.Fatalf("the shell should exit 0, RunVerify reported %+v", r)
	}
	var parent, grandchild int
	for _, line := range strings.Split(r.Output, "\n") {
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "pids %d %d", &parent, &grandchild); err == nil {
			break
		}
	}
	if parent == 0 || grandchild == 0 {
		t.Fatalf("helper output %q holds no pids", r.Output)
	}
	requireGone(t, "grandchild", grandchild)
}
