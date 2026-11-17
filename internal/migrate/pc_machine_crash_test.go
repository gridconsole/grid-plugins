package migrate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
	"github.com/gridconsole/grid-plugins/internal/testenv"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// TestCrashHelperMachine is TestCrashHelper with a machine folder: a copy of the test
// binary that applies the migration and exits, without a defer, after one step.
func TestCrashHelperMachine(t *testing.T) {
	step := os.Getenv("GRID_CRASH_STEP")
	if step == "" {
		t.Skip("entry point of the machine folder crash tests")
	}
	afterStep = func(s string) {
		if s == step {
			os.Exit(crashExit)
		}
	}
	folder := machinefolder.Folder{Path: os.Getenv("GRID_CRASH_MACHINE"), Source: machinefolder.SourceEnv}
	p, err := Scan(os.Getenv("GRID_CRASH_FROM"), os.Getenv("GRID_CRASH_HOME"), WithMachineFolder(folder))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
}

// A run that dies at any step of the move leaves a state the next run finishes: the
// worktrees end in the machine folder, git names them there, no stage folder is left,
// and the dirty files are intact.
func TestMachineFolderCrashMatrix(t *testing.T) {
	for _, step := range []string{"worktrees-staged", "journal-publishing", "displaced", "published", "worktrees-placed", "worktree-rewritten", "receipt-written", "previous-removed"} {
		t.Run(step, func(t *testing.T) {
			from, repo, machine, folder := machineFixture(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelperMachine$", "-test.count=1")
			cmd.Env = append(testenv.Temp(t, os.Environ()), "GRID_CRASH_STEP="+step, "GRID_CRASH_FROM="+from, "GRID_CRASH_HOME="+from, "GRID_CRASH_MACHINE="+machine)
			out, err := cmd.CombinedOutput()
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != crashExit {
				t.Fatalf("the run did not stop after %q: %v\n%s", step, err, out)
			}
			finishAfterCrash(t, from, from, WithMachineFolder(folder))
			listing := testgit.Run(t, repo, "worktree", "list", "--porcelain")
			for _, w := range machineWorkers {
				dir := machineChild(machine, w)
				if !strings.Contains(listing, "worktree "+dir+"\n") {
					t.Fatalf("git does not name %s:\n%s", dir, listing)
				}
				if testgit.Run(t, dir, "status", "--porcelain") != "?? untracked.txt" || read(t, dir, "untracked.txt") != "keep dirty work\r\n" {
					t.Fatalf("%s: worktree data lost", dir)
				}
			}
			entries, err := os.ReadDir(filepath.Join(machine, "worktrees"))
			if err != nil || len(entries) != 1 || entries[0].Name() != "widget" {
				t.Fatalf("machine worktrees folder holds %v (%v)", entries, err)
			}
			absent(t, journalPath(from))
		})
	}
}
