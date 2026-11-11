package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// ihClHome is newHome with HOME, GRID_HOME, CLAUDE_CONFIG_DIR and CODEX_HOME
// of the process under the test's temp folder too, so no host home is read.
func ihClHome(t *testing.T, title string) home {
	t.Helper()
	root := realPath(t, t.TempDir())
	t.Setenv("HOME", filepath.Join(root, "host"))
	t.Setenv("GRID_HOME", filepath.Join(root, "host", ".grid"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "host", ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "host", ".codex"))
	return newHome(t, title)
}

// ihClEnviron is the fixture's environment with the four homes named.
func ihClEnviron(h home) []string {
	return h.environ("GRID_HOME="+h.grid, "CLAUDE_CONFIG_DIR="+filepath.Join(h.root, "home", ".claude"),
		"CODEX_HOME="+filepath.Join(h.root, "home", ".codex"))
}

// ihClRun makes the call c in dir with the fixture's home and environment.
func ihClRun(h home, dir string, c Call) (int, string, string) {
	var out, errb bytes.Buffer
	c.GridHome, c.Dir, c.Environ, c.Stdout, c.Stderr = h.grid, dir, ihClEnviron(h), &out, &errb
	code := Run(c)
	return code, out.String(), errb.String()
}

// ihClLease reads the task lease's pid and pid_start in dir.
func ihClLease(t *testing.T, h home, dir string) (int, string) {
	t.Helper()
	code, out, errs := ihClRun(h, dir, Call{Args: []string{"lease", "status", "task", "--json"}})
	var st struct {
		PID      *int    `json:"pid"`
		PIDStart *string `json:"pid_start"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || st.PID == nil || st.PIDStart == nil {
		t.Fatalf("lease status: %d %s %s", code, out, errs)
	}
	return *st.PID, *st.PIDStart
}

// ihClStart is the process start of pid in the lease's time shape.
func ihClStart(t *testing.T, pid int) time.Time {
	t.Helper()
	start, err := session.ProcessStart(pid)
	if err != nil {
		t.Fatal(err)
	}
	return start
}

func TestIhClNoLockLeftAfterCancel(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("worker next needs Unix process identity (worker.go probes --pid)")
	}
	h := ihClHome(t, "Cancel a stream")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pid := os.Getpid()
	worker := Call{Context: ctx, Args: []string{"--member", "worker", "--role", "worker", "worker", "next", "--json",
		"--pid", strconv.Itoa(pid), "--pid-start", contract.FormatTime(ihClStart(t, pid))}}
	if code, out, errs := ihClRun(h, h.task, worker); code != 130 {
		t.Fatalf("worker next with a cancelled Context: %d %s %s", code, out, errs)
	}
	// job next runs under the task lease: hold it, assign the first Job, then wait with nothing to assign.
	if code, out, errs := ihClRun(h, h.task, Call{Args: []string{"lease", "take", "task", "--hold", "--json"}}); code != 0 {
		t.Fatalf("lease take --hold: %d %s %s", code, out, errs)
	}
	_, out, _ := ihClRun(h, h.task, Call{Args: []string{"lease", "status", "task", "--json"}})
	var held struct{ Gen, Fence int64 }
	if err := json.Unmarshal([]byte(out), &held); err != nil {
		t.Fatalf("lease status: %v %s", err, out)
	}
	epoch := strconv.FormatInt(held.Gen, 10) + "." + strconv.FormatInt(held.Fence, 10)
	if code, out, errs := ihClRun(h, h.task, Call{Args: []string{"--lease-epoch", epoch, "job", "next", "--json"}}); code != 0 {
		t.Fatalf("job next: %d %s %s", code, out, errs)
	}
	if code, out, errs := ihClRun(h, h.task, Call{Context: ctx, Args: []string{"--lease-epoch", epoch, "job", "next", "--json", "--wait"}}); code != 130 {
		t.Fatalf("job next --wait with a cancelled Context: %d %s %s", code, out, errs)
	}
	if code, out, errs := ihClRun(h, h.task, Call{Args: []string{"lease", "release", "task", "--epoch", epoch, "--json"}}); code != 0 {
		t.Fatalf("lease release: %d %s %s", code, out, errs)
	}
	_, before, _ := ihClRun(h, h.task, Call{Args: []string{"feed", "state", "--json"}})
	if code, out, errs := h.post("after the cancelled streams", ihClEnviron(h)); code != 0 {
		t.Fatalf("feed post after cancel: %d %s %s", code, out, errs)
	}
	code, after, errs := ihClRun(h, h.task, Call{Args: []string{"feed", "state", "--json"}})
	if code != 0 || after == before {
		t.Fatalf("feed state after the post: %d, unchanged %t\n%s %s", code, after == before, after, errs)
	}
	if code, out, errs := ihClRun(h, h.task, Call{Args: []string{"feed", "read", "--json"}}); code != 0 || !strings.Contains(out, "after the cancelled streams") {
		t.Fatalf("feed read after the post: %d %s %s", code, out, errs)
	}
}

// task new holds the task lease only while it posts the first Job and releases
// it before it returns (task_new_first_job.go, defer ts.ReleaseTask), so after
// the call the lease is free with no pid whatever the owner: the owner it
// recorded cannot be read back here. The test shows the owner fields are taken
// (both forms succeed) and leave the lease free as the binary does.
func TestIhClOwnerOnTaskNew(t *testing.T) {
	h := ihClHome(t, "Owner of the binary")
	free := func(dir string) {
		t.Helper()
		code, out, errs := ihClRun(h, dir, Call{Args: []string{"lease", "status", "task", "--json"}})
		var st struct {
			Holder *string `json:"holder"`
			PID    *int    `json:"pid"`
		}
		if code != 0 || json.Unmarshal([]byte(out), &st) != nil || st.Holder != nil || st.PID != nil {
			t.Fatalf("lease after task new is not free: %d %s %s", code, out, errs)
		}
	}
	free(h.task)
	owner := os.Getppid()
	start := ihClStart(t, owner)
	newTask := func(title string, c Call) string {
		t.Helper()
		c.Args = []string{"task", "new", title, "--project", "example", "--json"}
		code, out, errs := ihClRun(h, h.folder, c)
		var made struct{ Path string }
		if code != 0 || json.Unmarshal([]byte(out), &made) != nil || made.Path == "" {
			t.Fatalf("task new %q: %d %s %s", title, code, out, errs)
		}
		return made.Path
	}
	if migrate.ProcessStart(owner) == "" {
		t.Fatalf("no start for the owner %d", owner)
	}
	free(newTask("Owner and start", Call{OwnerPID: owner, OwnerStart: start}))
	free(newTask("Owner without start", Call{OwnerPID: owner}))
}

func TestIhClOwnerOnHold(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("lease take is refused off Linux and macOS before --hold is read (internal/cli/task_lease.go:75-77)")
	}
	h := ihClHome(t, "Hold for an owner")
	// The test's own pid: alive, and not the os.Getppid the binary path records.
	owner := os.Getpid()
	refused := Call{OwnerPID: owner, Args: []string{"lease", "take", "task", "--hold", "--json", "--pid", strconv.Itoa(owner)}}
	if code, _, errs := ihClRun(h, h.task, refused); code != 2 || !strings.Contains(errs, "--hold cannot be combined with --session, --pid or --pid-start") {
		t.Fatalf("--hold with --pid and an owner: %d %s", code, errs)
	}
	if code, out, errs := ihClRun(h, h.task, Call{OwnerPID: owner, Args: []string{"lease", "take", "task", "--hold", "--json"}}); code != 0 {
		t.Fatalf("lease take --hold with an owner: %d %s %s", code, out, errs)
	}
	// OwnerStart zero: the start is looked up for the owner (session.ProcessStart), never empty.
	pid, start := ihClLease(t, h, h.task)
	if pid != owner || start == "" || start != contract.FormatTime(ihClStart(t, owner)) {
		t.Fatalf("held lease: pid %d start %q, want owner %d (not os.Getppid %d)", pid, start, owner, os.Getppid())
	}
	// A given OwnerStart is written as it is, in the contract's time shape.
	_, out, _ := ihClRun(h, h.task, Call{Args: []string{"lease", "status", "task", "--json"}})
	var held struct{ Gen, Fence int64 }
	if err := json.Unmarshal([]byte(out), &held); err != nil {
		t.Fatal(err)
	}
	epoch := strconv.FormatInt(held.Gen, 10) + "." + strconv.FormatInt(held.Fence, 10)
	if code, out, errs := ihClRun(h, h.task, Call{Args: []string{"lease", "release", "task", "--epoch", epoch, "--json"}}); code != 0 {
		t.Fatalf("lease release: %d %s %s", code, out, errs)
	}
	given := ihClStart(t, owner)
	if code, out, errs := ihClRun(h, h.task, Call{OwnerPID: owner, OwnerStart: given, Args: []string{"lease", "take", "task", "--hold", "--json"}}); code != 0 {
		t.Fatalf("lease take --hold with an owner and start: %d %s %s", code, out, errs)
	}
	if pid, start = ihClLease(t, h, h.task); pid != owner || start != contract.FormatTime(given) {
		t.Fatalf("held lease with a given start: pid %d start %q, want %d %q", pid, start, owner, contract.FormatTime(given))
	}
}

func TestIhClDocNamesFive(t *testing.T) {
	src, err := os.ReadFile("lib.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	pkgDoc, rest, ok := strings.Cut(text, "\npackage lib\n")
	runDoc, _, ok2 := strings.Cut(rest[strings.Index(rest, "// Run executes"):], "\nfunc Run(")
	if !ok || !ok2 {
		t.Fatal("lib.go has no package doc or Run doc")
	}
	for name, doc := range map[string]string{"package doc": pkgDoc, "Run doc": runDoc} {
		if !strings.Contains(strings.Join(strings.Fields(strings.ReplaceAll(doc, "//", "")), " "), "5 taken") {
			t.Errorf("the %s does not name exit 5 taken", name)
		}
	}
}
