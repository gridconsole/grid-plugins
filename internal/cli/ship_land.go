package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// acceptedShipJob is the newest ship person Job of the Feed that is accepted,
// or "" when there is none.
func acceptedShipJob(snap orchestrator.Snapshot) string {
	found := ""
	for _, it := range snap.Items {
		if j := snap.State.Jobs[it.ID]; j != nil && j.Status == contract.Accepted && isShipJob(it) {
			found = it.ID
		}
	}
	return found
}

// landAfterShip is the next step of the task in dir once its ship person Job is
// accepted: grid land for the task, once per ship Job (land.Store.AfterShip), as
// who. The holder's pass of grid worker next calls it after its tick, which is
// where an unleased answer is accepted, and grid approve calls it after a leased
// answer. A task that is not waiting, or has no accepted ship Job, gets nothing.
// A land that fails is recorded in the task's history and said on stderr. A merge
// that conflicts never asks the person again: it posts a builder Job that rebases the
// delivered branch (queueRebase), the approval stands, and a later pass lands once that
// Job is accepted (rebaseStep); when no Job can be posted, the land is armed to run
// again on the next pass (land.Store.Retry). Any other failure puts the ship Job back
// with its person, with the failure text as its reason (reopenShipJob), so the task is
// in Needs you again and the person's next approve runs land once more. A ship Job an
// earlier version put back after a conflict is accepted again with the approval it
// held (reacceptShipJob), so the land runs without the person.
func landAfterShip(env Env, dir string, who contract.Author) {
	w, err := approvalWriter(env)
	if err != nil {
		w = feed.Writer{}
	}
	landAfterShipAs(env, dir, who, w)
}

// landAfterShipAs is landAfterShip with the Feed writer w that reopens the ship
// Job after a failed land: the task lease of grid approve, or of the holder's pass.
func landAfterShipAs(env Env, dir string, who contract.Author, w feed.Writer) {
	rec, err := taskstore.Read(dir)
	if err != nil || rec.State != "waiting" {
		return
	}
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return
	}
	fs := feed.Store{TaskDir: dir, TaskID: fid, Warn: env.Stderr}
	snap, err := orchestrator.Load(fs)
	if err != nil {
		return
	}
	job := acceptedShipJob(snap)
	reopened := ""
	if job == "" {
		if reopened = conflictReopenedShipJob(snap); reopened == "" {
			return
		}
		job = reopened
	}
	// An answer accepted while the delivery waits (an app answer the holder's tick
	// accepts) lands nothing: the ship Job goes back to its person, and the land
	// runs on the answer given once the rework is accepted (shipWait). While the
	// review of the delivery is out the answer stands and the tick that finds the
	// delivery accepted lands it.
	if err := shipWait(snap, job); err != nil {
		if shipReviewing(snap, err) {
			return
		}
		if reopened != "" {
			return
		}
		fmt.Fprintf(env.Stderr, "land: %s not run: %s\n", job, oneLineErr(err))
		reopenShipWaiting(env, fs, w, job, err)
		return
	}
	// The task's own record names its project; the folder this runs in never does.
	resolved, err := resolveTaskOperandIn(env, dir, rec.Project)
	if err != nil {
		fmt.Fprintln(env.Stderr, "land: "+oneLineErr(err))
		return
	}
	store := land.Store{Resolver: resolver(env), Who: who, Getenv: env.Getenv, Project: resolved.Project, Merge: func(id string) (string, error) { return landMergeBranch(env, dir, resolved.Project, id) }}
	if reopened != "" && !reacceptShipJob(env, fs, w, store, resolved.ID, reopened, snap) {
		return
	}
	// An approval that waits for a rebase Job lands once the Job is accepted.
	if pending, err := store.RebasePending(env.Cwd, resolved.ID, job); err == nil && pending {
		if rebaseStep(env, store, who, dir, resolved.Project, resolved.ID, job, snap) {
			return
		}
	}
	if _, ran, err := store.AfterShip(context.Background(), env.Cwd, resolved.ID, job); err != nil || !ran {
		if err == nil {
			return
		}
		fmt.Fprintf(env.Stderr, "land: %s after %s: %s\n", resolved.ID, job, oneLineErr(err))
		var failed *land.FailedError
		if errors.As(err, &failed) {
			// A merge that conflicts is rebased by a builder Job and lands again on its
			// own; the person is not asked again for it.
			var conflict *mergeConflictError
			if errors.As(err, &conflict) && len(conflict.Files) > 0 {
				if !queueRebase(env, fs, w, store, resolved.ID, job, conflict) {
					if err := store.Retry(env.Cwd, resolved.ID, job); err != nil {
						fmt.Fprintf(env.Stderr, "land: %s: the retry was not recorded: %s\n", job, oneLineErr(err))
					}
				}
				return
			}
			reopenShipJob(env, fs, w, store, resolved.ID, job, failed.Text)
		}
	}
}

// landArmed reports whether the task in dir has an accepted ship Job whose land is
// still to run: not run yet, or armed again after a conflict (land.Store.Armed). It is
// what keeps the holder's pass on a task that has no open Job, so the land is retried
// on the next pass, and tries it before the pass's other tasks.
func landArmed(env Env, dir string) bool {
	rec, err := taskstore.Read(dir)
	if err != nil || rec.State != "waiting" {
		return false
	}
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return false
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: fid, Warn: io.Discard})
	if err != nil {
		return false
	}
	job := acceptedShipJob(snap)
	if job == "" {
		return false
	}
	resolved, err := resolveTaskOperandIn(env, dir, rec.Project)
	if err != nil {
		return false
	}
	armed, err := land.Store{Resolver: resolver(env), Project: resolved.Project}.Armed(env.Cwd, resolved.ID, job)
	return err == nil && armed
}

// conflictReopenedShipJob is the ship Job an earlier version put back with its person
// after a land that stopped on a merge conflict or on its rebase: a posted ship Job
// whose reopening says "land failed:" with a conflict or a rebase. A deploy command's
// own failure is not it.
func conflictReopenedShipJob(snap orchestrator.Snapshot) string {
	found := ""
	for _, it := range snap.Items {
		j := snap.State.Jobs[it.ID]
		if j == nil || j.Status != contract.Posted || !isShipJob(it) {
			continue
		}
		reason, ok := strings.CutPrefix(reopeningReason(snap, it.ID), "land failed: ")
		if !ok || strings.Contains(reason, "grid land exited") || strings.Contains(reason, "grid land timed out") {
			continue
		}
		if strings.Contains(reason, "conflicts in ") || strings.HasPrefix(reason, "rebasing the delivered branch failed") || strings.HasPrefix(reason, "the rebase Job ") {
			found = it.ID
		}
	}
	return found
}

// reopeningReason is the reason of the newest control that set the Job id posted again
// (reopenShipJob). The Job's own reason is not it: the tick that follows writes "waiting
// for <person>" over it, so the card is told from the Feed's records.
func reopeningReason(snap orchestrator.Snapshot, id string) string {
	reason, latest := "", ""
	for _, r := range snap.Records {
		if r.Type != "control" || r.Body["target"] != id || r.Body["status"] != string(contract.Posted) || r.ID <= latest {
			continue
		}
		if text, ok := r.Body["reason"].(string); ok {
			reason, latest = text, r.ID
		}
	}
	return reason
}

// reacceptShipJob takes back the reopening of the ship Job job, which an earlier
// version put back with its person after a conflict: its land is armed again (the
// Reopened line is the newest) and the Feed holds the person's accepting control,
// whose answer and plan fields it carries over, so the approval that stands lands
// without a new one. It reports false when it did not accept the Job.
func reacceptShipJob(env Env, fs feed.Store, w feed.Writer, store land.Store, id, job string, snap orchestrator.Snapshot) bool {
	if w.Lease != "task" {
		return false
	}
	if armed, err := store.Armed(env.Cwd, id, job); err != nil || !armed {
		return false
	}
	// Record ids are ULIDs, so the greatest is the latest; the Records are a map.
	var prior map[string]any
	latest := ""
	for _, r := range snap.Records {
		if r.Type == "control" && r.Body["target"] == job && r.Body["status"] == string(contract.Accepted) && r.Body["plan_digest"] != nil && r.ID > latest {
			prior, latest = r.Body, r.ID
		}
	}
	if prior == nil {
		return false
	}
	body := map[string]any{"target": job, "status": string(contract.Accepted), "reason": "the person's earlier approval stands: the land failed on a conflict, not on the approval"}
	for _, k := range []string{"answer", "plan_hash", "plan_digest", "lands_hash", "note"} {
		if v, ok := prior[k]; ok {
			body[k] = v
		}
	}
	if _, err := orchestrator.Write(fs, w, feed.Input{Type: "control", Body: body}); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s not accepted again: %s\n", job, oneLineErr(err))
		return false
	}
	refoldAfterWrite(env, fs)
	fmt.Fprintf(env.Stderr, "land: %s accepted again with the person's earlier approval; the land runs\n", job)
	return true
}

// landMergeBranch is what a ship answer does on a project with no deploy topic:
// grid task merge for the task id in project proj, so the delivered branch lands on
// its base. A task with no branch in task.md has nothing to merge and answers "".
func landMergeBranch(env Env, dir, proj, id string) (string, error) {
	branch, _, err := taskMergeKeys(dir)
	if err != nil || branch == "" {
		return "", err
	}
	return taskMerge(env, id, proj)
}

// reopenShipJob puts the ship Job job back with its person after a failed land:
// a control sets it posted again with "land failed: <text>" as its reason, so it
// reads as waiting for its person and the task shows in Needs you. Only then does
// land.Store.Reopened let the next acceptance run land once more; a Job that could
// not be reopened keeps its once-guard, so no pass lands it again on its own. It is
// for a land that failed on something other than a merge conflict (a deploy command that
// exited non-zero, a merge that failed for another reason): one approve runs one land,
// and nothing here retries.
func reopenShipJob(env Env, fs feed.Store, w feed.Writer, store land.Store, id, job, text string) {
	if w.Lease != "task" {
		fmt.Fprintf(env.Stderr, "land: %s stays accepted: reopening it needs the task lease\n", job)
		return
	}
	body := map[string]any{"target": job, "status": string(contract.Posted), "reason": "land failed: " + text}
	if _, err := orchestrator.Write(fs, w, feed.Input{Type: "control", Body: body}); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s not reopened: %s\n", job, oneLineErr(err))
		return
	}
	refoldAfterWrite(env, fs)
	if err := store.Reopened(env.Cwd, id, job); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s reopened, but its history line failed: %s\n", job, oneLineErr(err))
	}
}

// reopenShipWaiting puts the accepted ship Job job back with its person while the
// delivery waits: a control sets it posted again with the wait as its reason. No
// land ran, so the land's once-guard is untouched and the next answer lands.
func reopenShipWaiting(env Env, fs feed.Store, w feed.Writer, job string, why error) {
	if w.Lease != "task" {
		fmt.Fprintf(env.Stderr, "land: %s stays accepted: reopening it needs the task lease\n", job)
		return
	}
	reason := why.Error()
	var waiting *orchestrator.WaitingError
	if errors.As(why, &waiting) {
		reason = waiting.Reason
	}
	body := map[string]any{"target": job, "status": string(contract.Posted), "reason": "the delivery is not accepted: " + reason}
	if _, err := orchestrator.Write(fs, w, feed.Input{Type: "control", Body: body}); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s not reopened: %s\n", job, oneLineErr(err))
		return
	}
	refoldAfterWrite(env, fs)
}

func oneLineErr(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }
