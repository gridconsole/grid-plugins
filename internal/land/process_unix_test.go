//go:build linux || darwin

package land

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimeoutKillsDescendants(t *testing.T) {
	s, dir, repo := fixture(t)
	s.Timeout = 100 * time.Millisecond
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	// The background shell ignores TERM and survives killing only its parent.
	write(t, path, "---\nlands:\n  target: local\n  branch: main\n  command: \"(trap '' TERM; printf ready > ready; sleep 0.4; printf survived > survived) & wait\"\n  needs-approve: true\n---\n")
	approve(t, s)
	result, err := s.Run(context.Background(), repo, "T-01", false)
	if err == nil || result.Status != "timed-out" {
		t.Fatalf("result %+v: %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "ready")); err != nil {
		t.Fatalf("child never started: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
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
