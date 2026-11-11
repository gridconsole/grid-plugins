//go:build unix

package land

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

// jpLdWait bounds every wait in these tests. It never proves anything: a wait
// that runs out fails by the name of what it waited for.
const jpLdWait = 30 * time.Second

// jpLdKillPidFile ends the process whose pid the file holds, if it holds one.
// It is cleanup only, never a probe.
func jpLdKillPidFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// A deploy command that leaves a background process holding its output is
// judged on the shell's exit code; the background process ends with the group.
func TestJpLdBackgroundChild(t *testing.T) {
	s, dir, repo := fixture(t)
	fifo := filepath.Join(t.TempDir(), "held")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	// The read end is open before the run, so the shell's exec 3> does not block.
	held, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	t.Cleanup(func() { jpLdKillPidFile(filepath.Join(repo, "bg.pid")) })
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	write(t, path, "---\nlands:\n  target: local\n  branch: main\n  command: \"exec 3>'"+fifo+"'; sleep 30 & echo $! > bg.pid; echo ok\"\n  needs-approve: true\n---\n")
	approve(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := s.Run(ctx, repo, "T-01", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "landed" || result.Execution == nil || result.Execution.ExitCode != 0 || result.Execution.Stdout != "ok\n" {
		t.Fatalf("result %+v execution %+v", result, result.Execution)
	}
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "land landed") {
		t.Fatalf("missing landed history: %s", b)
	}
	// EOF on the FIFO means no process holds its write end: the sleep is gone.
	read := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(held)
		read <- err
	}()
	select {
	case err := <-read:
		if err != nil {
			t.Fatalf("read FIFO: %v", err)
		}
	case <-time.After(jpLdWait):
		t.Fatal("background sleep still holds the FIFO")
	}
}

// Output larger than a read is read to the end.
func TestJpLdFullOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	script := `i=1; while [ "$i" -le 3000 ]; do echo "line $i"; i=$((i+1)); done`
	result := runProcess(ctx, []string{"sh", "-c", script}, t.TempDir(), allowedEnvironment(nil))
	var want strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&want, "line %d\n", i)
	}
	if result.Status != "passed" || result.Truncated || result.Stdout != want.String() {
		t.Fatalf("status=%s truncated=%v lines=%d bytes=%d want lines=3000 bytes=%d", result.Status, result.Truncated, strings.Count(result.Stdout, "\n"), len(result.Stdout), want.Len())
	}
}

// TestJpLdHelper is the helper process of TestJpLdContextEnds. Run as a plain
// test it has no arguments and returns at once.
func TestJpLdHelper(t *testing.T) {
	args := flag.Args()
	if len(args) != 2 || args[0] != "jp-ld-setsid" {
		return
	}
	// The sleep leaves the group in its own session and keeps this stdout.
	sleep := exec.Command("sleep", "600")
	sleep.Stdout, sleep.Stderr = os.Stdout, os.Stderr
	sleep.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := sleep.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	marker := args[1]
	if err := os.WriteFile(marker+".tmp", []byte(strconv.Itoa(sleep.Process.Pid)+"\n"), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.Rename(marker+".tmp", marker); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("ok")
	os.Exit(0)
}

// A descendant outside the group that keeps the output open is cut off when
// ctx ends.
func TestJpLdContextEnds(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	t.Cleanup(func() { jpLdKillPidFile(marker) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := "exec '" + exe + "' -test.run='^TestJpLdHelper$' -- jp-ld-setsid '" + marker + "'"
	done := make(chan eval.ProcessResult, 1)
	go func() { done <- runProcess(ctx, []string{"sh", "-c", command}, dir, allowedEnvironment(nil)) }()
	written := make(chan struct{})
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			if _, err := os.Stat(marker); err == nil {
				close(written)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	select {
	case <-written:
	case result := <-done:
		t.Fatalf("runProcess returned before the helper wrote the marker: %+v", result)
	case <-time.After(jpLdWait):
		t.Fatal("helper never wrote the marker")
	}
	select {
	case result := <-done:
		t.Fatalf("runProcess returned while the setsid sleep held stdout: %+v", result)
	default:
	}
	cancel()
	select {
	case result := <-done:
		if result.Status != "error" || result.Error != context.Canceled.Error() {
			t.Fatalf("result %+v", result)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("ctx %v", ctx.Err())
		}
	case <-time.After(jpLdWait):
		t.Fatal("runProcess did not return after cancel")
	}
}
