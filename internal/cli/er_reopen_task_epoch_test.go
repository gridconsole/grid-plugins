package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// erReopenJob is a task whose plan was approved and whose task was closed by the
// person, with the holder's Job J-1 assigned so its fence is real. The env is
// then the one a Job's agent gets: GRID_ROLE=worker, GRID_MEMBER the holder,
// GRID_LEASE the Job, GRID_LEASE_EPOCH the Job's own epoch and GRID_TASK_EPOCH
// the task epoch the holder holds.
type erReopenJob struct {
	h         *harness
	path      string
	store     feed.Store
	hash      string
	digest    string
	w         feed.Writer
	taskEpoch string
	jobEpoch  string
}

func erReopenSetup(t *testing.T) *erReopenJob {
	t.Helper()
	h, path, store, text := approveJobHarness(t)
	// orchestratorHarness left the task epoch the holder holds in GRID_LEASE_EPOCH.
	taskEpoch := os.Getenv("GRID_LEASE_EPOCH")
	parts := strings.Split(taskEpoch, ".")
	fence, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		t.Fatalf("task epoch %q: %v", taskEpoch, err)
	}
	a := &erReopenJob{h: h, path: path, store: store, hash: plan.Hash(text), digest: plan.Digest(text), taskEpoch: taskEpoch,
		w: feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task", Fence: fence}}
	if code := h.run(path, "approve", path, "--plan-hash", a.hash); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb)
	}
	if code := h.run(path, "task", "close", "T-07"); code != 0 {
		t.Fatalf("close: %d %s", code, h.errb)
	}
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil || state.Jobs["J-1"] == nil || state.Jobs["J-1"].Assign == nil {
		t.Fatalf("J-1 is not assigned: %v %v", state, err)
	}
	if state.Jobs["J-1"].Assign.Fence == fence {
		t.Fatalf("the Job fence %d must differ from the task fence %d", state.Jobs["J-1"].Assign.Fence, fence)
	}
	a.jobEpoch = fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence)
	a.agentEnv(t, a.taskEpoch)
	return a
}

// agentEnv is the env a Job's agent gets, with the task epoch it was handed.
func (a *erReopenJob) agentEnv(t *testing.T, taskEpoch string) {
	t.Helper()
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", a.jobEpoch)
	t.Setenv("GRID_TASK_EPOCH", taskEpoch)
}

// retake is the holder restarting: the task lease comes back with a higher fence.
func (a *erReopenJob) retake(t *testing.T) string {
	t.Helper()
	old := a.w.Fence
	ts := lease.TaskStore{Feed: a.store}
	if err := ts.ReleaseTask(a.w); err != nil {
		t.Fatal(err)
	}
	g, err := ts.TakeTask(a.w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	a.w.Fence = g.Fence
	if a.w.Fence <= old {
		t.Fatalf("the new fence %d should be higher than %d", a.w.Fence, old)
	}
	if _, err := a.store.Handover(feed.Input{Body: map[string]any{}}, a.w); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("0.%d", a.w.Fence)
}

// planJobRecord is the Feed record of the new plan Job J-3, or nil.
func (a *erReopenJob) planJobRecord(t *testing.T) *feed.Record {
	t.Helper()
	r, err := a.store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.Records {
		if r.Records[i].Job != nil && *r.Records[i].Job == "J-3" {
			return &r.Records[i].Record
		}
	}
	return nil
}

// Rows 1 and 2: a reopen from a Job posts the new plan Job under the task epoch,
// and the person's grid approve answers it.
func TestErReopenFromJobPostsPlanJobAtTaskEpoch(t *testing.T) {
	a := erReopenSetup(t)
	if code := a.h.run(a.path, "task", "reopen", "T-07"); code != 0 {
		t.Fatalf("grid task reopen from a Job: exit %d, stderr: %s", code, a.h.errb)
	}
	state, err := a.store.State()
	if err != nil {
		t.Fatal(err)
	}
	j3 := state.Jobs["J-3"]
	if j3 == nil || j3.Status == contract.Accepted || j3.Status == contract.Cancelled || j3.Kind != "approve" || j3.Continues == nil || *j3.Continues != "J-2" {
		t.Fatalf("the reopen did not post an open approve Job J-3 that continues J-2: %+v", j3)
	}
	rec := a.planJobRecord(t)
	if rec == nil || rec.Lease == nil || *rec.Lease != "task" || rec.Gen != 0 || rec.Fence != a.w.Fence {
		t.Fatalf("J-3 must be written under lease task at fence %d: %+v", a.w.Fence, rec)
	}
	if taskstore.Approved(a.path) {
		t.Fatal("Approved is true while the new plan Job is open")
	}
	// Row 2: the person answers the new Job, as the owner path of reopenedPlan does:
	// GRID_ROLE=member, the task epoch in GRID_LEASE_EPOCH, no GRID_TASK_EPOCH. (With
	// no epoch at all, grid approve posts an unleased answer that the holder still
	// has to accept, so the Job would stay open.)
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_LEASE_EPOCH", a.taskEpoch)
	t.Setenv("GRID_TASK_EPOCH", "")
	// The reopened task waits for the person, as in reopenedPlan.
	if code := a.h.run(a.path, "task", "status", "T-07", "waiting"); code != 0 {
		t.Fatalf("waiting: %d %s", code, a.h.errb)
	}
	if taskstore.Approved(a.path) {
		t.Fatal("Approved is true while the new plan Job is open")
	}
	if code := a.h.run(a.path, "approve", a.path, "--plan-hash", a.hash); code != 0 {
		t.Fatalf("grid approve by the person: exit %d, stderr: %s", code, a.h.errb)
	}
	if !taskstore.Approved(a.path) || !taskstore.ApprovedDigest(a.path, a.hash, a.digest) {
		t.Fatalf("grid approve did not approve the plan again: %s %s", a.h.out, a.h.errb)
	}
	if got, err := taskstore.ApprovalFromJob(a.path); err != nil || got.Job != "J-3" {
		t.Fatalf("approval from the new Job: %+v %v", got, err)
	}
}

// Row 3: a typed global --lease-epoch is used over GRID_TASK_EPOCH; with no flag
// the current GRID_TASK_EPOCH posts the plan Job.
func TestErReopenTypedGlobalLeaseEpochBeatsTaskEpoch(t *testing.T) {
	t.Run("stale typed global is used and nothing is posted", func(t *testing.T) {
		a := erReopenSetup(t)
		old := a.taskEpoch
		current := a.retake(t)
		a.agentEnv(t, current)
		if code := a.h.run(a.path, "--lease-epoch", old, "task", "reopen", "T-07"); code == 0 {
			t.Fatalf("the stale global --lease-epoch %s should be used and refused, GRID_TASK_EPOCH %s was used instead", old, current)
		}
		if rec := a.planJobRecord(t); rec != nil {
			t.Fatalf("no plan Job may be posted under a stale epoch: %+v", rec)
		}
	})
	t.Run("current GRID_TASK_EPOCH with no flag posts", func(t *testing.T) {
		a := erReopenSetup(t)
		current := a.retake(t)
		a.agentEnv(t, current)
		if code := a.h.run(a.path, "task", "reopen", "T-07"); code != 0 {
			t.Fatalf("GRID_TASK_EPOCH alone: exit %d, stderr: %s", code, a.h.errb)
		}
		if rec := a.planJobRecord(t); rec == nil || rec.Fence != a.w.Fence {
			t.Fatalf("J-3 must be written at the current task fence %d: %+v", a.w.Fence, rec)
		}
	})
}
