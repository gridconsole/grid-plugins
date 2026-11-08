package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// publishPlan runs under the task lease and pass lock. The Feed owns published
// Jobs; plan edits never reset their lifecycle. Missing Jobs are retried after a
// partial publication, including when a new task holder resumes the task.
func (s Store) publishPlan(snap Snapshot, m *memory) error {
	text, err := (plan.Store{TaskDir: s.Feed.TaskDir}).Read()
	if errors.Is(err, os.ErrNotExist) {
		return s.publicationBlocked(m, false)
	}
	if err != nil {
		return err
	}
	// Hold task before plan, matching approval/reopen writers. The exact plan
	// snapshot that passes the approval gate remains stable through publication.
	lock, err := fslock.OpenNoFollow(filepath.Join(s.Feed.TaskDir, ".task.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	return (plan.Store{TaskDir: s.Feed.TaskDir}).WithHash(func(_ string) error {
		text, err = (plan.Store{TaskDir: s.Feed.TaskDir}).Read()
		if err != nil {
			return err
		}
		workflow, err := os.ReadFile(filepath.Join(s.Feed.TaskDir, "task.md"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if taskfile.PlanApprovalRequired(string(workflow)) && !task.Approved(s.Feed.TaskDir) {
			return s.publicationBlocked(m, true)
		}
		if err := s.publicationBlocked(m, false); err != nil {
			return err
		}
		return s.publishPlanText(snap, text)
	})
}

// Persist transitions under the pass lock, including across CLI invocations.
// Save before continuing the tick so a later failure cannot repeat the notice.
func (s Store) publicationBlocked(m *memory, blocked bool) error {
	if m.PlanPublicationBlocked == blocked {
		return nil
	}
	m.PlanPublicationBlocked = blocked
	if err := s.saveMemory(*m); err != nil {
		return err
	}
	if blocked && s.Diagnostics != nil {
		fmt.Fprintln(s.Diagnostics, "plan publication skipped: workflow requires approval of the current plan; run grid approve before Jobs can post")
	}
	return nil
}

func (s Store) publishPlanText(snap Snapshot, text string) error {
	items, err := plan.Items(text)
	if err != nil {
		return err
	}
	pending := []feed.Input{}
	jobs := append([]job.Job{}, snap.Items...)
	seen := map[string]bool{}
	for _, item := range items {
		for _, raw := range item.Jobs {
			j, problems := job.DecodePlanned(raw, item.Kind)
			if seen[j.ID] {
				return fmt.Errorf("plan.md: duplicate Job %s", j.ID)
			}
			seen[j.ID] = true
			if snap.State.Jobs[j.ID] != nil {
				continue
			}
			if len(problems) > 0 {
				return fmt.Errorf("plan.md Job %s: %s", j.ID, strings.Join(problems, "; "))
			}
			j.Defaults()
			data, err := j.Record()
			if err != nil {
				return err
			}
			if err := contract.Validate("job-record", data); err != nil {
				return fmt.Errorf("plan.md Job %s: %w", j.ID, err)
			}
			floor, err := jobgraph.ReviewFloor(j, jobgraph.Policy{})
			if err != nil {
				return err
			}
			if len(floor.Errors) > 0 {
				return fmt.Errorf("plan.md Job %s: %s", j.ID, strings.Join(floor.Errors, "; "))
			}
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				return err
			}
			pending = append(pending, feed.Input{Type: "job", Job: j.ID, Body: body})
			jobs = append(jobs, j)
		}
	}
	// Validate the complete graph before publishing any part of it.
	graph, err := jobgraph.New(jobs)
	if err != nil {
		return err
	}
	if cycle := graph.Cycle(); len(cycle) > 0 {
		return &jobgraph.CycleError{Path: cycle}
	}
	for _, in := range pending {
		if _, err := s.Feed.Post(in, s.Writer); err != nil {
			return err
		}
	}
	return nil
}
