package cli

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// openApprovePersonJob is the first approve person Job of the Feed that is not
// accepted, cancelled or rejected, or "" when there is none.
func openApprovePersonJob(snap orchestrator.Snapshot) string {
	for _, it := range snap.Items {
		if !it.IsPerson() || it.Kind != "approve" {
			continue
		}
		j := snap.State.Jobs[it.ID]
		if j == nil || j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
			continue
		}
		return it.ID
	}
	return ""
}

// taskJobWriter is the Feed writer of a Job a delivery posts: a task write under
// the task lease. A Job agent's GRID_LEASE_EPOCH is its Job's, so the task epoch
// wins when GRID_LEASE_EPOCH was not typed, as `job post` does.
func taskJobWriter(env Env) (feed.Writer, error) {
	epoch := env.getenv("GRID_LEASE_EPOCH")
	if te := env.getenv("GRID_TASK_EPOCH"); te != "" && !env.typedGlobal["GRID_LEASE_EPOCH"] {
		epoch = te
	}
	return orchestratorWriter(env, "task", epoch)
}

// postShipPersonJob posts the person step that follows Check: the plan approve
// Job (migrate.PlanJobBody) for the person of the delivered task in dir, the
// owner when task.md names none. grid approve and the app answer it as any plan
// approve Job, and its answer is the task's one approval, of its plan (S6c4). A
// task that already has an open approve person Job gets nothing new, so a second
// delivery posts no second one; the id is "" then. The Job carries no continues:
// it adds a person step and does not reopen an earlier approval.
func postShipPersonJob(env Env, dir string) (string, error) {
	fid, owner, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return "", err
	}
	store := feed.Store{TaskDir: dir, TaskID: fid, Warn: env.Stderr}
	for try := 0; try < 3; try++ {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return "", err
		}
		if openApprovePersonJob(snap) != "" {
			return "", nil
		}
		w, err := taskJobWriter(env)
		if err != nil {
			return "", err
		}
		id := nextJobID(snap)
		body := migrate.PlanJobBody(id, owner)
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
			if err == nil && (openApprovePersonJob(s) != "" || s.State.Jobs[id] != nil || slices.Contains(s.JobIDs, id)) {
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
