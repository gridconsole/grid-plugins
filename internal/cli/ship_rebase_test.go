package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// These tests pin a land that stops on a merge conflict: a builder Job rebases the
// delivered branch on the base, the person's approval stands, and land runs again once
// that Job is accepted. shipMergeFixture is the delivered task (branch lane edits the
// line of a.txt); landFirst is the other task that edited the same line and landed first.

// landFirst lands another task's branch on main: it edits the line of a.txt that lane
// edits, as the first of two tasks to land does.
func landFirst(t *testing.T, repo string) string {
	t.Helper()
	shipGit(t, repo, "checkout", "-q", "-b", "first", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "commit", "-q", "-am", "the first task's work")
	shipGit(t, repo, "checkout", "-q", "main")
	shipGit(t, repo, "merge", "-q", "--no-ff", "-m", "land the first task", "first")
	return shipGit(t, repo, "rev-parse", "main")
}

// rebaseJobs is the rebase Jobs of the Feed, roots and reworks, in Feed order.
func rebaseJobs(t *testing.T, store feed.Store) ([]job.Job, orchestrator.Snapshot) {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	var out []job.Job
	for _, it := range snap.Items {
		if isRebaseJob(it) {
			out = append(out, it)
		}
	}
	return out, snap
}

// fakeBuilder is the builder of a rebase Job: it makes the branch the worker would,
// grid-<feed id>-<job>-a1, off main with the delivered branch merged in and the
// conflict resolved keeping both changes. It does not mark the Job accepted.
func fakeBuilder(t *testing.T, dir, repo, rebase string) string {
	t.Helper()
	fid, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	branch := "grid-" + fid + "-" + rebase + "-a1"
	shipGit(t, repo, "checkout", "-q", "-b", branch, "main")
	shipGit(t, repo, "merge", "-q", "-s", "ours", "--no-edit", "lane")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("first\nlane\n"), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "commit", "-q", "-am", "resolve a.txt keeping both changes")
	shipGit(t, repo, "checkout", "-q", "main")
	return branch
}

// setJobStatus is the control a Judge or a person posts on the Job id.
func setJobStatus(t *testing.T, env Env, store feed.Store, id string, status contract.Status, reason string) {
	t.Helper()
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"target": id, "status": string(status), "reason": reason}
	if status == contract.Cancelled || status == contract.Rejected {
		body["close_reason"] = reason
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "control", Body: body}); err != nil {
		t.Fatalf("control %s %s: %v", id, status, err)
	}
}

// Two tasks edit the same line. The first lands. The second's approval lands nothing,
// posts a rebase Job and keeps the approval; once a builder has resolved the conflict
// and the Job is accepted, the next pass lands the rebased branch and closes the task.
func TestShipConflictPostsRebaseJobThenLandsTheRebasedBranch(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	first := landFirst(t, repo)
	ship, _ := openApproveJob(t, store)

	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	if got := shipGit(t, repo, "rev-parse", "main"); got != first {
		t.Fatalf("main moved to %s on a conflicting land; stderr %s", got, stderr)
	}
	rebases, snap := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after the conflict, want 1; stderr %s; history:\n%s", len(rebases), stderr, shipHistory(t, dir))
	}
	rebase := rebases[0]
	if rebase.Role != "builder" || len(rebase.After) != 0 || rebase.Continues != nil {
		t.Fatalf("rebase Job %+v, want a builder Job with no after and no continues", rebase)
	}
	if want := "Rebase the delivered branch on main and resolve the conflicts in a.txt"; rebase.Title != want {
		t.Fatalf("rebase Job title %q, want %q", rebase.Title, want)
	}
	rows := ""
	for _, a := range rebase.Acceptance {
		rows += a.Text + "\n"
	}
	for _, want := range []string{"lane", "main", "a.txt", "both changes"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("rebase Job acceptance does not name %q:\n%s", want, rows)
		}
	}
	if st := snap.State.Jobs[ship.ID]; st == nil || st.Status != contract.Accepted {
		t.Fatalf("ship Job %s is %+v after the conflict, want accepted (the approval stands); stderr %s", ship.ID, st, stderr)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v while the rebase waits, want waiting", rec.State, err)
	}
	history := shipHistory(t, dir)
	for _, want := range []string{" · land failed · ", "approval kept; waits for rebase " + rebase.ID} {
		if !strings.Contains(history, want) {
			t.Fatalf("history has no %q:\n%s", want, history)
		}
	}
	// The rebase Job is a Job a worker can start: render's lint refuses nothing. The
	// Outcome heading is what a delivered task.md has and the Job's intent points at.
	if f, err := os.OpenFile(filepath.Join(dir, "task.md"), os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
		t.Fatal(err)
	} else {
		_, werr := f.WriteString("\n## Outcome\n\nThe delivered change.\n")
		if cerr := f.Close(); werr != nil || cerr != nil {
			t.Fatal(werr, cerr)
		}
	}
	findings, err := postFindings(env, dir, snap.Bodies[rebase.ID])
	if err != nil {
		t.Fatal(err)
	}
	if lint.HasErrors(findings) {
		t.Fatalf("render would refuse the rebase Job: %+v", findings)
	}

	// A pass while the rebase Job is open lands nothing and posts nothing more.
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	if n := strings.Count(shipHistory(t, dir), " · land failed · "); n != 1 {
		t.Fatalf("%d land failed lines while the rebase is open, want 1:\n%s", n, shipHistory(t, dir))
	}
	if again, _ := rebaseJobs(t, store); len(again) != 1 {
		t.Fatalf("%d rebase Jobs while the first is open, want 1", len(again))
	}

	// The builder resolves, the Job is accepted, and the next pass lands.
	branch := fakeBuilder(t, dir, repo, rebase.ID)
	setJobStatus(t, env, store, rebase.ID, contract.Accepted, "the conflicts are resolved and the tests pass")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})

	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rebase was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	parents := strings.Fields(shipGit(t, repo, "rev-list", "--parents", "-n", "1", "main"))
	if len(parents) != 3 || parents[2] != shipGit(t, repo, "rev-parse", branch) {
		t.Fatalf("main is %v, want a merge commit whose second parent is %s", parents, branch)
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main") // the delivered work is in main
	if h := shipHistory(t, dir); !strings.Contains(h, "delivered branch is now "+branch) {
		t.Fatalf("history does not record the rebased branch %s:\n%s", branch, h)
	}
	if b, _, err := taskMergeKeys(dir); err != nil || b != branch {
		t.Fatalf("task.md branch %q %v, want %s", b, err, branch)
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after landing, want 1", len(rebases))
	}
}

// A rebase Job that is cancelled, or rejected with no rework, puts the ship Job back
// with its person and the reason; nothing lands and nothing is retried on its own.
func TestShipConflictFailedRebaseReopensTheShipJob(t *testing.T) {
	for name, status := range map[string]contract.Status{"cancelled": contract.Cancelled, "rejected": contract.Rejected} {
		t.Run(name, func(t *testing.T) {
			env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
			first := landFirst(t, repo)
			ship, _ := openApproveJob(t, store)
			if code := Run([]string{"approve", "T-01"}, env); code != 0 {
				t.Fatalf("approve: %d %s", code, stderr)
			}
			rebases, _ := rebaseJobs(t, store)
			if len(rebases) != 1 {
				t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), stderr)
			}
			setJobStatus(t, env, store, rebases[0].ID, status, "its tests fail")
			landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})

			st, err := store.State()
			if err != nil {
				t.Fatal(err)
			}
			j := st.Jobs[ship.ID]
			if j == nil || j.Status != contract.Posted {
				t.Fatalf("ship Job %s is %+v after a failed rebase, want posted (back with its person); stderr %s", ship.ID, j, stderr)
			}
			if want := "land failed: rebasing the delivered branch failed: the rebase Job " + rebases[0].ID; !strings.Contains(j.Reason, want) {
				t.Fatalf("ship Job reason %q does not hold %q", j.Reason, want)
			}
			if got := shipGit(t, repo, "rev-parse", "main"); got != first {
				t.Fatalf("main moved to %s after a failed rebase", got)
			}
			if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
				t.Fatalf("task state %q %v, want waiting", rec.State, err)
			}
			// Nothing retries on its own: another pass posts no second rebase Job.
			landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
			if again, _ := rebaseJobs(t, store); len(again) != 1 {
				t.Fatalf("%d rebase Jobs after the failure, want 1", len(again))
			}
		})
	}
}

// A task gets two rebase Jobs. A conflict after that goes back to the person as a
// failed land, as it did before rebase Jobs.
func TestShipConflictStopsAfterTwoRebaseJobs(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	c := &mergeConflictError{Branch: "lane", Target: "main", Files: []string{"a.txt"}}
	for i := 0; i < rebaseTries; i++ {
		if id, err := postRebaseJob(env, store, w, c); err != nil || id == "" {
			t.Fatalf("seed rebase Job %d: %q %v", i+1, id, err)
		}
	}
	if id, err := postRebaseJob(env, store, w, c); err != nil || id != "" {
		t.Fatalf("a third rebase Job was posted: %q %v", id, err)
	}
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("a conflict must not fail the answer: %d %s", code, stderr)
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != rebaseTries {
		t.Fatalf("%d rebase Jobs, want the %d tries and no more", len(rebases), rebaseTries)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := st.Jobs[ship.ID]; j == nil || j.Status != contract.Posted || !strings.Contains(j.Reason, "conflicts in a.txt") {
		t.Fatalf("ship Job %s is %+v after the third conflict, want posted with the conflict as its reason; stderr %s", ship.ID, j, stderr)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v, want waiting", rec.State, err)
	}
}

// A rework of a rebase Job (the Judge posts one that continues it) is the same try: the
// task is not given a third Job, and the land waits for the rework, not for the Job it
// replaced.
func TestShipConflictFollowsTheReworkOfTheRebaseJob(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, snap := rebaseJobs(t, store)
	root := rebases[0]
	body := map[string]any{}
	for k, v := range snap.Bodies[root.ID] {
		body[k] = v
	}
	rework := nextJobID(snap)
	body["id"], body["continues"] = rework, root.ID
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: rework, Body: body}); err != nil {
		t.Fatal(err)
	}
	setJobStatus(t, env, store, root.ID, contract.Rejected, "rework after review round 1")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	if st, _ := store.State(); st.Jobs[rework] == nil || st.Jobs[rework].Status == contract.Accepted {
		t.Fatalf("rework %s is %+v, want open", rework, st.Jobs[rework])
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v while the rework is open, want waiting; history:\n%s", rec.State, err, shipHistory(t, dir))
	}
	if h := shipHistory(t, dir); strings.Count(h, "accepted; grid land runs") != 1 {
		t.Fatalf("a second land ran while the rework was open:\n%s", h)
	}

	branch := fakeBuilder(t, dir, repo, rework)
	setJobStatus(t, env, store, rework, contract.Accepted, "reworked")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rework was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if b, _, err := taskMergeKeys(dir); err != nil || b != branch {
		t.Fatalf("task.md branch %q %v, want the rework's %s", b, err, branch)
	}
}
