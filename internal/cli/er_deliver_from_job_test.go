package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// erDeliverJob is a task whose lease holder has posted and assigned J-1 to itself,
// and the environment of the agent for J-1 as the worker gives it: the Job's own
// lease and epoch, and the task epoch the holder holds.
type erDeliverJob struct {
	h         *harness
	dir, id   string
	store     feed.Store
	w         feed.Writer // the task lease holder
	jobFence  int64
	taskEpoch string
}

func erDeliverSetup(t *testing.T, proposals ...string) *erDeliverJob {
	t.Helper()
	h, dir, id, store := keeperHarness(t, proposals...)
	taskEpoch := os.Getenv("GRID_LEASE_EPOCH")
	fence, err := strconv.ParseInt(strings.TrimPrefix(taskEpoch, "0."), 10, 64)
	if err != nil {
		t.Fatal(taskEpoch, err)
	}
	a := &erDeliverJob{h: h, dir: dir, id: id, store: store, taskEpoch: taskEpoch,
		w: feed.Writer{Author: feed.Author{ID: "local", Role: "owner"}, Worker: "local@" + feedMachine, Lease: "task", Fence: fence}}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: workerNextBody(t, "J-1")}, a.w); err != nil {
		t.Fatal(err)
	}
	// The holder takes J-1 as a worker, with the task epoch as its lease epoch.
	t.Setenv("GRID_ROLE", "worker")
	if code := h.run(dir, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil || state.Jobs["J-1"].Assign == nil {
		t.Fatal(state, err)
	}
	a.jobFence = state.Jobs["J-1"].Assign.Fence
	if jobEpoch := fmt.Sprintf("0.%d", a.jobFence); jobEpoch == taskEpoch {
		t.Fatalf("the Job epoch %s must differ from the task epoch", jobEpoch)
	}
	a.agentEnv(t, taskEpoch)
	return a
}

// agentEnv is the environment of the agent for J-1, read from the worker's start of an agent.
func (a *erDeliverJob) agentEnv(t *testing.T, taskEpoch string) {
	t.Helper()
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_MEMBER", "local")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", a.jobFence))
	t.Setenv("GRID_TASK_EPOCH", taskEpoch)
}

// deliver runs `grid [global...] deliver --task T-01 --outcome done --json` from the home folder.
func (a *erDeliverJob) deliver(global ...string) int {
	a.h.out.Reset()
	a.h.errb.Reset()
	return a.h.run(a.h.home, append(global, "deliver", "--task", a.id, "--outcome", "done", "--json")...)
}

func (a *erDeliverJob) curatorJob(t *testing.T) string {
	t.Helper()
	var res struct {
		CuratorJob string `json:"curator_job"`
	}
	if err := json.Unmarshal(a.h.out.Bytes(), &res); err != nil {
		t.Fatalf("--json: %v %s", err, a.h.out)
	}
	return res.CuratorJob
}

func (a *erDeliverJob) jobCount(t *testing.T) int {
	t.Helper()
	snap, err := a.store.State()
	if err != nil {
		t.Fatal(err)
	}
	return len(snap.Jobs)
}

// retake is the holder restarting: it gives the lease back and takes it with a higher fence.
func (a *erDeliverJob) retake(t *testing.T) {
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
	if _, err := a.store.Handover(feed.Input{Body: map[string]any{}}, a.w); err != nil {
		t.Fatal(err)
	}
	if a.w.Fence <= old {
		t.Fatalf("the new fence %d should be higher than %d", a.w.Fence, old)
	}
}

// Row 3: from a Job agent, deliver posts the curator Job under the task lease at the task fence.
func TestErDeliverFromAJobPostsTheCuratorJobAtTheTaskEpoch(t *testing.T) {
	a := erDeliverSetup(t, "first-fact", "second-fact")
	if code := a.deliver(); code != 0 {
		t.Fatalf("deliver: exit %d %s", code, a.h.errb)
	}
	if a.h.errb.Len() != 0 {
		t.Fatalf("stderr: %s", a.h.errb)
	}
	curator := a.curatorJob(t)
	if curator == "" {
		t.Fatalf("curator_job is empty; stderr %q stdout %s", a.h.errb, a.h.out)
	}
	jobs, _ := curatorJobs(t, a.store)
	if len(jobs) != 1 || jobs[0].ID != curator || !strings.EqualFold(jobs[0].Role, "curator") {
		t.Fatalf("the Feed holds %+v, want curator Job %s", jobs, curator)
	}
	res, err := a.store.Read(feed.Filter{Type: "job", Job: curator})
	if err != nil || len(res.Records) != 1 {
		t.Fatalf("job record of %s: %d %v", curator, len(res.Records), err)
	}
	rec := res.Records[0]
	if rec.Lease == nil || *rec.Lease != "task" || rec.Fence != a.w.Fence || rec.Gen != 0 {
		t.Fatalf("the curator Job was written under lease %v gen %d fence %d, want task 0.%d (Job fence %d)", rec.Lease, rec.Gen, rec.Fence, a.w.Fence, a.jobFence)
	}
}

// Row 4: no proposal waits, so no curator Job is posted; the one new Job is the
// person Job of the ship step (owner ruling 2212Z M1, orchestrator 14 Nov 22:55).
// Was TestErDeliverFromAJobWithNoProposalPostsNothing.
func TestErDeliverFromAJobWithNoProposalPostsOnlyThePersonJob(t *testing.T) {
	a := erDeliverSetup(t)
	before := a.jobCount(t)
	if code := a.deliver(); code != 0 {
		t.Fatalf("deliver: exit %d %s", code, a.h.errb)
	}
	if got := a.curatorJob(t); got != "" {
		t.Fatalf("curator_job = %q, want empty", got)
	}
	if after := a.jobCount(t); after != before+1 {
		t.Fatalf("the Feed holds %d Jobs, was %d, want one more: the person Job", after, before)
	}
	var out struct {
		PersonJob string `json:"person_job"`
	}
	if err := json.Unmarshal(a.h.out.Bytes(), &out); err != nil {
		t.Fatalf("--json: %v %s", err, a.h.out)
	}
	if j, _ := openApproveJob(t, a.store); out.PersonJob == "" || j.ID != out.PersonJob {
		t.Fatalf("the new Job is not the open approve person Job: person_job %q, open approve Job %s", out.PersonJob, j.ID)
	}
}

// Row 5: a typed global --lease-epoch beats GRID_TASK_EPOCH, as it does for `job post`.
func TestErDeliverTypedLeaseEpochBeatsTheTaskEpoch(t *testing.T) {
	a := erDeliverSetup(t, "first-fact")
	stale := a.taskEpoch
	a.retake(t)
	a.agentEnv(t, a.epochNow())
	before := a.jobCount(t)
	if code := a.deliver("--lease-epoch", stale); code != 0 {
		t.Fatalf("deliver still exits 0: %d %s", code, a.h.errb)
	}
	if got := a.curatorJob(t); got != "" {
		t.Fatalf("the stale typed epoch %s was not used: curator_job %q", stale, got)
	}
	if !strings.Contains(a.h.errb.String(), "the curator Job was not posted") {
		t.Fatalf("stderr does not say the Job was not posted: %q", a.h.errb)
	}
	if after := a.jobCount(t); after != before {
		t.Fatalf("a Job was posted: %d, was %d", after, before)
	}
}

// Row 5, the other half: with no flag, the current GRID_TASK_EPOCH posts it after a retake.
func TestErDeliverCurrentTaskEpochPostsAfterARetake(t *testing.T) {
	a := erDeliverSetup(t, "first-fact")
	a.retake(t)
	a.agentEnv(t, a.epochNow())
	if code := a.deliver(); code != 0 {
		t.Fatalf("deliver: exit %d %s", code, a.h.errb)
	}
	if got := a.curatorJob(t); got == "" {
		t.Fatalf("the current GRID_TASK_EPOCH posted no curator Job: stderr %q", a.h.errb)
	}
}

func (a *erDeliverJob) epochNow() string { return fmt.Sprintf("0.%d", a.w.Fence) }
