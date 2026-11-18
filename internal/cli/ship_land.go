package cli

import (
	"context"
	"errors"
	"fmt"
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
// A land that fails is recorded in the task's history and said on stderr; it never
// fails the caller, and it posts no Job. It also puts the ship Job back with its
// person, with the failure text as its reason (reopenShipJob), so the task is in
// Needs you again and the person's next approve runs land once more.
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
	if job == "" {
		return
	}
	// An answer accepted while the delivery waits (an app answer the holder's tick
	// accepts) lands nothing: the ship Job goes back to its person, and the land
	// runs on the answer given once the rework is accepted (shipWait).
	if err := shipWait(snap, job); err != nil {
		fmt.Fprintf(env.Stderr, "land: %s not run: %s\n", job, oneLineErr(err))
		reopenShipWaiting(env, fs, w, job, err)
		return
	}
	resolved, err := resolveTaskOperand(env, dir)
	if err != nil {
		fmt.Fprintln(env.Stderr, "land: "+oneLineErr(err))
		return
	}
	store := land.Store{Resolver: resolver(env), Who: who, Getenv: env.Getenv, Merge: func(id string) (string, error) { return landMergeBranch(env, dir, id) }}
	if _, ran, err := store.AfterShip(context.Background(), env.Cwd, resolved.ID, job); err != nil || !ran {
		if err == nil {
			return
		}
		fmt.Fprintf(env.Stderr, "land: %s after %s: %s\n", resolved.ID, job, oneLineErr(err))
		var failed *land.FailedError
		if errors.As(err, &failed) {
			reopenShipJob(env, fs, w, store, resolved.ID, job, failed.Text)
		}
	}
}

// landMergeBranch is what a ship answer does on a project with no deploy topic:
// grid task merge for the task id in dir, so the delivered branch lands on its
// base. A task with no branch in task.md has nothing to merge and answers "".
func landMergeBranch(env Env, dir, id string) (string, error) {
	branch, _, err := taskMergeKeys(dir)
	if err != nil || branch == "" {
		return "", err
	}
	return taskMerge(env, id, "")
}

// reopenShipJob puts the ship Job job back with its person after a failed land:
// a control sets it posted again with "land failed: <text>" as its reason, so it
// reads as waiting for its person and the task shows in Needs you. Only then does
// land.Store.Reopened let the next acceptance run land once more; a Job that could
// not be reopened keeps its once-guard, so no pass lands it again on its own. One
// approve runs one land; nothing here retries.
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
