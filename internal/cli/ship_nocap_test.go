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
	"github.com/gridconsole/grid-plugins/internal/judge"
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

// postReview posts the review Job the Judge posts for the builder Job of: a reviewer
// discovered from it. It returns the review's id.
func postReview(t *testing.T, env Env, store feed.Store, of string) string {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	id := nextJobID(snap)
	body := rebaseJobBody(id, &mergeConflictError{Branch: "lane", Target: "main", Files: []string{"a.txt"}}, map[string]any{"kind": "file-exists", "path": "a.txt"})
	body["role"], body["title"], body["discovered_from"] = "reviewer", "Review "+of, of
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: id, Body: body}); err != nil {
		t.Fatalf("post review %s: %v", id, err)
	}
	return id
}

// A rebase Job is accepted but main did not move and its branch still conflicts: the same
// conflict is that Job's failure, so the next Job continues it (a rework, discovered from
// the review of the Job, which the Judge counts as one more round of the chain) and is not
// a new try; the person is not asked, and once the rework is accepted the task lands.
func TestShipConflictWithMainUnmovedPostsAReworkOfTheRebaseJob(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	fid, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Each builder leaves a branch that is the delivered one again: it conflicts with the same main.
	var reviews []string
	var rebases []job.Job
	for try := 0; try < 2; try++ {
		rebases = rebases2(t, store)
		if len(rebases) != try+1 {
			t.Fatalf("%d rebase Jobs on try %d, want %d; stderr %s; history:\n%s", len(rebases), try+1, try+1, stderr, shipHistory(t, dir))
		}
		reviews = append(reviews, postReview(t, env, store, rebases[try].ID))
		shipGit(t, repo, "branch", "grid-"+fid+"-"+rebases[try].ID+"-a1", "lane")
		setJobStatus(t, env, store, rebases[try].ID, contract.Accepted, "resolved")
		landAfterShip(env, dir, landOwner)
	}
	rebases, snap := rebaseJobs(t, store)
	if len(rebases) != 3 || len(rebaseRoots(snap.Items)) != 1 {
		t.Fatalf("%d rebase Jobs, %d roots after the same conflict twice, want two reworks and no new root; stderr %s; history:\n%s", len(rebases), len(rebaseRoots(snap.Items)), stderr, shipHistory(t, dir))
	}
	for i := 1; i < 3; i++ {
		r := rebases[i]
		if r.Continues == nil || *r.Continues != rebases[i-1].ID || r.DiscoveredFrom == nil || *r.DiscoveredFrom != reviews[i-1] {
			t.Fatalf("rework %s continues %v, discovered from %v; want %s and the review %s", r.ID, r.Continues, r.DiscoveredFrom, rebases[i-1].ID, reviews[i-1])
		}
		if r.ReviewRounds != i {
			t.Fatalf("rework %s has review_rounds %d, want %d: the rounds of its chain", r.ID, r.ReviewRounds, i)
		}
	}
	noPersonJob(t, store, ship.ID, stderr)

	// The Judge counts the reworks as one chain: the review of the last holds every round.
	last := postReview(t, env, store, rebases[2].ID)
	snap, err = orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := judge.Verdict(judge.State{Task: store.TaskID, Jobs: snap.State.Jobs, Bodies: snap.Bodies, Records: snap.Records, IDs: snap.JobIDs}, last)
	if err != nil || rep.Reviewed != rebases[2].ID || rep.RoundsUsed != 3 {
		t.Fatalf("the Judge reads review %s as of %s in round %d (%v), want %s and round 3", last, rep.Reviewed, rep.RoundsUsed, err, rebases[2].ID)
	}

	// A pass while the last rework is out posts nothing more.
	landAfterShip(env, dir, landOwner)
	if again, _ := rebaseJobs(t, store); len(again) != 3 {
		t.Fatalf("%d rebase Jobs after another pass, want 3", len(again))
	}

	rebaseBuilder(t, dir, repo, rebases[2].ID, "lane")
	setJobStatus(t, env, store, rebases[2].ID, contract.Accepted, "resolved")
	landAfterShip(env, dir, landOwner)
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rework was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nlane" {
		t.Fatalf("main:a.txt is %q, want both changes", got)
	}
}

// A rebase Job cancelled before it ran, with no review and no decision, does not send the ship
// Job back to its person: the approval stands, the land runs again on the next pass, finds the
// same conflict on the same main and posts a rework of the failed Job.
func TestShipConflictFailedRebaseRetriesWithoutAskingThePerson(t *testing.T) {
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
	setJobStatus(t, env, store, rebases[0].ID, contract.Cancelled, "no agent could take it")
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
	if len(again) != 2 || again[1].Continues == nil || *again[1].Continues != rebases[0].ID {
		t.Fatalf("%d rebase Jobs after the retry, want a rework of %s; stderr %s; history:\n%s", len(again), rebases[0].ID, stderr, shipHistory(t, dir))
	}
	noPersonJob(t, store, ship.ID, stderr)
}

// postDiscovered posts a Job as the Judge shapes one: of role, with title, discovered from a Job.
func postDiscovered(t *testing.T, env Env, store feed.Store, role, title, from string) string {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	id := nextJobID(snap)
	body := rebaseJobBody(id, &mergeConflictError{Branch: "lane", Target: "main", Files: []string{"a.txt"}}, map[string]any{"kind": "file-exists", "path": "a.txt"})
	body["role"], body["title"], body["discovered_from"] = role, title, from
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: id, Body: body}); err != nil {
		t.Fatalf("post %s %s: %v", role, id, err)
	}
	return id
}

// A rebase Job the Judge's decision path owns (it was reviewed and rejected, or a decision Job
// cancelled it) is not posted on again: the land neither retries it nor asks the person, the
// approval stands and the land does not run again while main is where it was. When another task
// lands and main moves, the land runs again and posts a fresh root rebase.
func TestShipConflictRebaseTheJudgeOwnsIsNotPostedOnAgain(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   contract.Status
		decision string // "" none, "review" a decision from the review, "builder" one from the Job (a lost review)
	}{
		{"rejected after its review", contract.Rejected, ""},
		{"cancelled by a decision from its review", contract.Cancelled, "review"},
		{"cancelled by a decision for a lost review", contract.Cancelled, "builder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			head := rebases[0].ID
			review := postReview(t, env, store, head)
			switch tc.decision {
			case "review":
				postDiscovered(t, env, store, "orchestrator", "Decide how "+head+" continues", review)
			case "builder":
				setJobStatus(t, env, store, review, contract.Cancelled, "no agent for the review")
				postDiscovered(t, env, store, "orchestrator", "Decide how "+head+" continues without its review", head)
			}
			setJobStatus(t, env, store, head, tc.status, "the Judge ended it")
			for pass := 0; pass < 3; pass++ {
				landAfterShip(env, dir, landOwner)
			}
			if again, _ := rebaseJobs(t, store); len(again) != 1 {
				t.Fatalf("%d rebase Jobs after three passes, want only %s: the Judge owns it; stderr %s; history:\n%s", len(again), head, stderr, shipHistory(t, dir))
			}
			if n := strings.Count(shipHistory(t, dir), "accepted; grid land runs"); n != 1 {
				t.Fatalf("the land ran %d times, want once; history:\n%s", n, shipHistory(t, dir))
			}
			noPersonJob(t, store, ship.ID, stderr)
			if got := shipGit(t, repo, "rev-parse", "main"); got != first {
				t.Fatalf("main moved to %s", got)
			}
			if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
				t.Fatalf("task state %q %v, want waiting", rec.State, err)
			}

			// Another task lands: main moved, so the land runs again and posts a new root.
			landAnother(t, repo, "moved", "moved")
			landAfterShip(env, dir, landOwner)
			again, snap := rebaseJobs(t, store)
			if len(again) != 2 || again[1].Continues != nil && *again[1].Continues != "" {
				t.Fatalf("%d rebase Jobs after main moved, want a second one that continues nothing; stderr %s; history:\n%s", len(again), stderr, shipHistory(t, dir))
			}
			if st := snap.State.Jobs[again[1].ID]; st == nil || st.Status == contract.Accepted || st.Status == contract.Cancelled {
				t.Fatalf("rebase Job %s is %+v, want open", again[1].ID, st)
			}
			noPersonJob(t, store, ship.ID, stderr)
		})
	}
}

// A land armed to retry (an earlier version's Retry line) on a rebase Job the Judge owns posts
// nothing either, and does not run again on every pass: the approval goes back to waiting for it.
func TestShipConflictRetryArmedOnAJudgedRebaseJobPostsNothing(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	head := rebases[0].ID
	review := postReview(t, env, store, head)
	postDiscovered(t, env, store, "orchestrator", "Decide how "+head+" continues", review)
	setJobStatus(t, env, store, head, contract.Cancelled, "the decision cancelled it")
	if err := (land.Store{Resolver: resolver(env), Project: "sample"}).Retry(env.Cwd, "T-01", ship.ID); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 3; pass++ {
		landAfterShip(env, dir, landOwner)
	}
	if again, _ := rebaseJobs(t, store); len(again) != 1 {
		t.Fatalf("%d rebase Jobs after the armed retry, want only %s; stderr %s; history:\n%s", len(again), head, stderr, shipHistory(t, dir))
	}
	h := shipHistory(t, dir)
	if n := strings.Count(h, "accepted; grid land runs"); n != 2 {
		t.Fatalf("the land ran %d times, want the first and the armed retry only; history:\n%s", n, h)
	}
	if n := strings.Count(h, "waits for rebase "+head); n != 2 {
		t.Fatalf("%d lines wait for %s, want the first and the one that takes the approval back to waiting; history:\n%s", n, head, h)
	}
	noPersonJob(t, store, ship.ID, stderr)
}

// rebaseJudged tells a rebase Job the Judge owns (rejected, or cancelled after it ran, after a
// review or with a decision raised) from one the land may retry (cancelled before it ran with no
// review and no decision), and an accepted or open one is neither.
func TestRebaseJudgedIsTheJudgesJobNotOneTheLandMayRetry(t *testing.T) {
	from := func(id string) *string { return &id }
	review := job.Job{ID: "J-3", Role: "reviewer", Title: "Review J-2", DiscoveredFrom: from("J-2")}
	decision := func(from string) job.Job {
		return job.Job{ID: "J-4", Role: "orchestrator", Title: "Decide how J-2 continues", DiscoveredFrom: &from}
	}
	for _, tc := range []struct {
		name    string
		status  contract.Status
		results []string
		extra   []job.Job
		want    bool
	}{
		{"rejected", contract.Rejected, nil, nil, true},
		{"cancelled before it ran", contract.Cancelled, nil, nil, false},
		{"cancelled after a result", contract.Cancelled, []string{"R-1"}, nil, true},
		{"cancelled with a review", contract.Cancelled, nil, []job.Job{review}, true},
		{"cancelled with a decision from it", contract.Cancelled, nil, []job.Job{decision("J-2")}, true},
		{"cancelled with a decision from its review", contract.Cancelled, nil, []job.Job{review, decision("J-3")}, true},
		{"cancelled with a decision about another Job", contract.Cancelled, nil, []job.Job{decision("J-9")}, false},
		{"accepted", contract.Accepted, []string{"R-1"}, []job.Job{review}, false},
		{"open", contract.Ready, nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := append([]job.Job{{ID: "J-2", Role: "builder", Title: rebaseTitle + "main"}}, tc.extra...)
			jobs := map[string]*contract.Job{"J-2": {ID: "J-2", Status: tc.status, Results: tc.results}}
			for _, it := range tc.extra {
				jobs[it.ID] = &contract.Job{ID: it.ID, Status: contract.Accepted}
			}
			snap := orchestrator.Snapshot{Items: items, State: contract.State{Jobs: jobs}}
			if got := rebaseJudged(snap, "J-2"); got != tc.want {
				t.Fatalf("rebaseJudged = %t, want %t", got, tc.want)
			}
		})
	}
}

// orchestrator.Load lists Jobs by their ids as text, so J-10 comes before J-9: the newest rebase
// Job is the one with the greater number, and of two reworks of a Job the later is the head.
func TestRebaseRootsAreInTheOrderOfTheirNumbers(t *testing.T) {
	rebase := func(id string) job.Job { return job.Job{ID: id, Role: "builder", Title: rebaseTitle + "main"} }
	nine, ten := rebase("J-9"), rebase("J-10")
	rework := rebase("J-11")
	rework.Continues = &ten.ID
	items := []job.Job{ten, rework, nine}
	if got := rebaseRoots(items); len(got) != 2 || got[0] != "J-9" || got[1] != "J-10" {
		t.Fatalf("rebaseRoots = %v, want [J-9 J-10]", got)
	}
	if got := lastRebaseHead(items); got != "J-11" {
		t.Fatalf("lastRebaseHead = %q, want J-11, the rework of the newest root", got)
	}
	root := "J-1"
	nine.Continues, ten.Continues = &root, &root
	if got := rebaseHead([]job.Job{ten, nine, rebase("J-1")}, "J-1"); got != "J-10" {
		t.Fatalf("rebaseHead = %q, want J-10", got)
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

// The card of the pickup-test kind reaches the holder's pass the way the real one did: a
// person's answer with no lease, accepted by the holder's tick, landed from the stream. The
// earlier version put it back with its person under the stream's own writer; the next
// pass takes it back under that writer too, and the approval it carries lands the task.
func TestShipReopenedCardIsTakenBackByTheHoldersPass(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	person := env
	person.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	if code := Run([]string{"approve", "T-01"}, person); code != 0 {
		t.Fatalf("unleased approve: %d %s", code, stderr)
	}
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
	fs := feed.Store{TaskDir: dir, TaskID: store.TaskID, Warn: stderr}
	text := "main moved again and the work still conflicts in a.txt after 2 rebase tries; approve again to retry the land"
	landed := s.Ticked
	s.Ticked = func(d string, w feed.Writer) {
		landed(d, w) // the tick accepted the answer; the land conflicts and queues the rebase
		rebases, _ := rebaseJobs(t, store)
		if len(rebases) != 1 {
			t.Errorf("%d rebase Jobs after the first land, want 1", len(rebases))
			return
		}
		// What the earlier version left: its tries used, the card back with the person.
		body := map[string]any{"target": rebases[0].ID, "status": string(contract.Cancelled), "reason": "done", "close_reason": "done"}
		if _, err := orchestrator.Write(fs, w, feed.Input{Type: "control", Body: body}); err != nil {
			t.Errorf("cancel: %v", err)
			return
		}
		reopenShipJob(worker, fs, w, land.Store{Resolver: resolver(worker), Project: "sample"}, "T-01", ship.ID, text)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if st := snap.State.Jobs[ship.ID]; st == nil || st.Status != contract.Posted || !strings.Contains(st.Reason, "after 2 rebase tries") {
		t.Fatalf("the card is %+v, want the earlier version's open card; stderr %s; diag %s", st, stderr, &diag)
	}

	s.Ticked = landed // the next tick, with the code as it is now
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	noPersonJob(t, store, ship.ID, stderr)
	if again, _ := rebaseJobs(t, store); len(again) != 2 {
		t.Fatalf("%d rebase Jobs after the pass took the card back, want 2; stderr %s; diag %s; history:\n%s", len(again), stderr, &diag, shipHistory(t, dir))
	}
	if n := len(landGrants(t, store, ship.ID)); n != 2 {
		t.Fatalf("%d accepting controls on the ship Job, want the tick's and the one that takes the card back", n)
	}
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
