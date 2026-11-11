package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

// ihClIsolate points HOME, GRID_HOME, CLAUDE_CONFIG_DIR and CODEX_HOME at the
// harness's own folders, so nothing read through the process environment
// leaves the test's temp home.
func ihClIsolate(t *testing.T, h *harness) {
	t.Helper()
	t.Setenv("HOME", h.home)
	t.Setenv("GRID_HOME", h.gridHome)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
}

// ihClCancelled is a context that is already done.
func ihClCancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// With Env.Context set, the streams stop on it and exit 130 (worker.go and
// orchestrator.go on context.Canceled); the helper behind them installs no
// signal handler then, and installs one only when Context is nil.
func TestIhClStreamStopsOnContext(t *testing.T) {
	h, first := feedHarness(t)
	ihClIsolate(t, h)
	ownerPost(t, first, feedTaskID, "J-1")
	pid, start := taskLeaseIdentity(t)
	h.env.Context = ihClCancelled()
	if code := h.run(first, "--member", "worker", "--role", "worker", "worker", "next", "--json",
		"--pid", strconv.Itoa(pid), "--pid-start", start); code != 130 {
		t.Fatalf("worker next with a cancelled Context: %d, stdout %q, stderr %q", code, h.out, h.errb)
	}
	if h.out.Len() != 0 {
		t.Fatalf("worker next with a cancelled Context spawned: %s", h.out)
	}

	o, path, _ := orchestratorHarness(t)
	if code := o.run(path, "job", "next"); code != 0 {
		t.Fatalf("job next: %d %s", code, o.errb)
	}
	o.env.Context = ihClCancelled()
	if code := o.run(path, "job", "next", "--json", "--wait"); code != 130 {
		t.Fatalf("job next --wait with nothing to assign and a cancelled Context: %d, stdout %q, stderr %q", code, o.out, o.errb)
	}

	parent, stopParent := context.WithCancel(context.Background())
	ctx, cancel := Env{Context: parent}.signalContext(os.Interrupt)
	defer cancel()
	if s := fmt.Sprint(ctx); strings.Contains(s, "signal.NotifyContext") {
		t.Fatalf("a set Context still installed a signal handler: %s", s)
	}
	stopParent()
	<-ctx.Done()
	ctx, cancel = Env{}.signalContext(os.Interrupt)
	s := fmt.Sprint(ctx)
	cancel()
	if !strings.Contains(s, "signal.NotifyContext") {
		t.Fatalf("a nil Context did not install the binary's signal handler: %s", s)
	}
}

// eval run and job verify, reached through cli.Run, stop on a cancelled
// Env.Context passed on to their own Env (eval.go, jobcmdEnv in job.go).
func TestIhClSubEnvContext(t *testing.T) {
	h := newHarness(t)
	ihClIsolate(t, h)
	t.Setenv("TMPDIR", h.tmp)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	agent := `["sh","-c","cat >/dev/null; printf '# Release notes\\n- Fix empty search results\\n- Add CSV export\\n' > RELEASE.md"]`
	args := []string{"eval", "run", "release-notes", "--agent", agent, "--json"}
	// A live, set Context runs the task to its end, as the handler path does
	// (evalcmd's own TestCommandRunsConfiguredAgent-style run gives 0).
	h.env.Context = context.Background()
	if code := h.run(root, append(args, "--results", t.TempDir())...); code != 0 {
		t.Fatalf("eval run with a live Context: %d %s %s", code, h.out, h.errb)
	}
	// A cancelled one fails the agent and the checks at their ctx.Err checks
	// (eval/process.go, eval/check.go): status is not passed, so exit 1.
	h.env.Context = ihClCancelled()
	code := h.run(root, append(args, "--results", t.TempDir())...)
	var runs []eval.Result
	_ = json.Unmarshal(h.out.Bytes(), &runs)
	if code != 1 {
		t.Fatalf("eval run with a cancelled Context: %d %s %s", code, h.out, h.errb)
	}
	for _, r := range runs {
		if r.Status == "passed" {
			t.Fatalf("eval run with a cancelled Context passed: %s", h.out)
		}
	}

	f, task := feedHarness(t)
	ihClIsolate(t, f)
	f.write(filepath.Join(task, "jobs", "J-1", "job.yaml"), "id: J-1\nverify:\n  kind: reviewer-check\n")
	f.env.Context = ihClCancelled()
	// RunVerify returns ctx.Err first (job/verify.go), and verify.go exits 1 on an error.
	if code := f.run(task, "job", "verify", "J-1", "--task", task, "--json"); code != 1 || !strings.Contains(f.out.String(), "context canceled") {
		t.Fatalf("job verify with a cancelled Context: %d %s %s", code, f.out, f.errb)
	}
}
