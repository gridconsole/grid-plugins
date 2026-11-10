package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// agentTask is a task whose worker holds the task lease and has assigned J-1 to itself:
// the state an agent for J-1 starts in.
type agentTask struct {
	h        *harness
	path     string
	store    feed.Store
	ts       lease.TaskStore
	w        feed.Writer
	pid      int
	start    string
	jobFence int64
}

func newAgentTask(t *testing.T) *agentTask {
	t.Helper()
	h, path := feedHarness(t)
	pid, start := taskLeaseIdentity(t)
	a := &agentTask{h: h, path: path, store: feed.Store{TaskDir: path, TaskID: feedTaskID}, pid: pid, start: start,
		w: feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + feedMachine, Lease: "task"}}
	a.ts = lease.TaskStore{Feed: a.store}
	a.hold(t)
	if _, err := a.store.Post(feed.Input{Type: "job", Job: "J-1", Body: workerNextBody(t, "J-1")}, a.w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_MEMBER", "worker")
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", a.epoch())
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: %d %s", code, h.errb)
	}
	state, err := a.store.State()
	if err != nil || state.Jobs["J-1"].Assign == nil {
		t.Fatal(state, err)
	}
	a.jobFence = state.Jobs["J-1"].Assign.Fence
	return a
}

// hold takes the task lease for the worker and writes the handover record.
func (a *agentTask) hold(t *testing.T) {
	t.Helper()
	g, err := a.ts.TakeTask(a.w, a.pid, a.start)
	if err != nil {
		t.Fatal(err)
	}
	a.w.Fence = g.Fence
	if _, err := a.store.Handover(feed.Input{Body: map[string]any{}}, a.w); err != nil {
		t.Fatal(err)
	}
}

// retake is the worker restarting: it gives the lease back and takes it with a higher fence.
func (a *agentTask) retake(t *testing.T) {
	t.Helper()
	old := a.w.Fence
	if err := a.ts.ReleaseTask(a.w); err != nil {
		t.Fatal(err)
	}
	a.hold(t)
	if a.w.Fence <= old {
		t.Fatalf("the new fence %d should be higher than %d", a.w.Fence, old)
	}
}

// epoch is the task epoch the worker holds now, as it gives it to agents.
func (a *agentTask) epoch() string { return fmt.Sprintf("0.%d", a.w.Fence) }

// agentEnv sets the environment of an agent for J-1: its Job lease and epoch, and the task epoch it was given.
func (a *agentTask) agentEnv(t *testing.T, taskEpoch string) {
	t.Helper()
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", a.jobFence))
	t.Setenv("GRID_TASK_EPOCH", taskEpoch)
}

func (a *agentTask) postJob(t *testing.T, cwd, id string, before ...string) int {
	t.Helper()
	body, _ := json.Marshal(workerNextBody(t, id))
	args := append(before, "job", "post", "--type", "job", "--body", string(body))
	return a.h.run(cwd, args...)
}

// Row 3: a global --lease-epoch is seen by job post and GRID_TASK_EPOCH does not override it.
func TestJobPostGlobalLeaseEpochBeatsTaskEpoch(t *testing.T) {
	a := newAgentTask(t)
	first := a.epoch()
	a.retake(t)
	second := a.epoch()
	// The agent's task epoch is the stale one; the current epoch is given as the global flag.
	a.agentEnv(t, first)
	if code := a.postJob(t, a.path, "J-2", "--lease-epoch", second); code != 0 {
		t.Fatalf("the global --lease-epoch %s should be used over GRID_TASK_EPOCH %s: exit %d %s", second, first, code, a.h.errb)
	}
	// And the other way round: a stale global flag is used, so the post is refused, though GRID_TASK_EPOCH is current.
	a.agentEnv(t, second)
	if code := a.postJob(t, a.path, "J-3", "--lease-epoch", first); code != 3 {
		t.Fatalf("the stale global --lease-epoch %s should be used and refused: exit %d %s", first, code, a.h.errb)
	}
	// With no flag, GRID_TASK_EPOCH still decides.
	if code := a.postJob(t, a.path, "J-3"); code != 0 {
		t.Fatalf("GRID_TASK_EPOCH alone: exit %d %s", code, a.h.errb)
	}
	if code := a.h.run(a.path, "job", "tree", "--json"); code != 0 || !strings.Contains(a.h.out.String(), `"J-2"`) || !strings.Contains(a.h.out.String(), `"J-3"`) {
		t.Fatalf("job tree should show J-2 and J-3: %d %s %s", code, a.h.out, a.h.errb)
	}
}

// postFromAgentCwd posts J-2 from a folder with no --task and checks the Job reached the task.
func (a *agentTask) postFromAgentCwd(t *testing.T, cwd string) {
	t.Helper()
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := a.postJob(t, cwd, "J-2"); code != 0 {
		t.Fatalf("job post from %s with no --task: exit %d %s", cwd, code, a.h.errb)
	}
	if code := a.h.run(a.path, "job", "tree", "--json"); code != 0 || !strings.Contains(a.h.out.String(), `"J-2"`) {
		t.Fatalf("job tree should show J-2: %d %s %s", code, a.h.out, a.h.errb)
	}
}

// Row 4: with only GRID_TASK_EPOCH, GRID_LEASE and GRID_LEASE_EPOCH, the task comes from a cwd inside
// its folder, and the task epoch is the lease. (This held before the lane.)
func TestJobPostResolvesTaskFromCwdInTaskFolder(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	a.postFromAgentCwd(t, filepath.Join(a.path, "worktrees", "J-1"))
}

// Row 4: an agent's git worktree is under <home>/projects/<project>/worktrees, outside the task folder, so
// the cwd names no task; the worker gives the agent GRID_FEED_TASK, and job post reads it as grid feed does.
func TestJobPostResolvesTaskFromAgentWorktree(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	t.Setenv("GRID_FEED_TASK", a.path)
	a.postFromAgentCwd(t, filepath.Join(a.h.gridHome, "projects", "example", "worktrees", "J-1"))
}

// An explicit --task wins over the session's GRID_FEED_TASK.
func TestJobPostTaskFlagBeatsFeedTaskEnv(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	t.Setenv("GRID_FEED_TASK", filepath.Join(a.h.gridHome, "projects", "example", "tasks", "T-99-missing"))
	body, _ := json.Marshal(workerNextBody(t, "J-2"))
	if code := a.h.run(a.h.home, "job", "post", "--task", a.path, "--type", "job", "--body", string(body)); code != 0 {
		t.Fatalf("--task should win over GRID_FEED_TASK: exit %d %s", code, a.h.errb)
	}
}
