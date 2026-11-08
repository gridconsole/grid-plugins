package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// publishPlan runs under the task lease and pass lock. The Feed owns published
// Jobs; plan edits never reset their lifecycle. Missing Jobs are retried after a
// partial publication, including when a new task holder resumes the task.
func (s Store) publishPlan(snap Snapshot) error {
	text, err := (plan.Store{TaskDir: s.Feed.TaskDir}).Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
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
