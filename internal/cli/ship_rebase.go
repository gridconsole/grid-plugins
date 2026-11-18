package cli

import (
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
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// rebaseTitle starts the title of the builder Job a land conflict posts, and is how
// the land step tells it from the task's other Jobs; the app's Judge gives a rework
// of it the same title.
const rebaseTitle = "Rebase the delivered branch on "

// rebaseTries is how many rebase Jobs one task gets; a conflict after that goes back
// to the person as a failed land.
const rebaseTries = 2

// isRebaseJob reports whether it is a rebase Job or a rework of one.
func isRebaseJob(it job.Job) bool {
	return !it.IsPerson() && strings.HasPrefix(strings.TrimSpace(it.Title), rebaseTitle)
}

// rebaseRoots is the id of each rebase Job that is not a rework, in Feed order: one
// per try.
func rebaseRoots(items []job.Job) []string {
	var roots []string
	for _, it := range items {
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
	for _, it := range items {
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

// rebaseVerify is the verify of the rebase Job: the test command of the task's
// newest builder Job, so the rebased branch is held to the tests its work was. A task
// with no such command checks that the first conflicting file is still there, and the
// Job's review judges the rest.
func rebaseVerify(items []job.Job, c *mergeConflictError) map[string]any {
	found := map[string]any{"kind": "file-exists", "path": c.Files[0]}
	for _, it := range items {
		if strings.EqualFold(strings.TrimSpace(it.Role), "builder") && strings.TrimSpace(it.Verify.Command) != "" && it.Verify.MinTests > 0 {
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
	write := make([]any, 0, len(c.Files))
	for _, f := range c.Files {
		write = append(write, f)
	}
	pointer := "task.md#outcome"
	return map[string]any{
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
}

// postRebaseJob posts the rebase Job of the conflict c as the task writer w and
// returns its id, or "" when the task has used its tries (rebaseTries). A task write
// under the lease, as every Job a delivery posts is; the id is chosen again when
// another writer took it first.
func postRebaseJob(env Env, store feed.Store, w feed.Writer, c *mergeConflictError) (string, error) {
	for try := 0; try < 3; try++ {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return "", err
		}
		tries := len(rebaseRoots(snap.Items))
		if tries >= rebaseTries {
			return "", nil
		}
		id := nextJobID(snap)
		body := rebaseJobBody(id, c, rebaseVerify(snap.Items, c))
		data, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		if err := contract.Validate("job-record", data); err != nil {
			return "", err
		}
		skipped := false
		_, err = orchestrator.WriteUnless(store, w, feed.Input{Type: "job", Job: id, Body: body}, func() bool {
			s, err := orchestrator.Load(store)
			if err == nil && (len(rebaseRoots(s.Items)) != tries || s.State.Jobs[id] != nil || slices.Contains(s.JobIDs, id)) {
				skipped = true
			}
			return skipped
		})
		if err != nil {
			return "", err
		}
		if !skipped {
			return id, nil
		}
	}
	return "", errors.New("the Feed changed while the Job was being posted")
}

// queueRebase answers a land that stopped on the merge conflict c: it posts the
// rebase Job, records that the person's approval of the ship Job ship waits for it
// (land.Store.RebaseQueued), and reports true. It reports false when no rebase Job
// can be posted (no task lease, the tries are used, the post failed), so the caller
// puts the ship Job back with its person.
func queueRebase(env Env, fs feed.Store, w feed.Writer, store land.Store, id, ship string, c *mergeConflictError) bool {
	if w.Lease != "task" || len(c.Files) == 0 {
		return false
	}
	rebase, err := postRebaseJob(env, fs, w, c)
	if err != nil {
		fmt.Fprintf(env.Stderr, "land: %s: the rebase Job was not posted: %s\n", ship, oneLineErr(err))
		return false
	}
	if rebase == "" {
		fmt.Fprintf(env.Stderr, "land: %s: %d rebase Jobs were tried; %s still conflicts\n", ship, rebaseTries, c.Branch)
		return false
	}
	refoldAfterWrite(env, fs)
	if err := store.RebaseQueued(env.Cwd, id, ship, rebase); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s: rebase Job %s posted, but its history line failed: %s\n", ship, rebase, oneLineErr(err))
		return false
	}
	fmt.Fprintf(env.Stderr, "land: %s: %s conflicts with %s in %s; rebase Job %s posted, the approval stands\n", ship, c.Branch, c.Target, strings.Join(c.Files, ", "), rebase)
	return true
}

// rebaseStep is a pass over a ship Job whose approval waits for its rebase Job. It
// reports true when the pass is over: the rebase is still out, or it failed and the
// ship Job went back to its person with the reason. It reports false when the rebase
// Job is accepted and task.md names its branch, so land runs again on the approval.
func rebaseStep(env Env, fs feed.Store, w feed.Writer, store land.Store, who contract.Author, dir, proj, id, ship string, snap orchestrator.Snapshot) bool {
	fail := func(text string) bool {
		fmt.Fprintf(env.Stderr, "land: %s: %s\n", ship, text)
		reopenShipJob(env, fs, w, store, id, ship, text)
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
	case contract.Cancelled:
		return fail(fmt.Sprintf("rebasing the delivered branch failed: the rebase Job %s was cancelled", head))
	case contract.Rejected:
		return fail(fmt.Sprintf("rebasing the delivered branch failed: the rebase Job %s was rejected and no rework follows", head))
	default:
		return true
	}
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return fail(err.Error())
	}
	branch, sha, err := rebasedBranch(env, proj, id, fid, head)
	if err != nil {
		return fail("rebasing the delivered branch failed: " + oneLineErr(err))
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
