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
)

// publishPlan runs under the task lease and pass lock. The Feed owns published
// Jobs; plan edits never reset their lifecycle. Missing Jobs are retried after a
// partial publication, including when a new task holder resumes the task.
func (s Store) publishPlan(snap Snapshot, m *memory) error {
	return s.publishPlanWithReader(snap, m, (plan.Store{TaskDir: s.Feed.TaskDir}).Read)
}

// The reader seam lets tests replace plan.md after the publication snapshot is read.
func (s Store) publishPlanWithReader(snap Snapshot, m *memory, read func() (string, error)) error {
	text, err := read()
	if errors.Is(err, os.ErrNotExist) {
		return s.publicationBlocked(m, false)
	}
	if err != nil {
		return err
	}
	// Hold task before plan, matching approval/reopen writers. The exact plan
	// snapshot is checked directly because writers can ignore the advisory locks.
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
		text, err = read()
		if err != nil {
			return err
		}
		pending, err := s.planJobInputs(snap, text)
		if err != nil {
			// A fault of the plan text posts no Job and leaves the rest of the
			// tick to run as on a task without plan.md.
			return s.planUnpublishable(snap, m, err)
		}
		if err := s.publicationBlocked(m, false); err != nil {
			return err
		}
		return s.postPlanJobs(pending)
	})
}

// planUnpublishable marks publication blocked and tells the task once why. The
// notice text is its own durable deduplication marker, as in warnDuplicateGates.
func (s Store) planUnpublishable(snap Snapshot, m *memory, cause error) error {
	if err := s.publicationBlocked(m, true); err != nil {
		return err
	}
	text := "plan.md cannot be published: " + oneLine(cause) + ". No Job from plan.md is posted until it is fixed."
	for _, r := range snap.Records {
		if r.Type == "post" && r.Lease != nil && *r.Lease == "task" && r.Body["kind"] == "message" && stringValue(r.Body["ref"]) == "task" && stringValue(r.Body["text"]) == text {
			return nil
		}
	}
	_, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, s.Writer)
	return err
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
	return nil
}

// planJobInputs checks the plan text and the whole graph and returns the Job
// records still to post. Any error is a fault of the plan text.
func (s Store) planJobInputs(snap Snapshot, text string) ([]feed.Input, error) {
	items, err := plan.Items(text)
	if err != nil {
		return nil, err
	}
	pending := []feed.Input{}
	jobs := append([]job.Job{}, snap.Items...)
	seen := map[string]bool{}
	for _, item := range items {
		for _, raw := range item.Jobs {
			j, problems := job.DecodePlanned(raw, item.Kind)
			if seen[j.ID] {
				return nil, fmt.Errorf("plan.md: duplicate Job %s", j.ID)
			}
			seen[j.ID] = true
			if snap.State.Jobs[j.ID] != nil {
				continue
			}
			if len(problems) > 0 {
				return nil, fmt.Errorf("plan.md Job %s: %s", j.ID, strings.Join(problems, "; "))
			}
			j.Defaults()
			data, err := j.Record()
			if err != nil {
				return nil, err
			}
			if err := contract.Validate("job-record", data); err != nil {
				return nil, fmt.Errorf("plan.md Job %s: %w", j.ID, err)
			}
			if err := jobgraph.CheckPlanJob(j); err != nil {
				return nil, fmt.Errorf("plan.md Job %s: %w", j.ID, err)
			}
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				return nil, err
			}
			pending = append(pending, feed.Input{Type: "job", Job: j.ID, Body: body})
			jobs = append(jobs, j)
		}
	}
	// Validate the complete graph before publishing any part of it.
	graph, err := jobgraph.New(jobs)
	if err != nil {
		return nil, err
	}
	if cycle := graph.Cycle(); len(cycle) > 0 {
		return nil, &jobgraph.CycleError{Path: cycle}
	}
	return pending, nil
}

func (s Store) postPlanJobs(pending []feed.Input) error {
	for _, in := range pending {
		if _, err := s.Feed.Post(in, s.Writer); err != nil {
			return err
		}
	}
	return nil
}
