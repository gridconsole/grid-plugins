package cli

import (
	"bytes"
	"context"
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
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// These tests pin a land that is never sent back to the person: pickup-test T-02..T-05
// land on one file at about the same time, and the owner was asked again after two
// rebase tries. The approval stands; each conflict gets a rebase Job while the target
// keeps moving under it, and an accepted rebase lands at once.

var landOwner = contract.Author{ID: "local", Role: "owner"}

// noPersonJob fails the test when the ship Job is not accepted, or any other person Job
// (a ship Job put back, a new approval) is open on the Feed.
func noPersonJob(t *testing.T, store feed.Store, ship string, stderr *bytes.Buffer) {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if st := snap.State.Jobs[ship]; st == nil || st.Status != contract.Accepted {
		t.Fatalf("ship Job %s is %+v, want accepted: the approval stands; stderr %s", ship, st, stderr)
	}
	for _, it := range snap.Items {
		if !it.IsPerson() {
			continue
		}
		switch snap.State.Jobs[it.ID].Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
		default:
			t.Fatalf("person Job %s (%s) is open, want none; stderr %s", it.ID, snap.State.Jobs[it.ID].Status, stderr)
		}
	}
}

// A third conflict, after main moved under the second rebase, posts a third rebase Job:
// nothing counts the tries, the ship Job stays accepted and no person Job opens. When
// the third is accepted with main unmoved the task lands with every change.
func TestShipThirdConflictPostsAThirdRebaseJobAndAsksNoPerson(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	from := "lane"
	for try := 0; try < 2; try++ {
		rebases, _ := rebaseJobs(t, store)
		if len(rebases) != try+1 {
			t.Fatalf("%d rebase Jobs on try %d, want %d; stderr %s", len(rebases), try+1, try+1, stderr)
		}
		from = rebaseBuilder(t, dir, repo, rebases[try].ID, from)
		landAnother(t, repo, fmt.Sprintf("moved%d", try), fmt.Sprintf("moved%d", try))
		setJobStatus(t, env, store, rebases[try].ID, contract.Accepted, "resolved")
		landAfterShip(env, dir, landOwner)
	}
	rebases, snap := rebaseJobs(t, store)
	if len(rebases) != 3 {
		t.Fatalf("%d rebase Jobs after the third conflict, want 3; stderr %s; history:\n%s", len(rebases), stderr, shipHistory(t, dir))
	}
	if rebases[2].Continues != nil && *rebases[2].Continues != "" {
		t.Fatalf("the third rebase Job continues %s: main moved, so it is a new try, not a rework", *rebases[2].Continues)
	}
	if st := snap.State.Jobs[rebases[2].ID]; st == nil || st.Status == contract.Accepted || st.Status == contract.Cancelled {
		t.Fatalf("rebase Job %s is %+v, want open", rebases[2].ID, st)
	}
	noPersonJob(t, store, ship.ID, stderr)
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v while the third rebase waits, want waiting", rec.State, err)
	}

	rebaseBuilder(t, dir, repo, rebases[2].ID, from)
	setJobStatus(t, env, store, rebases[2].ID, contract.Accepted, "resolved")
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the third rebase was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nmoved0\nmoved1\nlane" {
		t.Fatalf("main:a.txt is %q, want every task's change", got)
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main")
	if n := len(landGrants(t, store, ship.ID)); n != 1 {
		t.Fatalf("%d accepting controls on the ship Job, want the person's one", n)
	}
}

// landGrants is the accepting controls the Feed holds for the ship Job id.
func landGrants(t *testing.T, store feed.Store, id string) []feed.Entry {
	t.Helper()
	read, err := store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Entry
	for _, r := range read.Records {
		if r.Body["target"] == id && r.Body["status"] == string(contract.Accepted) {
			out = append(out, r)
		}
	}
	return out
}

// A rebase Job is accepted but main did not move and its branch still conflicts: the same
// conflict is that Job's failure, so the next Job continues it (a rework the Judge
// reviews) and is not a new try; the person is not asked, and once the rework is
// accepted the task lands.
func TestShipConflictWithMainUnmovedPostsAReworkOfTheRebaseJob(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after the conflict, want 1; stderr %s", len(rebases), stderr)
	}
	// The builder left a branch that is the delivered one again: it conflicts with the same main.
	fid, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "branch", "grid-"+fid+"-"+rebases[0].ID+"-a1", "lane")
	setJobStatus(t, env, store, rebases[0].ID, contract.Accepted, "resolved")
	landAfterShip(env, dir, landOwner)

	rebases, snap := rebaseJobs(t, store)
	if len(rebases) != 2 || len(rebaseRoots(snap.Items)) != 1 {
		t.Fatalf("%d rebase Jobs, %d roots after the same conflict, want a rework of the first and no new root; stderr %s; history:\n%s", len(rebases), len(rebaseRoots(snap.Items)), stderr, shipHistory(t, dir))
	}
	if c := rebases[1].Continues; c == nil || *c != rebases[0].ID {
		t.Fatalf("the second rebase Job continues %v, want %s", c, rebases[0].ID)
	}
	if rebases[1].ReviewRounds != 1 {
		t.Fatalf("the rework has review_rounds %d, want 1", rebases[1].ReviewRounds)
	}
	noPersonJob(t, store, ship.ID, stderr)

	// A pass while the rework is out posts nothing more.
	landAfterShip(env, dir, landOwner)
	if again, _ := rebaseJobs(t, store); len(again) != 2 {
		t.Fatalf("%d rebase Jobs after another pass, want 2", len(again))
	}

	rebaseBuilder(t, dir, repo, rebases[1].ID, "lane")
	setJobStatus(t, env, store, rebases[1].ID, contract.Accepted, "resolved")
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rework was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
}

// A rebase Job that is cancelled, or rejected with no rework, does not send the ship Job
// back to its person: the approval stands, the land runs again on the next pass, finds
// the same conflict on the same main and posts a rework of the failed Job.
func TestShipConflictFailedRebaseRetriesWithoutAskingThePerson(t *testing.T) {
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
			landAfterShip(env, dir, landOwner)

			noPersonJob(t, store, ship.ID, stderr)
			if got := shipGit(t, repo, "rev-parse", "main"); got != first {
				t.Fatalf("main moved to %s after a failed rebase", got)
			}
			if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
				t.Fatalf("task state %q %v, want waiting", rec.State, err)
			}
			// The next pass lands again: it finds the conflict and posts the next Job.
			landAfterShip(env, dir, landOwner)
			again, _ := rebaseJobs(t, store)
			if len(again) != 2 {
				t.Fatalf("%d rebase Jobs after the retry, want 2; stderr %s; history:\n%s", len(again), stderr, shipHistory(t, dir))
			}
			noPersonJob(t, store, ship.ID, stderr)
		})
	}
}

// A land that cannot post its rebase Job (the pass holds no task lease) does not ask the
// person: the approval stands and the next pass that holds the lease posts it.
func TestShipConflictWithNoLeaseRetriesOnTheNextPass(t *testing.T) {
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
	// The first rebase Job is cancelled: the retry that follows finds the conflict again.
	setJobStatus(t, env, store, rebases[0].ID, contract.Cancelled, "no agent")
	landAfterShipAs(env, dir, landOwner, feed.Writer{})
	landAfterShipAs(env, dir, landOwner, feed.Writer{})
	if again, _ := rebaseJobs(t, store); len(again) != 1 {
		t.Fatalf("%d rebase Jobs while no pass holds the lease, want 1", len(again))
	}
	noPersonJob(t, store, ship.ID, stderr)

	landAfterShip(env, dir, landOwner)
	if again, _ := rebaseJobs(t, store); len(again) != 2 {
		t.Fatalf("%d rebase Jobs after a pass with the lease, want 2; stderr %s; history:\n%s", len(again), stderr, shipHistory(t, dir))
	}
	noPersonJob(t, store, ship.ID, stderr)
}

// An accepted rebase Job lands from the holder's pass though the pass holds no lease and no
// Job is open: the stream does not drop the task, so the land runs at once, with the
// approval already given and before another task's land can move main again.
func TestShipAcceptedRebaseLandsFromAPassWithNoOpenJob(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), stderr)
	}
	fakeBuilder(t, dir, repo, rebases[0].ID)
	setJobStatus(t, env, store, rebases[0].ID, contract.Accepted, "resolved")
	// Nobody holds the task now and no Job is open: the shape that dropped the task.
	freeTaskLease(t, store)

	worker := env
	worker.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "GRID_ROLE":
			return "member"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	pid, start := taskLeaseIdentity(t)
	var out, diag bytes.Buffer
	s := workerStream(worker, "dana", env.Machine, "member", pid, start, &out, &diag)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after one pass, want closed; stderr %s; diag %s; history:\n%s", rec.State, err, stderr, &diag, shipHistory(t, dir))
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main")
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
	noPersonJob(t, store, ship.ID, stderr)
	if n := len(landGrants(t, store, ship.ID)); n != 1 {
		t.Fatalf("%d accepting controls on the ship Job, want the person's one", n)
	}
}

// A land armed to retry (its rebase Job failed and the pass that saw it held no lease) is
// not forgotten when no Job is open and nobody holds the task: the stream's next pass
// visits the task, retries the land and posts the next rebase Job.
func TestShipArmedRetryRunsFromAPassWithNoOpenJob(t *testing.T) {
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
	setJobStatus(t, env, store, rebases[0].ID, contract.Cancelled, "no agent")
	landAfterShipAs(env, dir, landOwner, feed.Writer{}) // no lease: armed to retry, nothing posted
	freeTaskLease(t, store)

	worker := env
	worker.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "GRID_ROLE":
			return "member"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	pid, start := taskLeaseIdentity(t)
	var out, diag bytes.Buffer
	s := workerStream(worker, "dana", env.Machine, "member", pid, start, &out, &diag)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if again, _ := rebaseJobs(t, store); len(again) != 2 {
		t.Fatalf("%d rebase Jobs after the pass, want 2; stderr %s; diag %s; history:\n%s", len(again), stderr, &diag, shipHistory(t, dir))
	}
	noPersonJob(t, store, ship.ID, stderr)
}

// The ship Job an earlier version put back with its person after the second rebase try
// (pickup-test J-7: "Approve the delivered work. Landing failed: main moved again and the
// work still conflicts ... after 2 rebase tries; approve again to retry the land") is
// taken up by the next pass: the person's earlier approval is accepted again, the land
// runs, and the conflict that is still there gets its third rebase Job. Nobody answers.
func TestShipReopenedCardOfAnEarlierVersionLandsWithoutThePerson(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	from := "lane"
	for try := 0; try < 2; try++ {
		rebases, _ := rebaseJobs(t, store)
		from = rebaseBuilder(t, dir, repo, rebases[try].ID, from)
		landAnother(t, repo, fmt.Sprintf("moved%d", try), fmt.Sprintf("moved%d", try))
		setJobStatus(t, env, store, rebases[try].ID, contract.Accepted, "resolved")
		if try == 0 {
			landAfterShip(env, dir, landOwner)
		}
	}
	// What the earlier version wrote when its second try was used: the rebased branch in
	// task.md, then the ship Job back with its person and the Reopened line in the history.
	fid, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	branch, sha, err := rebasedBranch(env, "sample", "T-01", fid, rebases2(t, store)[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := taskWriter(env)
	tasks.Project = "sample"
	if _, err := tasks.RecordBranch(task.WriteOptions{Cwd: env.Cwd, ID: "T-01", Who: landOwner}, branch, sha); err != nil {
		t.Fatal(err)
	}
	w, err := approvalWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	fs := feed.Store{TaskDir: dir, TaskID: store.TaskID, Warn: stderr}
	text := "main moved again and the work still conflicts in a.txt after 2 rebase tries; approve again to retry the land"
	reopenShipJob(env, fs, w, land.Store{Resolver: resolver(env), Project: "sample"}, "T-01", ship.ID, text)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if st := snap.State.Jobs[ship.ID]; st == nil || st.Status != contract.Posted || !strings.Contains(st.Reason, "after 2 rebase tries") {
		t.Fatalf("the card is %+v, want the earlier version's open card", st)
	}

	landAfterShip(env, dir, landOwner) // the next pass

	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 3 {
		t.Fatalf("%d rebase Jobs after the pass, want the third; stderr %s; history:\n%s", len(rebases), stderr, shipHistory(t, dir))
	}
	noPersonJob(t, store, ship.ID, stderr)
	if n := len(landGrants(t, store, ship.ID)); n != 2 {
		t.Fatalf("%d accepting controls on the ship Job, want the person's and the one that takes the card back", n)
	}
	rebaseBuilder(t, dir, repo, rebases[2].ID, from)
	setJobStatus(t, env, store, rebases[2].ID, contract.Accepted, "resolved")
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the third rebase, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main")
}

// A ship Job put back after a deploy command failed is not taken back: that failure is a
// person's to look at, and a pass does not run the command again on its own.
func TestShipReopenedCardOfADeployFailureStaysWithThePerson(t *testing.T) {
	env, dir, _, _, store, stderr := shipMergeFixture(t, false, true)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	w, err := approvalWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	fs := feed.Store{TaskDir: dir, TaskID: store.TaskID, Warn: stderr}
	reopenShipJob(env, fs, w, land.Store{Resolver: resolver(env), Project: "sample"}, "T-01", ship.ID, "grid land exited 1: boom")
	landAfterShip(env, dir, landOwner)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if st := snap.State.Jobs[ship.ID]; st == nil || st.Status != contract.Posted {
		t.Fatalf("the card is %+v after a pass, want it still with its person", st)
	}
}

// rebases2 is the rebase Jobs of the Feed, for a line that has no use for the snapshot.
func rebases2(t *testing.T, store feed.Store) []job.Job {
	t.Helper()
	jobs, _ := rebaseJobs(t, store)
	return jobs
}

// freeTaskLease leaves the task lease of store with no holder, as a worker does when its
// last Job closes.
func freeTaskLease(t *testing.T, store feed.Store) {
	t.Helper()
	data, err := json.Marshal(lease.Tenure{Lease: feed.Lease{Lease: "task", Gen: 0, Fence: 1}, Since: contract.FormatTime(time.Now())})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), "lease", "task.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
