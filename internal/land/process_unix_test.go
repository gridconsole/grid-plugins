//go:build linux || darwin

package land

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gtLandReadyCtx ends the run when the background child has started: its Done
// channel closes once ready exists and bg.pid holds a whole line, and Err then
// reports a deadline. The test owns the deadline, so no timer decides it.
type gtLandReadyCtx struct {
	context.Context
	done chan struct{}
}

func (c *gtLandReadyCtx) Done() <-chan struct{} { return c.done }

func (c *gtLandReadyCtx) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func gtLandPid(dir string) (int, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "bg.pid"))
	if err != nil || !strings.HasSuffix(string(b), "\n") {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid, err == nil && pid > 0
}

func gtLandReady(t *testing.T, dir string) *gtLandReadyCtx {
	c := &gtLandReadyCtx{Context: context.Background(), done: make(chan struct{})}
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
				if _, ok := gtLandPid(dir); ok {
					close(c.done)
					return
				}
			}
			select {
			case <-stop:
				return
			case <-tick.C:
			}
		}
	}()
	return c
}

func TestTimeoutKillsDescendants(t *testing.T) {
	s, dir, repo := fixture(t)
	// The test ends the run itself; this timeout is never the proof.
	s.Timeout = 60 * time.Second
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	// The background shell ignores TERM and survives killing only its parent.
	write(t, path, "---\nlands:\n  target: local\n  branch: main\n  command: \"(trap '' TERM; printf ready > ready; sleep 30; printf survived > survived) & echo $! > bg.pid; wait\"\n  needs-approve: true\n---\n")
	approve(t, s)
	result, err := s.Run(gtLandReady(t, repo), repo, "T-01", false)
	if err == nil || result.Status != "timed-out" {
		t.Fatalf("result %+v: %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "ready")); err != nil {
		t.Fatalf("child never started: %v", err)
	}
	pid, ok := gtLandPid(repo)
	if !ok {
		t.Fatal("child never started: no bg.pid")
	}
	gone := make(chan struct{})
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for syscall.Kill(pid, 0) == nil {
			<-tick.C
		}
		close(gone)
	}()
	select {
	case <-gone:
	case <-time.After(30 * time.Second):
		t.Fatal("descendant still alive")
	}
	if _, err := os.Stat(filepath.Join(repo, "survived")); !os.IsNotExist(err) {
		t.Fatal("descendant survived timeout")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if !strings.Contains(string(b), "land timed-out") {
		t.Fatalf("missing timeout history: %s", b)
	}
}

func TestProcessBoundsOutput(t *testing.T) {
	result := runProcess(context.Background(), []string{"sh", "-c", "head -c 131072 /dev/zero; head -c 131072 /dev/zero >&2"}, t.TempDir(), allowedEnvironment(nil))
	if result.Status != "passed" || !result.Truncated || len(result.Stdout) != outputLimit || len(result.Stderr) != outputLimit {
		t.Fatalf("bounded result: status=%s truncated=%v stdout=%d stderr=%d", result.Status, result.Truncated, len(result.Stdout), len(result.Stderr))
	}
}
