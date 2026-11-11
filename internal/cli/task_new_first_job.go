package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// firstJobID is the id of the Job a new task starts with.
const firstJobID = "J-1"

// firstJob is the record of the orchestrator Job that every new task gets, so the
// worker has something to start: the orchestrator reads intent.md#request, decides
// the Jobs and posts them. The owner wrote the request, so the Job's intent points
// at it; the Job's role carries agents/orchestrator.md into its rendered prompt.
func firstJob() map[string]any {
	request := "intent.md#request"
	return map[string]any{
		"id": firstJobID, "item": 1,
		"title":  "Read the task and post the Jobs it needs",
		"role":   "orchestrator",
		"skills": []string{"plan-jobs"},
		"review": "L0", "risk": []string{}, "max_rounds": 1,
		"intent": map[string]any{"owner": request, "item": request, "goal": request},
		"links":  []any{}, "keep": []string{}, "decisions": []string{},
		"read": []string{request}, "write": []string{"plan.md"}, "out_of_scope": []string{},
		"acceptance": []map[string]any{{"id": "A1", "text": "The Jobs the task needs are posted with `grid job post`, each with its `after` list, plan.md is stored with `grid plan write`, and a result is posted with `grid job result`.", "traces": "owner"}},
		"verify":     map[string]any{"kind": "file-exists", "path": "plan.md"},
		"budget":     map[string]any{"turns_estimate": 10, "reads_before_first_edit": 3},
		"on_blocked": "ask-and-continue",
		"after":      []any{}, "attempt": 1, "continues": nil, "discovered_from": nil, "close_reason": nil,
		"checkpoint": nil, "review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []string{"result", "question"},
	}
}

// postFirstJob posts J-1 on the Feed of a task that was just made. The task has no
// lease holder, so the owner takes the task lease for this one post and gives it
// back: the worker's stream then finds an open Job and a free lease, takes the
// lease itself and starts the Job. A task made with a workflow gets the same J-1:
// the request is in intent.md for every task, and no workflow code posts a Job.
func postFirstJob(env Env, t task.Task, o task.NewOptions) error {
	machine := env.Machine
	if home := resolver(env).GridHome; home != "" {
		if m := gridhome.Machine(home); m != "" {
			machine = m
		}
	}
	if machine == "" {
		return fmt.Errorf("machine id is missing; initialize the Grid home")
	}
	id, _, err := task.FeedIdentity(t.Path)
	if err != nil {
		return err
	}
	worker := t.Owner + "@" + machine
	if !contract.ValidWorker(worker) {
		// A real Grid home always has a ULID machine id; with another value the Feed takes no writer, so say so and keep the task.
		fmt.Fprintf(env.Stderr, "grid task new: %s has no first Job: %q is not a valid <member>@<machine ULID> writer id\n", t.ID, worker)
		return nil
	}
	data, err := json.Marshal(firstJob())
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
	store := feed.Store{TaskDir: t.Path, TaskID: id}
	w := feed.Writer{Author: feed.Author{ID: t.Owner, Role: t.OwnerRole}, Worker: worker, Lease: "task"}
	pid, began := os.Getpid(), time.Time{}
	if env.OwnerPID != 0 {
		pid, began = env.OwnerPID, env.OwnerStart
	}
	// The lease names the process by its start time, in the contract's time shape.
	if began.IsZero() {
		var err error
		began, err = time.Parse(time.RFC3339Nano, migrate.ProcessStart(pid))
		if err != nil {
			began = time.Now()
		}
	}
	start := contract.FormatTime(began)
	ts := lease.TaskStore{Feed: store}
	grant, err := ts.TakeTask(w, pid, start)
	if err != nil {
		return err
	}
	w.Gen, w.Fence = grant.Gen, grant.Fence
	defer ts.ReleaseTask(w)
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		return err
	}
	_, err = orchestrator.Write(store, w, feed.Input{Type: "job", Job: firstJobID, Body: body})
	return err
}
