package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
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

// shipAcceptance is the acceptance text of the ship person Job. It names what the
// answer lets happen next, landing, and is how landAfterShip tells the ship Job
// from the plan Job, which has the same kind and item.
const shipAcceptance = "The delivered work is approved to land"

// shipTitle is the title of the ship person Job, which the app shows as its need.
const shipTitle = "Approve the delivered work"

// shipJobBody is the plan approve Job (migrate.PlanJobBody) with the title and the
// acceptance text of the ship step; everything else, kind and item included, is
// the plan Job's.
func shipJobBody(id, owner string) map[string]any {
	body := migrate.PlanJobBody(id, owner)
	body["title"] = shipTitle
	body["acceptance"] = []any{map[string]any{"id": "A1", "text": shipAcceptance, "traces": "item"}}
	return body
}

// isShipJob reports whether it is the ship person Job a delivery posts.
func isShipJob(it job.Job) bool {
	return it.IsPerson() && it.Kind == "approve" && len(it.Acceptance) == 1 && it.Acceptance[0].Text == shipAcceptance
}

// isShipJobID reports whether id is a ship person Job of the Feed.
func isShipJobID(snap orchestrator.Snapshot, id string) bool {
	for _, it := range snap.Items {
		if it.ID == id {
			return isShipJob(it)
		}
	}
	return false
}

// shipWait is why the ship person Job ship may not be answered or land yet: a
// deliver Job of the task is not accepted, or was rejected and its rework is not
// yet accepted (jobgraph.DeliveryWait). The ship Job stays bound to the task's
// delivery, not to the run that posted it: a rework delivers again into the same
// task.md and finds the ship Job open, so the person's approve lands the rework's
// delivery once the rework is accepted, and never a rejected one (round 14 F1).
func shipWait(snap orchestrator.Snapshot, ship string) error {
	jobs := map[string]contract.Job{}
	for id, j := range snap.State.Jobs {
		if j != nil {
			jobs[id] = *j
		}
	}
	if w, ok := jobgraph.DeliveryWait(snap.Items, jobgraph.Snapshot{Jobs: jobs}); !ok {
		return &orchestrator.WaitingError{Job: ship, Target: w.Target, Reason: w.Reason}
	}
	return nil
}

// shipReviewing reports whether err, the answer of shipWait, is only a delivery
// that was submitted and whose review has not decided it. That is the delivery the
// person is reading, so the answer is recorded and the land waits for the review
// (landAfterShipAs) rather than the answer being refused. A rejected delivery, or
// one still being made, is not that delivery: its answer stays refused.
func shipReviewing(snap orchestrator.Snapshot, err error) bool {
	var waiting *orchestrator.WaitingError
	if !errors.As(err, &waiting) {
		return false
	}
	j := snap.State.Jobs[waiting.Target]
	return j != nil && j.Status == contract.Submitted
}

// renderShipJob saves the brief of the ship person Job in jobs/<id>/brief.md, so
// grid job show reads it as it reads every Job a worker starts (round 14 F6). A
// person Job is never started by a worker, and grid job render refuses its body
// (intent.owner names the person, not a pointer), so the brief is the Job record's
// metadata with the brief's sections, which is what grid job show decodes. A
// brief that cannot be saved is said on stderr and never fails the delivery.
func renderShipJob(env Env, dir, id string) {
	if err := saveShipBrief(env, dir, id); err != nil {
		fmt.Fprintf(env.Stderr, "deliver: the brief of %s was not saved: %s\n", id, oneLineErr(err))
	}
}

func saveShipBrief(env Env, dir, id string) error {
	fid, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return err
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: fid, Warn: env.Stderr})
	if err != nil {
		return err
	}
	i := slices.IndexFunc(snap.Items, func(it job.Job) bool { return it.ID == id })
	if i < 0 {
		return fmt.Errorf("%s is not on the Feed", id)
	}
	meta, err := job.Metadata(snap.Items[i])
	if err != nil {
		return err
	}
	// Each folder is checked inside the task before the next is made in it, as
	// grid job render saves a brief.
	for _, rel := range []string{"jobs", filepath.Join("jobs", id)} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			return err
		}
		if _, err := job.Within(dir, rel); err != nil {
			return err
		}
	}
	// A brief already there (a symlink included) is left as it is.
	path := filepath.Join(dir, "jobs", id, "brief.md")
	if _, err := os.Lstat(path); err == nil {
		return nil
	}
	brief := meta + "\n## Intent\nThe person approves the delivered work (task.md, Outcome) to land.\n## Job\nAnswer with grid approve or in the Grid app.\n## Context\n## Out of scope\n## Open\n## Acceptance\n- A1 " + shipAcceptance + "\n## Outcome\n## Verdict\n## Follow-up\n"
	// Atomic, as every other task-folder writer is: the next delivery keeps a brief
	// that exists, so a part-written one would stay for good.
	return atomicfile.Write(path, []byte(brief), 0o644)
}

// postShipPersonJob posts the person step that follows Check: the plan approve
// Job (shipJobBody) for the person of the delivered task in dir, the
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
		body := shipJobBody(id, owner)
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
