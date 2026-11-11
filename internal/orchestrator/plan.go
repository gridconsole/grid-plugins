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
		pending, taken, err := s.planJobInputs(snap, text)
		if err != nil {
			// A fault of the plan text posts no Job and leaves the rest of the
			// tick to run as on a task without plan.md.
			return s.planUnpublishable(snap, m, text, err)
		}
		if err := s.publicationBlocked(m, false); err != nil {
			return err
		}
		if err := s.postPlanJobs(pending); err != nil {
			return err
		}
		return s.postTakenNotices(snap, taken)
	})
}

// postTakenNotices tells the task once, on the Feed only, that a person already
// holds the id of a plan Job. The text is its own dedup marker and only a leased
// task message counts, so a person's post cannot silence it.
func (s Store) postTakenNotices(snap Snapshot, texts []string) error {
	for _, text := range texts {
		found := false
		for _, r := range snap.Records {
			if r.Type == "post" && r.Lease != nil && *r.Lease == "task" && r.Body["kind"] == "message" && stringValue(r.Body["ref"]) == "task" && stringValue(r.Body["text"]) == text {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if _, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, s.Writer); err != nil {
			return err
		}
	}
	return nil
}

// planJobTaken returns the notice text when the stored Job with this id was
// first posted by a person (no lease) and differs from the plan's Job in title
// or role. The text carries the id and the stamped author id only, no title.
func planJobTaken(snap Snapshot, j job.Job) (string, bool) {
	var first *feed.Record
	for _, r := range snap.Records {
		if r.Type != "job" || r.Job == nil || *r.Job != j.ID {
			continue
		}
		r := r
		if first == nil || r.At < first.At || (r.At == first.At && r.ID < first.ID) {
			first = &r
		}
	}
	if first == nil || first.Lease != nil {
		return "", false
	}
	if stringValue(first.Body["title"]) == j.Title && stringValue(first.Body["role"]) == j.Role {
		return "", false
	}
	return "plan.md's Job " + j.ID + " was not posted: " + first.Author.ID + " already posted a Job with that id.", true
}

// planUnpublishable marks publication blocked and tells the task once why. The
// notice text is its own durable deduplication marker, as in warnDuplicateGates.
// It then posts one re-plan Job to the orchestrator role with the cause, unless
// one is still open (postPlanFaultJob).
func (s Store) planUnpublishable(snap Snapshot, m *memory, planText string, cause error) error {
	if err := s.publicationBlocked(m, true); err != nil {
		return err
	}
	text := "plan.md cannot be published: " + oneLine(cause) + ". No Job from plan.md is posted until it is fixed."
	found := false
	for _, r := range snap.Records {
		if r.Type == "post" && r.Lease != nil && *r.Lease == "task" && r.Body["kind"] == "message" && stringValue(r.Body["ref"]) == "task" && stringValue(r.Body["text"]) == text {
			found = true
			break
		}
	}
	if !found {
		if _, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, s.Writer); err != nil {
			return err
		}
	}
	return s.postPlanFaultJob(snap, planText, oneLine(cause))
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
// The second result holds the notice texts for plan Jobs whose id a person
// already holds.
func (s Store) planJobInputs(snap Snapshot, text string) ([]feed.Input, []string, error) {
	items, err := plan.Items(text)
	if err != nil {
		return nil, nil, err
	}
	pending := []feed.Input{}
	taken := []string{}
	jobs := append([]job.Job{}, snap.Items...)
	seen := map[string]bool{}
	for _, item := range items {
		for _, raw := range item.Jobs {
			j, problems := job.DecodePlanned(raw, item.Kind)
			if seen[j.ID] {
				return nil, nil, fmt.Errorf("plan.md: duplicate Job %s", j.ID)
			}
			seen[j.ID] = true
			if snap.State.Jobs[j.ID] != nil {
				if text, ok := planJobTaken(snap, j); ok {
					taken = append(taken, text)
				}
				continue
			}
			if len(problems) > 0 {
				return nil, nil, fmt.Errorf("plan.md Job %s: %s", j.ID, strings.Join(problems, "; "))
			}
			j.Defaults()
			data, err := j.Record()
			if err != nil {
				return nil, nil, err
			}
			if err := contract.Validate("job-record", data); err != nil {
				return nil, nil, fmt.Errorf("plan.md Job %s: %w", j.ID, err)
			}
			if err := jobgraph.CheckPlanJob(j); err != nil {
				return nil, nil, fmt.Errorf("plan.md Job %s: %w", j.ID, err)
			}
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				return nil, nil, err
			}
			pending = append(pending, feed.Input{Type: "job", Job: j.ID, Body: body})
			jobs = append(jobs, j)
		}
	}
	// Validate the complete graph before publishing any part of it.
	graph, err := jobgraph.New(jobs)
	if err != nil {
		return nil, nil, err
	}
	if cycle := graph.Cycle(); len(cycle) > 0 {
		return nil, nil, &jobgraph.CycleError{Path: cycle}
	}
	return pending, taken, nil
}

func (s Store) postPlanJobs(pending []feed.Input) error {
	for _, in := range pending {
		if _, err := s.Feed.Post(in, s.Writer); err != nil {
			return err
		}
	}
	return nil
}
