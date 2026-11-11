package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// cancelJob runs `grid job cancel` for J-3 as the agent's cwd-less call: --task is typed, the Job and reason too.
func (a *agentTask) cancelJob(t *testing.T, id string, before ...string) int {
	t.Helper()
	args := append(before, "job", "cancel", "--task", a.path, "--job", id, "--reason", "x")
	return a.h.run(a.path, args...)
}

// lastControl is the newest control record of the task.
func (a *agentTask) lastControl(t *testing.T) feed.Record {
	t.Helper()
	r, err := a.store.Read(feed.Filter{Type: "control"})
	if err != nil || len(r.Records) == 0 {
		t.Fatalf("control records: %v %v", r.Records, err)
	}
	return r.Records[len(r.Records)-1].Record
}

func (a *agentTask) status(t *testing.T, id string) contract.Status {
	t.Helper()
	state, err := a.store.State()
	if err != nil || state.Jobs[id] == nil {
		t.Fatalf("state of %s: %v %v", id, state.Jobs[id], err)
	}
	return state.Jobs[id].Status
}

// A Job agent holds GRID_LEASE=J-1 with its own Job epoch, and GRID_TASK_EPOCH for
// the task. Cancelling a Job is a task write, like posting one, so it carries the
// task epoch; on main it sent the Job epoch against the task lease and exit 3.
func TestJobCancelUsesTaskEpochForJobAgent(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	if code := a.postJob(t, a.path, "J-3"); code != 0 {
		t.Fatalf("post J-3: exit %d %s", code, a.h.errb)
	}
	if code := a.cancelJob(t, "J-3"); code != 0 {
		t.Fatalf("job cancel by a Job agent with GRID_TASK_EPOCH: exit %d %s", code, a.h.errb)
	}
	if got := a.status(t, "J-3"); got != contract.Cancelled {
		t.Fatalf("J-3 is %q, want cancelled", got)
	}
	c := a.lastControl(t)
	if c.Lease == nil || *c.Lease != "task" || c.Fence != a.w.Fence {
		t.Fatalf("the cancel should be written under the task lease at fence %d: lease %v fence %d", a.w.Fence, c.Lease, c.Fence)
	}
}

// Typed flags still win over GRID_TASK_EPOCH, as for job post: a typed --lease-epoch
// (global, before the command, or after it) is used even when it is the stale one,
// and a typed current one is used even when GRID_TASK_EPOCH is the stale one.
func TestJobCancelTypedLeaseEpochBeatsTaskEpoch(t *testing.T) {
	a := newAgentTask(t)
	first := a.epoch()
	a.retake(t)
	second := a.epoch()
	a.agentEnv(t, second)
	for _, id := range []string{"J-3", "J-4", "J-5"} {
		if code := a.postJob(t, a.path, id); code != 0 {
			t.Fatalf("post %s: exit %d %s", id, code, a.h.errb)
		}
	}
	// A stale typed flag is used and refused, though GRID_TASK_EPOCH is current.
	if code := a.cancelJob(t, "J-3", "--lease-epoch", first); code != 3 {
		t.Fatalf("stale global --lease-epoch %s should be used and refused: exit %d %s", first, code, a.h.errb)
	}
	if code := a.h.run(a.path, "job", "cancel", "--task", a.path, "--job", "J-3", "--reason", "x", "--lease-epoch", first); code != 3 {
		t.Fatalf("stale --lease-epoch %s after the command should be used and refused: exit %d %s", first, code, a.h.errb)
	}
	if got := a.status(t, "J-3"); got == contract.Cancelled {
		t.Fatalf("J-3 should not be cancelled by a refused write")
	}
	// A current typed flag is used, though GRID_TASK_EPOCH is the stale one.
	a.agentEnv(t, first)
	if code := a.cancelJob(t, "J-4", "--lease-epoch", second); code != 0 {
		t.Fatalf("current global --lease-epoch %s over stale GRID_TASK_EPOCH %s: exit %d %s", second, first, code, a.h.errb)
	}
	if code := a.h.run(a.path, "job", "cancel", "--task", a.path, "--job", "J-5", "--reason", "x", "--lease-epoch", second); code != 0 {
		t.Fatalf("current --lease-epoch %s after the command over stale GRID_TASK_EPOCH %s: exit %d %s", second, first, code, a.h.errb)
	}
	if a.status(t, "J-4") != contract.Cancelled || a.status(t, "J-5") != contract.Cancelled {
		t.Fatal("J-4 and J-5 should be cancelled")
	}
	// With no flag, GRID_TASK_EPOCH decides: a stale one is refused.
	if code := a.postJob(t, a.path, "J-6"); code != 3 {
		t.Fatalf("job post with the stale GRID_TASK_EPOCH should be refused: exit %d %s", code, a.h.errb)
	}
}

// The boundary: only `cancel` is added. A `post --type control` with the same
// env keeps today's behaviour: it sends the agent's own Job lease and Job epoch
// and does not read GRID_TASK_EPOCH. The task epoch in the env is the current
// one, so were it used the post would be accepted; as on main it is refused
// (exit 3). The same post with the task lease and epoch typed is accepted, so
// the refusal comes from the epoch the env path sends and not from the record.
func TestJobPostControlStillSendsJobEpoch(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	post := func(extra ...string) int {
		args := append([]string{"job", "post", "J-1", "--task", a.path, "--type", "control", "--body", `{"target":"J-1","order":5}`}, extra...)
		return a.h.run(a.path, args...)
	}
	if code := post(); code != 3 {
		t.Fatalf("control post by a Job agent still sends the Job epoch and is refused as on main: exit %d %s", code, a.h.errb)
	}
	if code := post("--lease", "task", "--lease-epoch", a.epoch()); code != 0 {
		t.Fatalf("control post with the task lease and epoch typed: exit %d %s", code, a.h.errb)
	}
	c := a.lastControl(t)
	if c.Lease == nil || *c.Lease != "task" || c.Fence != a.w.Fence {
		t.Fatalf("the typed control should carry the task lease at fence %d: lease %v fence %d", a.w.Fence, c.Lease, c.Fence)
	}
}
