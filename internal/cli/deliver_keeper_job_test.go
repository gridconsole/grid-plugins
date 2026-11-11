package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// keeperHarness is a task with a plan, a proof index that passes and two
// proposals waiting, a Feed identity, and a task lease held by GRID_MEMBER.
func keeperHarness(t *testing.T, proposals ...string) (*harness, string, string, feed.Store) {
	t.Helper()
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	for _, name := range proposals {
		h.write(filepath.Join(dir, "brain-proposals", name+".md"), "---\nstatus: proposed\ntopic: testing\nreason: Reuse "+name+"\n---\n")
	}
	id, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	h.env.Machine = feedMachine
	// The fixture task belongs to the local owner, who is the lease holder here.
	t.Setenv("GRID_MEMBER", "local")
	t.Setenv("GRID_ROLE", "owner")
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_WAKE", "")
	store := feed.Store{TaskDir: dir, TaskID: id}
	w := feed.Writer{Author: feed.Author{ID: "local", Role: "owner"}, Worker: "local@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", w.Fence))
	// The first task of the fixture project, named as the delivery tests name it.
	return h, dir, "T-01", store
}

func curatorJobs(t *testing.T, store feed.Store) ([]job.Job, orchestrator.Snapshot) {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	var out []job.Job
	for _, it := range snap.Items {
		if strings.EqualFold(it.Role, "curator") {
			out = append(out, it)
		}
	}
	return out, snap
}

func (h *harness) deliverRun(dir string, args ...string) int {
	h.out.Reset()
	h.errb.Reset()
	// Run from the home folder, so the task is found by its id as an agent finds it.
	return h.run(h.home, append([]string{"deliver"}, args...)...)
}

// A delivery with proposals waiting posts one curator Job for the keeper.
func TestDeliverPostsOneCuratorJobForWaitingProposals(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	jobs, snap := curatorJobs(t, store)
	if len(jobs) != 1 {
		t.Fatalf("want one curator Job on the Feed, got %d (stdout %q)", len(jobs), h.out.String())
	}
	j := jobs[0]
	if j.ID != "J-1" || len(j.Skills) != 1 || j.Skills[0] != "brain-keeper" || len(j.After) != 0 {
		t.Fatalf("job fields: %+v", j)
	}
	if !strings.Contains(j.Title, "2") || !strings.HasPrefix(j.Intent.Item, "task.md#") {
		t.Fatalf("title or intent does not name the count and the task: %q %+v", j.Title, j.Intent)
	}
	if got := strings.Join(j.Read, ","); got != "brain-proposals/first-fact.md,brain-proposals/second-fact.md" {
		t.Fatalf("read pointers: %s", got)
	}
	if len(j.Acceptance) != 1 || !strings.Contains(j.Acceptance[0].Text, "grid brain keep --task "+id) || !strings.Contains(j.Acceptance[0].Text, "accepted, merged or rejected with a reason") {
		t.Fatalf("acceptance: %+v", j.Acceptance)
	}
	if j.Verify.Kind != "file-exists" || j.Verify.Path == "" {
		t.Fatalf("verify: %+v", j.Verify)
	}
	body, err := json.Marshal(snap.Bodies["J-1"])
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("the posted record is not a valid job-record: %v", err)
	}
	if st := snap.State.Jobs["J-1"].Status; st == contract.Accepted || st == contract.Cancelled || st == contract.Rejected {
		t.Fatalf("the Job is not open: %s", st)
	}
	if !strings.Contains(h.out.String(), "2 brain proposals wait for the curator (Job J-1)") {
		t.Fatalf("count line does not name the Job: %q", h.out.String())
	}
	h.out.Reset()
	if code := h.run(dir, "job", "tree"); code != 0 || !strings.Contains(h.out.String(), "J-1") {
		t.Fatalf("job tree: %d %s %s", code, h.out, h.errb)
	}
}

// A second post while the curator Job is open posts nothing; a closed one does
// not block the next.
func TestDeliverKeeperJobPostsOnceWhileOpen(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact")
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	// A second delivery may be refused by the state table; either way no second Job.
	h.deliverRun(dir, "--outcome", "x", "--task", id)
	env := h.env
	posted, err := postCuratorJob(env, dir, id, []string{"first-fact"})
	if err != nil || posted != "" {
		t.Fatalf("an open curator Job exists, nothing may be posted: %q %v", posted, err)
	}
	if jobs, _ := curatorJobs(t, store); len(jobs) != 1 {
		t.Fatalf("want one curator Job, got %d", len(jobs))
	}
	if code := h.run(dir, "job", "cancel", "J-1", "--reason", "scope withdrawn"); code != 0 {
		t.Fatalf("cancel: %d %s", code, h.errb)
	}
	posted, err = postCuratorJob(env, dir, id, []string{"first-fact"})
	if err != nil || posted != "J-2" {
		t.Fatalf("a cancelled curator Job must not block the next: %q %v", posted, err)
	}
}

func TestDeliverWithNoProposalsPostsNoJobAndPrintsAsBefore(t *testing.T) {
	h, dir, id, store := keeperHarness(t)
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	if want := id + " delivered; waiting for the person\n"; h.out.String() != want || h.errb.Len() != 0 {
		t.Fatalf("output changed: %q %q", h.out.String(), h.errb.String())
	}
	if jobs, _ := curatorJobs(t, store); len(jobs) != 0 {
		t.Fatalf("no proposals, yet a Job was posted: %+v", jobs)
	}
}

// A delivery that cannot post still delivers, exit 0, with one stderr line.
func TestDeliverStillDeliversWhenTheCuratorJobCannotBePosted(t *testing.T) {
	for name, epoch := range map[string]string{"no-epoch": "", "taken-by-someone-else": "0.999"} {
		t.Run(name, func(t *testing.T) {
			h, dir, id, store := keeperHarness(t, "first-fact")
			t.Setenv("GRID_LEASE_EPOCH", epoch)
			if code := h.deliverRun(dir, "--outcome", "x", "--task", id); code != 0 {
				t.Fatalf("a delivery that cannot post must still exit 0: %d %s", code, h.errb)
			}
			lines := strings.Split(strings.TrimSpace(h.errb.String()), "\n")
			if len(lines) != 1 || !strings.Contains(lines[0], "curator Job was not posted") {
				t.Fatalf("stderr: %q", h.errb.String())
			}
			if !strings.Contains(h.out.String(), id+" delivered; waiting for the person") || !strings.Contains(h.out.String(), "1 brain proposals wait for the curator") {
				t.Fatalf("stdout: %q", h.out.String())
			}
			rec, err := task.Read(dir)
			if err != nil || rec.State != "waiting" {
				t.Fatalf("task did not reach waiting: %+v %v", rec, err)
			}
			if jobs, _ := curatorJobs(t, store); len(jobs) != 0 {
				t.Fatalf("a Job was posted: %+v", jobs)
			}
		})
	}
}

func TestDeliverDryRunPostsNothingAndListsTheCuratorJob(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id, "--dry-run"); code != 0 {
		t.Fatalf("dry run: %d %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), "Would post a curator Job for 2 proposals") {
		t.Fatalf("dry run does not list the Job: %q", h.out.String())
	}
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id, "--dry-run", "--json"); code != 0 {
		t.Fatalf("dry run json: %d %s", code, h.errb)
	}
	var res struct {
		Actions    []string `json:"actions"`
		CuratorJob *string  `json:"curator_job"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.CuratorJob == nil || *res.CuratorJob != "" {
		t.Fatalf("dry run json: %v %s", err, h.out)
	}
	if got := strings.Join(res.Actions, "|"); !strings.Contains(got, "Post a curator Job for 2 proposals") {
		t.Fatalf("actions: %s", got)
	}
	if jobs, _ := curatorJobs(t, store); len(jobs) != 0 {
		t.Fatalf("a dry run posted a Job: %+v", jobs)
	}
}

func TestDeliverJSONNamesTheCuratorJobAndKeepsWaitingProposals(t *testing.T) {
	h, dir, id, _ := keeperHarness(t, "first-fact", "second-fact")
	if code := h.deliverRun(dir, "--outcome", "x", "--task", id, "--json"); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	var res struct {
		Waiting    []map[string]any `json:"waiting_proposals"`
		CuratorJob string           `json:"curator_job"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.CuratorJob != "J-1" || len(res.Waiting) != 2 {
		t.Fatalf("json: %v %s", err, h.out)
	}
}
