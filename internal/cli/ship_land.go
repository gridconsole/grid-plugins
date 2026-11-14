package cli

import (
	"context"
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
// fails the caller, and it posts no Job.
func landAfterShip(env Env, dir string, who contract.Author) {
	rec, err := taskstore.Read(dir)
	if err != nil || rec.State != "waiting" {
		return
	}
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: fid})
	if err != nil {
		return
	}
	job := acceptedShipJob(snap)
	if job == "" {
		return
	}
	resolved, err := resolveTaskOperand(env, dir)
	if err != nil {
		fmt.Fprintln(env.Stderr, "land: "+oneLineErr(err))
		return
	}
	store := land.Store{Resolver: resolver(env), Who: who, Getenv: env.Getenv}
	if _, ran, err := store.AfterShip(context.Background(), env.Cwd, resolved.ID, job); err != nil || !ran {
		if err == nil {
			return
		}
		fmt.Fprintf(env.Stderr, "land: %s after %s: %s\n", resolved.ID, job, oneLineErr(err))
	}
}

func oneLineErr(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }
