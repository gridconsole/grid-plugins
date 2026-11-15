package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// r18PersonAtTerminal turns the environment of the running J-1 agent into a person's at a
// terminal: the member who started the worker (so the same <member>@<machine> as the task
// lease holder), role owner, and no lease, no lease epoch and no task epoch.
func r18PersonAtTerminal(t *testing.T, member string) {
	t.Helper()
	t.Setenv("GRID_MEMBER", member)
	t.Setenv("GRID_ROLE", "owner")
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_LEASE_EPOCH", "")
	t.Setenv("GRID_TASK_EPOCH", "")
	t.Setenv("GRID_JOB", "")
}

// r18Cancel runs `grid job cancel` as a person types it: the task and the Job, a reason, and
// whatever else the caller adds.
func r18Cancel(a *agentTask, extra ...string) int {
	args := append([]string{"job", "cancel", "--task", a.path, "--job", "J-1", "--reason", "scope withdrawn"}, extra...)
	return a.h.run(a.path, args...)
}

// Real run round 18, F4: a Job is Running under a worker that holds the task lease, and the
// owner at a terminal runs `grid job cancel`. A person holds no lease epoch, so the cancel had
// nothing to send: with none typed it stopped at "GRID_LEASE_EPOCH or --lease-epoch must be
// gen.fence", and the owner had to guess. The CLI reads the current task lease from the Feed and
// writes the cancel under it, with the person as the author.
func TestR18PersonCancelsRunningJobWithoutAnEpoch(t *testing.T) {
	a, _ := kbRoleAgent(t)
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q before the cancel, want running", got)
	}
	r18PersonAtTerminal(t, "worker")
	if code := r18Cancel(a); code != 0 {
		t.Fatalf("a person's cancel of a running Job: exit %d %s", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %q after the cancel, want cancelled", got)
	}
	c := a.lastControl(t)
	if c.Lease == nil || *c.Lease != "task" || c.Fence != a.w.Fence {
		t.Fatalf("the cancel should be written under the task lease at fence %d: lease %v fence %d", a.w.Fence, c.Lease, c.Fence)
	}
	if c.Author.Role != "owner" || c.Author.ID != "worker" {
		t.Fatalf("the cancel should be authored by the person (owner): %+v", c.Author)
	}
	if c.Body["status"] != "cancelled" || c.Body["target"] != "J-1" || c.Body["close_reason"] != "scope withdrawn" {
		t.Fatalf("the cancel body: %v", c.Body)
	}
}

// The shape r18 typed: a guessed task epoch, and the Job lease. A typed epoch is the caller's
// word and is used as typed, as for an agent: a wrong one is still "stale lease", exit 3, and the
// Job stays Running. The person leaves the epoch out to have it read.
func TestR18PersonTypedGuessedEpochStaysStale(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "worker")
	guess := fmt.Sprintf("0.%d", a.w.Fence+5)
	for _, extra := range [][]string{
		{"--lease", "task", "--lease-epoch", guess},
		{"--lease-epoch", guess},
		{"--lease", "J-1", "--lease-epoch", fmt.Sprintf("0.%d", a.jobFence)},
	} {
		if code := r18Cancel(a, extra...); code != 3 || !strings.Contains(a.h.errb.String(), "stale lease") {
			t.Fatalf("%v: exit %d %s, want 3 stale lease", extra, code, a.h.errb)
		}
		if got := a.status(t, "J-1"); got != contract.Running {
			t.Fatalf("%v: J-1 is %q after a refused cancel, want running", extra, got)
		}
		a.h.errb.Reset()
	}
}

// The person is not the member the task lease is held by: reading the lease does not let one
// member write as another. The cancel is refused as stale lease, exit 3, and the Job runs on.
func TestR18PersonOfAnotherMemberStaysStale(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r18Cancel(a); code != 3 || !strings.Contains(a.h.errb.String(), "stale lease") {
		t.Fatalf("a cancel by a member who does not hold the task lease: exit %d %s, want 3 stale lease", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q after a refused cancel, want running", got)
	}
}

// The agent path is as it was: an agent (role worker) has the epochs the worker gave it, and
// the CLI does not read the lease for it. With a stale GRID_TASK_EPOCH it is refused as stale
// lease, exit 3; with no epoch at all it is told to give one, and nothing is cancelled.
func TestR18AgentCancelIsNotGivenTheLease(t *testing.T) {
	a, _ := kbRoleAgent(t)
	a.agentEnv(t, fmt.Sprintf("0.%d", a.w.Fence+5))
	if code := r18Cancel(a); code != 3 || !strings.Contains(a.h.errb.String(), "stale lease") {
		t.Fatalf("an agent with a stale GRID_TASK_EPOCH: exit %d %s, want 3 stale lease", code, a.h.errb)
	}
	a.h.errb.Reset()
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_LEASE_EPOCH", "")
	t.Setenv("GRID_TASK_EPOCH", "")
	if code := r18Cancel(a); code == 0 || !strings.Contains(a.h.errb.String(), "gen.fence") {
		t.Fatalf("an agent with no epoch: exit %d %s, want a refusal that asks for gen.fence", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q after the refused cancels, want running", got)
	}
}
