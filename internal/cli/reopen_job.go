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
	store := feed.Store{TaskDir: dir, TaskID: id, Warn: env.Stderr}
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
	// A Job's agent holds its own Job epoch in GRID_LEASE_EPOCH and the task epoch
	// in GRID_TASK_EPOCH; the plan Job is a task write, so it takes the task epoch
	// unless --lease-epoch was typed as a global.
	epoch := env.getenv("GRID_LEASE_EPOCH")
	if t := env.getenv("GRID_TASK_EPOCH"); t != "" && !env.typedGlobal["GRID_LEASE_EPOCH"] {
		epoch = t
	}
	w, err := orchestratorWriter(env, "task", epoch)
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
		// The id is picked now, not before the reopen: a Job posted since the first
		// read, or one the fold holds out of the snapshot, may already carry it. The
		// post is checked again under .orchestrator.lock, and a taken id is picked again.
		for {
			top, err := reopenJobTop(store, highest)
			if err != nil {
				return err
			}
			next.ID = "J-" + strconv.Itoa(top+1)
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
			skipped := false
			var readErr error
			_, err = orchestrator.WriteUnless(store, w, feed.Input{Type: "job", Job: next.ID, Body: body}, func() bool {
				taken, e := reopenJobTaken(store, next.ID)
				if e != nil {
					// Nothing is written on a failed read; the error is returned below.
					readErr = e
					return true
				}
				skipped = taken
				return taken
			})
			if err != nil {
				return err
			}
			if readErr != nil {
				return readErr
			}
			if !skipped {
				return nil
			}
		}
	}, nil
}

// reopenJobNumber is the n of a J-<n> id; a dotted child id counts by its J-<n> part.
// An id that is not of that form is 0.
func reopenJobNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// reopenJobTop is the highest J-<n> that floor or any `job` record of the Feed names,
// whatever the record's verdict: a held, ignored or bad record still holds its id.
func reopenJobTop(store feed.Store, floor int) (int, error) {
	r, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		return 0, err
	}
	top := floor
	for _, e := range r.Records {
		if e.Type != "job" {
			continue
		}
		id, _ := e.Body["id"].(string)
		if n := reopenJobNumber(id); n > top {
			top = n
		}
	}
	return top, nil
}

// reopenJobTaken reports whether a `job` record of the Feed already carries id.
func reopenJobTaken(store feed.Store, id string) (bool, error) {
	r, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		return false, err
	}
	for _, e := range r.Records {
		if e.Type != "job" {
			continue
		}
		if got, _ := e.Body["id"].(string); got == id {
			return true, nil
		}
	}
	return false, nil
}
