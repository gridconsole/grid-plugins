package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/judge"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// rebaseTitle starts the title of the builder Job a land conflict posts, and is how
// the land step tells it from the task's other Jobs; the app's Judge gives a rework
// of it the same title.
const rebaseTitle = "Rebase the delivered branch on "

// isRebaseJob reports whether it is a rebase Job or a rework of one.
func isRebaseJob(it job.Job) bool {
	return !it.IsPerson() && strings.HasPrefix(strings.TrimSpace(it.Title), rebaseTitle)
}

// byJobNumber is items in the order the Feed posted them, by the number of their J-<n> ids.
// orchestrator.Load lists them by id as text, which puts J-10 before J-9.
func byJobNumber(items []job.Job) []job.Job {
	out := slices.Clone(items)
	slices.SortStableFunc(out, func(a, b job.Job) int {
		return cmp.Or(cmp.Compare(reopenJobNumber(a.ID), reopenJobNumber(b.ID)), strings.Compare(a.ID, b.ID))
	})
	return out
}

// rebaseRoots is the id of each rebase Job that is not a rework, in Feed order: one
// per time the target moved under the delivered branch.
func rebaseRoots(items []job.Job) []string {
	var roots []string
	for _, it := range byJobNumber(items) {
		if isRebaseJob(it) && (it.Continues == nil || *it.Continues == "") && !slices.Contains(roots, it.ID) {
			roots = append(roots, it.ID)
		}
	}
	return roots
}

// rebaseHead is the newest Job of the chain that starts at the rebase Job root: the
// Job itself, or the last rework that continues it, and theirs in turn.
func rebaseHead(items []job.Job, root string) string {
	next := map[string]string{}
	for _, it := range byJobNumber(items) {
		if it.Continues != nil && *it.Continues != "" {
			next[*it.Continues] = it.ID
		}
	}
	head := root
	for range items {
		n := next[head]
		if n == "" {
			break
		}
		head = n
	}
	return head
}

// lastRebaseHead is the newest rebase Job of the Feed, a rework followed to its end, or
// "" when the Feed has none.
func lastRebaseHead(items []job.Job) string {
	roots := rebaseRoots(items)
	if len(roots) == 0 {
		return ""
	}
	return rebaseHead(items, roots[len(roots)-1])
}

// rebaseCount is how many rebase Jobs the Feed holds, roots and reworks.
func rebaseCount(items []job.Job) int {
	n := 0
	for _, it := range items {
		if isRebaseJob(it) {
			n++
		}
	}
	return n
}

// rebaseVerify is the verify of the rebase Job: the test command of the task's
// newest builder Job, so the rebased branch is held to the tests its work was. A task
// with no such command checks that the first conflicting file is still there, and the
// Job's review judges the rest.
func rebaseVerify(items []job.Job, c *mergeConflictError) map[string]any {
	found := map[string]any{"kind": "file-exists", "path": c.Files[0]}
	for _, it := range items {
		if contract.NormalizeRole(it.Role) == "builder" && strings.TrimSpace(it.Verify.Command) != "" && it.Verify.MinTests > 0 {
			found = map[string]any{"command": it.Verify.Command, "min_tests": it.Verify.MinTests}
		}
	}
	return found
}

// rebaseJobBody is the builder Job that keeps a conflicting delivery landing: its
// worktree starts from the base (it has no after, since a worker refuses a Job whose
// dependency branch conflicts), and it merges the delivered branch in, keeping both
// sides. Its acceptance rows say what to do and how it is checked.
func rebaseJobBody(id string, c *mergeConflictError, verify map[string]any) map[string]any {
	files := strings.Join(c.Files, ", ")
	// The merge brings in every file the delivered branch changed, so the Job may write them all, the conflicting ones first.
	write := make([]any, 0, len(c.Files)+len(c.Changed))
	seen := map[string]bool{}
	for _, f := range append(slices.Clone(c.Files), c.Changed...) {
		if !seen[f] {
			seen[f] = true
			write = append(write, f)
		}
	}
	pointer := "intent.md#request" // task.md has no Outcome heading until delivery
	body := map[string]any{
		"id": id, "item": 1, "role": "builder",
		"title":  fmt.Sprintf("%s%s and resolve the conflicts in %s", rebaseTitle, c.Target, files),
		"skills": []any{}, "risk": []any{},
		"intent": map[string]any{"owner": pointer, "item": pointer, "goal": pointer},
		"links":  []any{}, "keep": []any{}, "decisions": []any{},
		"read": []any{}, "write": write, "out_of_scope": []any{},
		"acceptance": []any{
			map[string]any{"id": "A1", "traces": "item", "text": fmt.Sprintf("Your branch holds the work of %s on top of %s, with the conflicts in %s resolved and both changes kept: merge %s into your worktree, which starts from %s, resolve each conflict so the code of %s and the code %s gained both stay, and commit. Nothing of either side is dropped.", c.Branch, c.Target, files, c.Branch, c.Target, c.Branch, c.Target)},
			map[string]any{"id": "A2", "traces": "item", "text": "The tests pass again on your branch: run the Job's verify and every test the two changes added; a test that fails because of the merge is fixed, never removed."},
		},
		"verify":     verify,
		"budget":     map[string]any{"turns_estimate": 12, "reads_before_first_edit": 3},
		"on_blocked": "ask-and-continue", "after": []any{},
		"attempt": 1, "continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": nil, "posts_back": []any{"result", "question"},
	}
	if c.Repo != "" {
		body["repo"] = c.Repo
	}
	return body
}

// postRebaseJob posts the rebase Job of the conflict c as the task writer w and
// returns its id. A task write under the lease, as every Job a delivery posts is; the
// id is chosen again when another writer took it first. again is true when the target
// has not moved since the last rebase Job was queued: the conflict is then the last
// Job's own failure, so the Job posted is a rework of it (it continues it, with the
// conflict as one more acceptance row), which the Judge reviews as it does any rework.
// When that last Job ended in the Judge's hands (rebaseJudged) nothing is posted: posted
// is false and id is that Job, which the Judge's decision path continues or ends. An
// accepted last Job is not reworked either: its branch was the land's to merge, so the
// conflict goes to one decision Job raised from it (decideRebase), and posted is false.
func postRebaseJob(env Env, store feed.Store, w feed.Writer, c *mergeConflictError, again bool) (id string, posted bool, err error) {
	for try := 0; try < 3; try++ {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return "", false, err
		}
		count := rebaseCount(snap.Items)
		id := nextJobID(snap)
		body := rebaseJobBody(id, c, rebaseVerify(snap.Items, c))
		if head := lastRebaseHead(snap.Items); again && head != "" {
			if rebaseJudged(snap, head) {
				return head, false, nil
			}
			if st := snap.State.Jobs[head]; st != nil && st.Status == contract.Accepted {
				why := fmt.Sprintf("Rebase Job %s was accepted, but the land of %s still conflicts with %s in %s at %s, the commit %s pointed at when %s was queued.", head, c.Branch, c.Target, strings.Join(c.Files, ", "), shortTip(c.TargetSHA), c.Target, head)
				return head, false, decideRebase(env, store, w, snap, head, c.Target, why)
			}
			reworkRebaseJob(body, c, head, snap)
		}
		data, err := json.Marshal(body)
		if err != nil {
			return "", false, err
		}
		if err := contract.Validate("job-record", data); err != nil {
			return "", false, err
		}
		skipped := false
		_, err = orchestrator.WriteUnless(store, w, feed.Input{Type: "job", Job: id, Body: body}, func() bool {
			s, err := orchestrator.Load(store)
			if err == nil && (rebaseCount(s.Items) != count || s.State.Jobs[id] != nil || slices.Contains(s.JobIDs, id)) {
				skipped = true
			}
			return skipped
		})
		if err != nil {
			return "", false, err
		}
		if !skipped {
			return id, true, nil
		}
	}
	return "", false, errors.New("the Feed changed while the Job was being posted")
}

// shortTip is the first 12 characters of the commit tip, as the land's lines name one.
func shortTip(tip string) string { return tip[:min(len(tip), 12)] }

// reworkRebaseJob makes body, a rebase Job, the rework of the rebase Job head, which
// ended before it was accepted and not in the Judge's hands (cancelled before it ran),
// while the delivered branch still conflicts with the target at the commit the last
// rebase was queued on. It is a rework as the Judge makes one: it continues head and
// carries the review rounds of the whole chain (judge.State.roundsOf); such a head has no
// review of its own (one with a review is rebaseJudged), so it is discovered from none.
func reworkRebaseJob(body map[string]any, c *mergeConflictError, head string, snap orchestrator.Snapshot) {
	rows, _ := body["acceptance"].([]any)
	tip := shortTip(c.TargetSHA)
	body["acceptance"] = append(rows, map[string]any{"id": "A3", "traces": "item", "text": fmt.Sprintf("Must-fix from the land: after rebase Job %s, %s still conflicts in %s with %s at %s, and %s has not moved since that rebase. Rebase %s on %s again and check that it merges into %s with no conflict before you finish.", head, c.Branch, strings.Join(c.Files, ", "), c.Target, tip, c.Target, c.Branch, c.Target, c.Target)})
	body["continues"] = head
	if reviews := rebaseReviews(snap, head); len(reviews) > 0 {
		body["discovered_from"] = reviews[len(reviews)-1]
	}
	body["review_rounds"] = rebaseRounds(snap, head)
}

// isReviewJob reports whether it is a review Job: a reviewer or a checker, as the Judge
// reads one (judge.isReviewer).
func isReviewJob(it job.Job) bool {
	role := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(it.Role)), "grid-")
	return role == "reviewer" || role == "checker"
}

// rebaseReviews is the review Jobs of the Job id, oldest first, as the Judge reads them
// (judge.State.reviewsOf): reviews discovered from it or posted after it, except one that
// still waits on it.
func rebaseReviews(snap orchestrator.Snapshot, id string) []string {
	var out []string
	for _, it := range snap.Items {
		st := snap.State.Jobs[it.ID]
		if !isReviewJob(it) || it.ID == id || st == nil {
			continue
		}
		waits := slices.ContainsFunc(it.After, func(d job.Dependency) bool { return d.Job == id })
		if waits && (st.Status == contract.Posted || st.Status == contract.Ready) {
			continue
		}
		if waits || it.DiscoveredFrom != nil && *it.DiscoveredFrom == id {
			out = append(out, it.ID)
		}
	}
	slices.Sort(out)
	return out
}

// rebaseRounds is the review rounds over the chain of the Job head, as the Judge counts
// them (judge.State.roundsOf): the reviews of head and of each Job it continues, for as
// long as the Job was discovered from a review.
func rebaseRounds(snap orchestrator.Snapshot, head string) int {
	byID := map[string]job.Job{}
	for _, it := range snap.Items {
		if _, ok := byID[it.ID]; !ok {
			byID[it.ID] = it
		}
	}
	n := 0
	for id, seen := head, map[string]bool{}; id != "" && !seen[id]; {
		seen[id] = true
		n += len(rebaseReviews(snap, id))
		it := byID[id]
		if it.DiscoveredFrom == nil || it.Continues == nil || !isReviewJob(byID[*it.DiscoveredFrom]) {
			break
		}
		id = *it.Continues
	}
	return n
}

// queueRebase answers a land that stopped on the merge conflict c: it posts the
// rebase Job, records that the person's approval of the ship Job ship waits for it
// (land.Store.RebaseQueued), and reports true. It reports false when no rebase Job
// can be posted (no task lease, the post failed), so the caller arms the land to run
// again on the next pass (land.Store.Retry) with the approval standing.
//
// What ends a chain of rebase Jobs is that the target has moved: a conflict with the
// target at the commit the last rebase Job was queued on is that Job's own failure, not
// a new try; a new try needs the target to have moved under it, which another task's
// land does, and the tasks are finite. The land reworks such a Job only when it ended
// before it ran. A last Job the Judge owns (rebaseJudged) gets nothing from the land, and
// an accepted one gets one decision Job (decideRebase), after which the Judge owns it:
// the approval waits for the decision path, and for the target to move.
func queueRebase(env Env, fs feed.Store, w feed.Writer, store land.Store, id, ship string, c *mergeConflictError) (queued bool) {
	if w.Lease != "task" {
		fmt.Fprintf(env.Stderr, "land: %s: %s conflicts with %s in %s; no task lease to post the rebase Job, the next pass retries\n", ship, c.Branch, c.Target, strings.Join(c.Files, ", "))
		return false
	}
	_, tip, _ := store.RebaseTip(env.Cwd, id, ship)
	again := tip != "" && strings.HasPrefix(c.TargetSHA, tip)
	rebase, posted, err := postRebaseJob(env, fs, w, c, again)
	if err != nil {
		fmt.Fprintf(env.Stderr, "land: %s: the rebase Job was not posted, the next pass retries: %s\n", ship, oneLineErr(err))
		return false
	}
	if posted {
		refoldAfterWrite(env, fs)
	}
	if err := store.RebaseQueued(env.Cwd, id, ship, rebase, c.TargetSHA); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s: rebase Job %s posted, but its history line failed: %s\n", ship, rebase, oneLineErr(err))
		return false
	}
	if !posted {
		fmt.Fprintf(env.Stderr, "land: %s: %s conflicts with %s in %s; rebase Job %s is left to its decision Job, the approval waits for it\n", ship, c.Branch, c.Target, strings.Join(c.Files, ", "), rebase)
		return true
	}
	fmt.Fprintf(env.Stderr, "land: %s: %s conflicts with %s in %s; rebase Job %s posted, the approval stands\n", ship, c.Branch, c.Target, strings.Join(c.Files, ", "), rebase)
	return true
}

// rebaseJudged reports whether the Judge's decision path owns the closed rebase Job
// head: it was rejected, or it was cancelled after it ran, after a review of it or with a
// decision Job raised for it, or it was accepted and the land raised its decision Job
// from it (landDecided). The decision path continues that chain or ends it, so the land
// posts nothing on top of it; a Job cancelled before it ran, with no review and no
// decision, is not the Judge's (rebaseStep retries it), an open one is not closed, and
// an accepted one is the land's to merge until the land hands it on.
func rebaseJudged(snap orchestrator.Snapshot, head string) bool {
	st := snap.State.Jobs[head]
	if st == nil {
		return false
	}
	switch st.Status {
	case contract.Rejected:
		return true
	case contract.Cancelled:
		return len(st.Results) > 0 || len(rebaseReviews(snap, head)) > 0 || rebaseDecided(snap, head)
	case contract.Accepted:
		return landDecided(snap, head)
	}
	return false
}

// landDecisionTitle is the title of the decision Job the land raises from the accepted
// rebase Job head whose delivery still cannot land on the target it was queued on.
func landDecisionTitle(head string) string {
	return "Decide how " + head + " continues: its accepted rebase does not land"
}

// landDecided reports whether the land raised its decision Job from the rebase Job head.
// A decision the Judge raised from it (a lost review's) is not that one: a rebase Job such
// a decision accepted is still the land's to merge.
func landDecided(snap orchestrator.Snapshot, head string) bool {
	return slices.ContainsFunc(snap.Items, func(it job.Job) bool {
		return !it.IsPerson() && it.DiscoveredFrom != nil && *it.DiscoveredFrom == head && strings.TrimSpace(it.Title) == landDecisionTitle(head)
	})
}

// decideRebase hands the accepted rebase Job head, whose delivery cannot land on target
// at the commit head was queued on for the reason why, to an agent decision Job raised
// from it as the Judge raises one (judge.Decide), once per head: a decision the land
// raised from head already, or one another process writes first, posts nothing.
func decideRebase(env Env, store feed.Store, w feed.Writer, snap orchestrator.Snapshot, head, target, why string) error {
	if target == "" {
		target = "the target branch"
	}
	goal := why + fmt.Sprintf(" %s has not moved since, so the land neither reworks %s nor retries its land by itself, and the approval of the delivery waits for this decision.\n\n"+
		"Cancelling %s, or leaving it accepted as it is, parks the land until %s moves, when it runs again by itself. Posting a rework of %s, or a new %q Job, resumes the land once that Job is accepted.",
		target, head, head, target, head, rebaseTitle+target)
	plan, err := judge.Decide(judge.State{Task: store.TaskID, Jobs: snap.State.Jobs, Bodies: snap.Bodies, Records: snap.Records, IDs: snap.JobIDs}, head, landDecisionTitle(head), goal)
	if err != nil || len(plan.Posts) == 0 {
		return err
	}
	p := plan.Posts[0]
	rec, err := orchestrator.WriteUnless(store, w, feed.Input{ID: p.ID(), Type: p.Type, Job: p.Job, Body: p.Body}, func() bool {
		s, err := orchestrator.Load(store)
		return err == nil && landDecided(s, head)
	})
	if err == nil && rec.ID != "" {
		refoldAfterWrite(env, store)
	}
	return err
}

// rebaseDecided reports whether a decision Job ("Decide how ... continues", the Judge's
// judge.Person) was raised from the rebase Job head or from a review of it.
func rebaseDecided(snap orchestrator.Snapshot, head string) bool {
	from := append([]string{head}, rebaseReviews(snap, head)...)
	return slices.ContainsFunc(snap.Items, func(it job.Job) bool {
		return it.DiscoveredFrom != nil && slices.Contains(from, *it.DiscoveredFrom) && strings.HasPrefix(strings.TrimSpace(it.Title), "Decide how ")
	})
}

// rebaseStep is a pass over a ship Job whose approval waits for its rebase Job. It
// reports true when the pass is over: the rebase is still out, or it failed and the
// land is armed to run again (land.Store.Retry), which finds the conflict again and
// posts a rework of the failed Job; the person is not asked. Retry is only for a Job
// cancelled before it ran: one the Judge owns (rebaseJudged) is left to its decision
// path, and the approval waits until the target moves, when it reports false so the land
// runs and posts a fresh rebase Job. It reports false too when the rebase Job is accepted
// and task.md names its branch, so land runs again on the approval. An accepted Job is
// never retried: when its branch is not in this checkout and the target has not moved,
// it gets one decision Job (decideRebase) as the task writer w, and the approval waits as
// it does for a Job the Judge owns; once the target moved the land runs on the branch
// task.md names.
func rebaseStep(env Env, fs feed.Store, w feed.Writer, store land.Store, who contract.Author, proj, id, ship string, snap orchestrator.Snapshot) bool {
	fail := func(text string) bool {
		fmt.Fprintf(env.Stderr, "land: %s: %s; the land runs again on the next pass\n", ship, text)
		if err := store.Retry(env.Cwd, id, ship); err != nil {
			fmt.Fprintf(env.Stderr, "land: %s: the retry was not recorded: %s\n", ship, oneLineErr(err))
		}
		return true
	}
	roots := rebaseRoots(snap.Items)
	if len(roots) == 0 {
		return fail("the rebase Job is not on the Feed")
	}
	head := rebaseHead(snap.Items, roots[len(roots)-1])
	st := snap.State.Jobs[head]
	if st == nil {
		return true
	}
	switch st.Status {
	case contract.Accepted:
		if rebaseJudged(snap, head) {
			_, moved := rebaseMoved(env, store, proj, id, ship)
			return !moved
		}
	case contract.Cancelled, contract.Rejected:
		if !rebaseJudged(snap, head) {
			return fail(fmt.Sprintf("rebasing the delivered branch failed: the rebase Job %s was cancelled", head))
		}
		_, moved := rebaseMoved(env, store, proj, id, ship)
		return !moved
	default:
		return true
	}
	branch, sha, err := rebasedBranch(env, proj, id, fs.TaskID, head)
	if err != nil {
		target, moved := rebaseMoved(env, store, proj, id, ship)
		if moved {
			fmt.Fprintf(env.Stderr, "land: %s: %s; the target moved since, so the land runs on the branch task.md names\n", ship, oneLineErr(err))
			return false
		}
		if w.Lease != "task" {
			fmt.Fprintf(env.Stderr, "land: %s: %s; no task lease to post its decision Job, the next pass posts it\n", ship, oneLineErr(err))
			return true
		}
		why := fmt.Sprintf("Rebase Job %s was accepted, but the land has no branch of it to merge: %s. Its builder may have run on another machine, and the land brings no branch here from one.", head, oneLineErr(err))
		if err := decideRebase(env, fs, w, snap, head, target, why); err != nil {
			fmt.Fprintf(env.Stderr, "land: %s: the decision Job for rebase Job %s was not posted, the next pass posts it: %s\n", ship, head, oneLineErr(err))
			return true
		}
		fmt.Fprintf(env.Stderr, "land: %s: %s; rebase Job %s is left to its decision Job, the approval waits for it\n", ship, oneLineErr(err), head)
		return true
	}
	// task.md naming no branch here lands the builder branches, the rebased one in its original's place.
	if staleBranchKey(env, proj, id) {
		return false
	}
	// The task's project decides which T-NN this is, as it does for land.
	tasks := taskWriter(env)
	tasks.Project = proj
	if _, err := tasks.RecordBranch(taskstore.WriteOptions{Cwd: env.Cwd, ID: id, Who: who, Note: "delivered branch is now " + branch + " (rebase Job " + head + ")"}, branch, sha); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s: the rebased branch %s was not recorded: %s\n", ship, branch, oneLineErr(err))
		return true
	}
	return false
}

// rebaseMoved reports whether the target the task's delivered branch lands on has moved
// since the newest rebase line of the approval ship (land.Store.RebaseTip), and names that
// target. A line with no tip, as an earlier version wrote, counts as moved, so it never
// parks the land for good; a target that cannot be read counts as unmoved.
func rebaseMoved(env Env, store land.Store, proj, id, ship string) (target string, moved bool) {
	_, tip, _ := store.RebaseTip(env.Cwd, id, ship)
	target, now, err := taskTargetTip(env, proj, id)
	// Every builder branch in its target, as a person's hand merge leaves it, counts as moved.
	return target, tip == "" || errors.Is(err, errAllLanded) || err == nil && !strings.HasPrefix(now, tip)
}

// staleBranchKey reports whether task.md names no branch or one no repo of the project holds.
func staleBranchKey(env Env, proj, id string) bool {
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: id, Project: proj})
	if err != nil {
		return false
	}
	branch, _, err := taskMergeKeys(res.TaskPath)
	if err != nil {
		return false
	}
	if branch == "" {
		return true
	}
	var missing *branchMissingError
	_, err = taskMergeRepo(res, branch)
	return errors.As(err, &missing)
}

// taskTargetTip is the branch the task's delivered branch lands on and the commit it
// points at now, found as taskMerge finds them.
func taskTargetTip(env Env, proj, id string) (target, tip string, err error) {
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: id, Project: proj})
	if err != nil {
		return "", "", err
	}
	branch, base, err := taskMergeKeys(res.TaskPath)
	if err != nil {
		return "", "", err
	}
	repo, err := taskMergeRepo(res, branch)
	var missing *branchMissingError
	if branch == "" || errors.As(err, &missing) {
		// A branch no repo holds: the land waits on the first builder branch not in.
		built, rebased, noWrite, berr := builderBranches(res.TaskPath)
		if berr != nil {
			return "", "", berr
		}
		repo, branch, err = firstUnlanded(res, base, built, rebased, noWrite)
	}
	if err != nil {
		return "", "", err
	}
	if target = taskMergeTarget(repo, base); target == "" {
		return "", "", fmt.Errorf("no base branch for %s in %s", branch, repo)
	}
	tip, err = taskMergeGit(repo, "rev-parse", "refs/heads/"+target)
	return target, tip, err
}

// rebasedBranch is the branch the worker made for the rebase Job head of the task:
// grid-<feed id>-<job>-a<attempt>, the newest attempt in the project's checkout, and
// the commit it points at.
func rebasedBranch(env Env, proj, id, fid, head string) (branch, sha string, err error) {
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: id, Project: proj})
	if err != nil {
		return "", "", err
	}
	if res.Folder == "" {
		return "", "", fmt.Errorf("project %s has no folder on this machine", res.Project)
	}
	prefix := "grid-" + fid + "-" + head + "-a"
	dirs := []string{res.Folder}
	for _, r := range res.Repos {
		if !filepath.IsAbs(r) {
			r = filepath.Join(res.Folder, r)
		}
		dirs = append(dirs, r)
	}
	for _, dir := range dirs {
		out, err := taskMergeGit(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix+"*")
		if err != nil {
			continue
		}
		best, newest := "", 0
		for _, name := range strings.Fields(out) {
			if n, err := strconv.Atoi(strings.TrimPrefix(name, prefix)); err == nil && n >= newest {
				best, newest = name, n
			}
		}
		if best == "" {
			continue
		}
		sha, err := taskMergeGit(dir, "rev-parse", "refs/heads/"+best)
		return best, sha, err
	}
	return "", "", fmt.Errorf("no branch %s* in the checkout of project %s; the rebase Job %s left no work to land", prefix, res.Project, head)
}
