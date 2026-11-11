// testclean runs the test suite without changing its temporary directory and
// rejects new test-owned entries left in the system temp folder.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

var prefixes = []string{
	"host-test", "host-repos", "grid-allow", "grid-trust", "grid-core",
	"ready-steps", "grid-lib", "brain-read", "brain-recall", "grid-fb",
	"kill-test", "grid-env", "item-rec", "grid-cleanup-test",
	"grid-outer-", "grid-eval-",
}

// settle is how long new entries may take to disappear before they are reported.
var settle = 10 * time.Second

// pause waits between scans of the settle loop; tests replace it.
var pause = time.Sleep

func snapshot(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		for _, prefix := range prefixes {
			if strings.HasPrefix(entry.Name(), prefix) {
				names[entry.Name()] = true
				break
			}
		}
	}
	return names, nil
}

// check only observes the temp folder. Existing entries belong to earlier runs
// or other users and must never be removed by this guard.
func check(dir string, run func() int, stderr io.Writer) int {
	start := time.Now()
	before, err := snapshot(dir)
	if err != nil {
		fmt.Fprintf(stderr, "temp leak check: before: %v\n", err)
		return 1
	}
	scanTime := time.Since(start)
	status := run()
	start = time.Now()
	// Concurrent test runs of other checkouts share this folder; their folders
	// are removed within moments, so only entries that outlive the settle
	// window count as leaks.
	deadline := time.Now().Add(settle)
	var leaked []string
	// An entry seen on a scan is reported only after at least one pause and
	// rescan, whatever the clock says.
	paused := false
	for {
		after, err := snapshot(dir)
		if err != nil {
			fmt.Fprintf(stderr, "temp leak check: after: %v\n", err)
			return 1
		}
		leaked = leaked[:0]
		for name := range after {
			if !before[name] {
				leaked = append(leaked, name)
			}
		}
		if len(leaked) == 0 || (paused && !time.Now().Before(deadline)) {
			break
		}
		pause(50 * time.Millisecond)
		paused = true
	}
	slices.Sort(leaked)
	for _, name := range leaked {
		fmt.Fprintf(stderr, "temp leak check: new entry %s\n", name)
	}
	if len(leaked) > 0 {
		return 1
	}
	fmt.Fprintf(stderr, "temp leak check: no new matching entries (scans %s)\n", (scanTime + time.Since(start)).Round(time.Millisecond))
	return status
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		// Always execute tests: cached successes cannot demonstrate cleanup.
		args = []string{"go", "test", "-count=1", "./..."}
	}
	os.Exit(check(os.TempDir(), func() int {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() > 0 {
				return exit.ExitCode()
			}
			fmt.Fprintf(os.Stderr, "temp leak check: command failed: %v\n", err)
			return 1
		}
		return 0
	}, os.Stderr))
}
