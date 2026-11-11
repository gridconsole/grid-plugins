package brain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testenv"
)

const helperEnv = "BRAIN_LOCK_HELPER"

// TestLockHelperProcess is the body of the child processes the tests below
// start; it does nothing in a normal run.
func TestLockHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		t.Skip("helper process only")
	}
	s := Store{GridHome: os.Getenv("BRAIN_HOME"), Project: "example"}
	marker := os.Getenv("BRAIN_MARKER")
	switch mode {
	case "propose":
		n := sample()
		n.Name = os.Getenv("BRAIN_NAME")
		touch(marker)
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
	case "hold":
		// Hold the lock until the parent kills this process.
		root, err := s.scope("project", true)
		if err != nil {
			t.Fatal(err)
		}
		_ = locked(root, func() error {
			touch(marker)
			time.Sleep(time.Hour)
			return nil
		})
	}
}

func touch(path string) { _ = os.WriteFile(path, nil, 0o600) }

type child struct {
	cmd    *exec.Cmd
	marker string
	out    *strings.Builder
}

func start(t *testing.T, s Store, mode, name string) child {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "marker")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(testenv.Temp(t, os.Environ()), helperEnv+"="+mode, "BRAIN_HOME="+s.GridHome, "BRAIN_NAME="+name, "BRAIN_MARKER="+marker)
	out := &strings.Builder{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return child{cmd, marker, out}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestTwoProcessesProposeAtOnce(t *testing.T) {
	s := testStore(t)
	root, err := s.scope("project", true)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	names := []string{"first", "second"}
	var children []child
	// Both writers start while this process holds the lock, so each has to wait
	// for it and then for the other.
	err = locked(root, func() error {
		for _, name := range names {
			children = append(children, start(t, s, "propose", name))
		}
		for _, c := range children {
			waitFor(t, "a writer to start", func() bool { return exists(c.marker) })
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("writer %s failed: %v\n%s", names[i], err, c.out)
		}
		path := filepath.Join(s.GridHome, "projects", "example", "brain-proposals", names[i]+".md")
		if !exists(path) {
			t.Fatalf("proposal %s was lost", names[i])
		}
	}
}

func TestKilledWriterLeavesNoStaleLock(t *testing.T) {
	s := testStore(t)
	holder := start(t, s, "hold", "")
	waitFor(t, "the holder to take the lock", func() bool { return exists(holder.marker) })
	if err := holder.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = holder.cmd.Wait()
	if !exists(filepath.Join(s.GridHome, "projects", "example", lockName)) {
		t.Fatal("the killed holder should leave its lock file behind")
	}
	n := sample()
	n.Name = "after-kill"
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatalf("lock left by a killed writer wedged the brain: %v", err)
	}
}

func TestLeftoverLockFileDoesNotBlock(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/"+lockName, "pid 1\n")
	n := sample()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatalf("leftover lock file blocked a proposal: %v", err)
	}
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatal(err)
	}
}
