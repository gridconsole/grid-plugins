package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// These tests pin a land whose accepted rebase Job still cannot land on the same main: its
// branch still conflicts, or no branch of it is in this checkout (its builder ran on another
// machine). The land neither reworks nor retries it: it raises one agent decision Job from the
// rebase Job, the way the Judge's decide does, and parks until main moves or the decision posts
// the next rebase Job.

// decisionsFrom is the agent decision Jobs ("Decide how <head> continues ...") raised from head.
func decisionsFrom(t *testing.T, store feed.Store, head string) []job.Job {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	var out []job.Job
	for _, it := range snap.Items {
		if strings.EqualFold(it.Role, "orchestrator") && it.DiscoveredFrom != nil && *it.DiscoveredFrom == head && strings.HasPrefix(it.Title, "Decide how "+head+" continues") {
			out = append(out, it)
		}
	}
	return out
}

// acceptedRebase approves the delivery, which conflicts with main, and accepts its rebase Job
// after a review; branch, when set, is the branch its builder left, made from it. It returns
// the ship Job and the rebase Job.
func acceptedRebase(t *testing.T, branch string) (env Env, dir, repo string, store feed.Store, ship job.Job, head string, stderr *bytes.Buffer) {
	t.Helper()
	env, dir, repo, _, store, buf := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ = openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, buf)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), buf)
	}
	head = rebases[0].ID
	postReview(t, env, store, head)
	if branch != "" {
		fid, _, err := task.FeedIdentity(dir)
		if err != nil {
			t.Fatal(err)
		}
		shipGit(t, repo, "branch", "grid-"+fid+"-"+head+"-a1", branch)
	}
	setJobStatus(t, env, store, head, contract.Accepted, "resolved")
	return env, dir, repo, store, ship, head, buf
}

// landParked fails the test unless the land of an accepted rebase Job on an unmoved main
// raised one decision Job from it and nothing else: no rework or new rebase Job, no retry,
// no person Job, and later passes neither post nor run the land again.
func landParked(t *testing.T, env Env, dir, repo string, store feed.Store, ship job.Job, head string, stderr *bytes.Buffer) {
	t.Helper()
	main := shipGit(t, repo, "rev-parse", "main")
	landAfterShip(env, dir, landOwner)
	if d := decisionsFrom(t, store, head); len(d) != 1 {
		t.Fatalf("%d decision Jobs from %s, want one; stderr %s; history:\n%s", len(d), head, stderr, shipHistory(t, dir))
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want only %s: an accepted rebase on an unmoved main is not reworked; stderr %s", len(rebases), head, stderr)
	}
	h := shipHistory(t, dir)
	if strings.Contains(h, "grid land runs again on the next pass") {
		t.Fatalf("the land was armed to retry; history:\n%s", h)
	}
	runs := strings.Count(h, "accepted; grid land runs")
	for pass := 0; pass < 3; pass++ {
		landAfterShip(env, dir, landOwner)
	}
	if d := decisionsFrom(t, store, head); len(d) != 1 {
		t.Fatalf("%d decision Jobs from %s after three more passes, want still one", len(d), head)
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after three more passes, want only %s", len(rebases), head)
	}
	if n := strings.Count(shipHistory(t, dir), "accepted; grid land runs"); n != runs {
		t.Fatalf("the land ran %d times, want %d: a parked land does not run while main is where it was; history:\n%s", n, runs, shipHistory(t, dir))
	}
	noPersonJob(t, store, ship.ID, stderr)
	if got := shipGit(t, repo, "rev-parse", "main"); got != main {
		t.Fatalf("main moved to %s", got)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v, want waiting", rec.State, err)
	}
}

// The rebase Job is accepted, but no grid-<feed id>-<job>-a* branch of it is in this checkout,
// and main has not moved: one decision Job, no rework, and a second pass posts nothing.
func TestShipAcceptedRebaseWithNoBranchRaisesOneDecision(t *testing.T) {
	env, dir, repo, store, ship, head, stderr := acceptedRebase(t, "")
	landParked(t, env, dir, repo, store, ship, head, stderr)
	d := decisionsFrom(t, store, head)[0]
	text := ""
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if intent, ok := snap.Bodies[d.ID]["intent"].(map[string]any); ok {
		text, _ = intent["goal"].(string)
	}
	for _, want := range []string{"grid-", head + "-a", "parks the land until main moves", rebaseTitle} {
		if !strings.Contains(text, want) {
			t.Fatalf("the decision Job's goal does not say %q:\n%s", want, text)
		}
	}
}

// The rebase Job is accepted and its branch is found, but it still conflicts with the same main:
// one decision Job, no rework, and a second pass neither posts nor runs the land.
func TestShipAcceptedRebaseStillConflictingRaisesOneDecision(t *testing.T) {
	env, dir, repo, store, ship, head, stderr := acceptedRebase(t, "lane")
	landParked(t, env, dir, repo, store, ship, head, stderr)
}

// A rebase line an earlier version wrote holds no target tip. Once main moves, the land of a
// rebase Job the Judge owns runs again and posts a fresh root rebase, as it does for a line with
// a tip; the empty tip does not park it for good.
func TestShipRebaseLineWithNoTipRunsOnceMainMoved(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), stderr)
	}
	head := rebases[0].ID
	if err := (land.Store{Resolver: resolver(env), Project: "sample"}).RebaseQueued(env.Cwd, "T-01", ship.ID, head, ""); err != nil {
		t.Fatal(err)
	}
	postReview(t, env, store, head)
	setJobStatus(t, env, store, head, contract.Rejected, "the Judge ended it")
	landAnother(t, repo, "moved", "moved")
	landAfterShip(env, dir, landOwner)
	again, snap := rebaseJobs(t, store)
	if len(again) != 2 || again[1].Continues != nil && *again[1].Continues != "" {
		t.Fatalf("%d rebase Jobs after main moved, want a second root; stderr %s; history:\n%s", len(again), stderr, shipHistory(t, dir))
	}
	if st := snap.State.Jobs[again[1].ID]; st == nil || st.Status == contract.Accepted || st.Status == contract.Cancelled {
		t.Fatalf("rebase Job %s is %+v, want open", again[1].ID, st)
	}
	noPersonJob(t, store, ship.ID, stderr)
}

// A rebase Job whose review was lost and whose decision Job (raised from the rebase Job itself)
// accepted it is not parked by that decision: its branch lands.
func TestShipRebaseAcceptedByALostReviewDecisionLands(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	head := rebases[0].ID
	review := postReview(t, env, store, head)
	setJobStatus(t, env, store, review, contract.Cancelled, "no agent for the review")
	decision := postDiscovered(t, env, store, "orchestrator", "Decide how "+head+" continues without its review", head)
	fakeBuilder(t, dir, repo, head)
	setJobStatus(t, env, store, head, contract.Accepted, "accepted by "+decision)
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the decision accepted the rebase, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
}

// A land parked on an accepted rebase Job runs again once main moves, with the branch of that Job
// or without one: the delivery still conflicts with the new main, so the land posts a fresh root
// rebase Job, and the person is not asked.
func TestShipParkedRebaseRunsOnceMainMoved(t *testing.T) {
	for _, branch := range []string{"", "lane"} {
		t.Run("branch "+branch, func(t *testing.T) {
			env, dir, repo, store, ship, head, stderr := acceptedRebase(t, branch)
			landAfterShip(env, dir, landOwner)
			if d := decisionsFrom(t, store, head); len(d) != 1 {
				t.Fatalf("%d decision Jobs from %s, want one; stderr %s", len(d), head, stderr)
			}
			landAnother(t, repo, "moved", "moved")
			landAfterShip(env, dir, landOwner)
			again, snap := rebaseJobs(t, store)
			if len(again) != 2 || len(rebaseRoots(snap.Items)) != 2 {
				t.Fatalf("%d rebase Jobs, %d roots after main moved, want a second root; stderr %s; history:\n%s", len(again), len(rebaseRoots(snap.Items)), stderr, shipHistory(t, dir))
			}
			noPersonJob(t, store, ship.ID, stderr)
		})
	}
}

// A land parked on an accepted rebase Job resumes when its decision Job posts a rework of it: once
// the rework is accepted the task lands, on the same main.
func TestShipParkedRebaseLandsAfterTheDecisionsRework(t *testing.T) {
	env, dir, repo, store, _, head, stderr := acceptedRebase(t, "lane")
	landAfterShip(env, dir, landOwner)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if d := decisionsFrom(t, store, head); len(d) != 1 {
		t.Fatalf("%d decision Jobs from %s, want one; stderr %s", len(d), head, stderr)
	}
	body := map[string]any{}
	for k, v := range snap.Bodies[head] {
		body[k] = v
	}
	rework := nextJobID(snap)
	body["id"], body["continues"] = rework, head
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: rework, Body: body}); err != nil {
		t.Fatal(err)
	}
	fakeBuilder(t, dir, repo, rework)
	setJobStatus(t, env, store, rework, contract.Accepted, "reworked")
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rework was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
}
