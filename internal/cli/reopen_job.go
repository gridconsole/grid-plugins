package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// reopenPlanJob prepares the plan Job that a reopen posts (Q8). A task whose plan
// Job was answered needs a person to answer again, so the reopen posts a new
// person Job of the approve step that continues the old one; the old result stays
// in the Feed as history and no longer approves the plan (planJobResult). It
// returns nil when there is nothing to post: the task has no plan Job, or its
// plan Job is still open. The writer is built before the task is reopened, so a
// reopen never leaves the old result approving the plan. The returned func runs
// after the reopen, holding no task or plan lock: the post takes .orchestrator.lock.
func reopenPlanJob(env Env, dir string) (func() error, error) {
	id, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return nil, nil
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	snap, err := orchestrator.Load(store)
	if err != nil {
		return nil, nil
	}
	last := -1
	highest := 0
	for i, it := range snap.Items {
		if n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(it.ID, ".", 2)[0], "J-")); err == nil && n > highest {
			highest = n
		}
		j := snap.State.Jobs[it.ID]
		if !it.IsPerson() || it.Kind != "approve" || j == nil || j.Status == contract.Cancelled || j.Status == contract.Rejected {
			continue
		}
		if j.Status != contract.Accepted {
			return nil, nil
		}
		last = i
	}
	if last < 0 {
		return nil, nil
	}
	w, err := orchestratorWriter(env, "task", env.getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		return nil, fmt.Errorf("the plan Job cannot be posted for the reopened task: %w", err)
	}
	next := snap.Items[last]
	previous := next.ID
	next.ID = "J-" + strconv.Itoa(highest+1)
	next.Title = "Approve the plan again after the reopen"
	next.Continues = &previous
	next.Order, next.Attempt, next.Status = nil, 1, ""
	// A person answers this Job; no worktree is used. The copied one would not
	// encode back to the Feed's string form either.
	next.Worktree = nil
	return func() error {
		next.Defaults()
		data, err := next.Record()
		if err != nil {
			return err
		}
		if err := contract.Validate("job-record", data); err != nil {
			return err
		}
		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			return err
		}
		_, err = orchestrator.Write(store, w, feed.Input{Type: "job", Job: next.ID, Body: body})
		return err
	}, nil
}
