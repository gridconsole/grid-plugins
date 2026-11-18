package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// curatorRole is the author role of the Job that keeps a task's brain proposals;
// the keeper's steps are skills/brain-keeper/SKILL.md ("Keep").
const curatorRole = "curator"

// openCuratorJob returns the id of a curator Job that is not yet closed, or "".
// A Job the fold ignored (a duplicate id) has no state and does not count.
func openCuratorJob(snap orchestrator.Snapshot) string {
	for _, it := range snap.Items {
		if !strings.EqualFold(strings.TrimSpace(it.Role), curatorRole) {
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

// nextJobID is the next free top-level J-<n>, counting the Jobs of the fold and
// the id of every job record the Feed holds, even one the fold holds out of State
// (snap.JobIDs). It is the rule of gkNextJobID in internal/orchestrator.
func nextJobID(snap orchestrator.Snapshot) string {
	highest := 0
	count := func(id string) {
		if n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-")); err == nil && n > highest {
			highest = n
		}
	}
	for _, it := range snap.Items {
		count(it.ID)
	}
	for id := range snap.State.Jobs {
		count(id)
	}
	for _, id := range snap.JobIDs {
		count(id)
	}
	return "J-" + strconv.Itoa(highest+1)
}

// curatorJobBody is the job-record of the curator Job. It names the proposals by
// pointer and copies none of their text. The Job writes only the brain, so it has
// no test command to count and, at L0, no reviewer Job to name: verify is the one
// remaining kind, file-exists, on the task file the keeper reads. It asks no
// person (on_blocked exit): the keeper accepts, merges or rejects with a reason.
// Each proposal is read whole (the lint lets a brain-proposals/<name>.md read go
// unranged), and the one acceptance row names the title's words, which are the
// plan the lint checks a curator Job's acceptance against.
func curatorJobBody(id, taskID string, ids []string) map[string]any {
	reads := make([]any, 0, len(ids))
	var elsewhere []string
	for _, p := range ids {
		// A project or home proposal lives under the Grid home, not in the task folder.
		if strings.HasPrefix(p, "brain:") {
			elsewhere = append(elsewhere, p)
			continue
		}
		reads = append(reads, "brain-proposals/"+p+".md")
	}
	where := ""
	if len(elsewhere) > 0 {
		where = fmt.Sprintf("; the project and home proposals (%s) are read with `grid brain keep --task %s` and `grid brain show`", strings.Join(elsewhere, ", "), taskID)
	}
	noun := "proposals"
	if len(ids) == 1 {
		noun = "proposal"
	}
	pointer := "task.md#outcome"
	return map[string]any{
		"id": id, "item": 1, "role": curatorRole,
		"title":  fmt.Sprintf("Curate %d brain %s of %s", len(ids), noun, taskID),
		"skills": []any{"brain-keeper"}, "risk": []any{},
		"intent": map[string]any{"owner": pointer, "item": pointer, "goal": pointer},
		"links":  []any{}, "keep": []any{}, "decisions": []any{},
		"read": reads, "write": []any{}, "out_of_scope": []any{},
		"acceptance": []any{map[string]any{
			"id":     "A1",
			"text":   fmt.Sprintf("Curate the %d brain %s of %s: `grid brain keep --task %s` lists no waiting proposal; each one is accepted, merged or rejected with a reason%s", len(ids), noun, taskID, taskID, where),
			"traces": "item",
		}},
		"verify":     map[string]any{"kind": "file-exists", "path": "task.md"},
		"budget":     map[string]any{"turns_estimate": 12, "reads_before_first_edit": 1},
		"on_blocked": "exit", "after": []any{},
		"attempt": 1, "continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": nil, "posts_back": []any{"result"},
	}
}

// postCuratorJob posts the curator Job for the proposals that wait and returns
// its id. It returns "" and no error when there is nothing to post: no proposal
// waits, or a curator Job of the task is still open. The caller holds no task or
// plan lock: the post takes .orchestrator.lock, and the check for an open curator
// Job is made again under it, so two deliveries cannot both post one. If the id
// is taken in between by another writer the id is chosen again.
func postCuratorJob(env Env, dir, taskID string, ids []string) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return "", err
	}
	store := feed.Store{TaskDir: dir, TaskID: fid, Warn: env.Stderr}
	for try := 0; try < 3; try++ {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return "", err
		}
		if openCuratorJob(snap) != "" {
			return "", nil
		}
		// The curator Job is a task write, so it takes the task epoch (taskJobWriter).
		w, err := taskJobWriter(env)
		if err != nil {
			return "", err
		}
		id := nextJobID(snap)
		body := curatorJobBody(id, taskID, ids)
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
			// A held record (its after target is on no record) is outside State but
			// has taken the id: the fold gives it to the first record once released,
			// so the id is chosen again, as nextJobID counts it.
			if err == nil && (openCuratorJob(s) != "" || s.State.Jobs[id] != nil || slices.Contains(s.JobIDs, id)) {
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

// curatorJobOpen reports whether a curator Job of the task is open, for the dry
// run: it reads the Feed and posts nothing.
func curatorJobOpen(env Env, dir string) (bool, error) {
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return false, err
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: fid, Warn: env.Stderr})
	if err != nil {
		return false, err
	}
	return openCuratorJob(snap) != "", nil
}

// deliverOutput is the --json result: the delivery, the curator Job it posted and
// the person Job of the ship step it posted ("" when it posted none).
type deliverOutput struct {
	taskstore.Delivery
	CuratorJob string `json:"curator_job"`
	PersonJob  string `json:"person_job,omitempty"`
}

// proposalPointer is what the curator Job names a waiting proposal by: its id for
// a task proposal (read from the task folder) and the keep path, brain:<level>/...,
// for a project or home one.
func proposalPointer(w brain.WaitingProposal) string {
	if strings.HasPrefix(w.Path, "brain:") {
		return w.Path
	}
	return w.ID
}
