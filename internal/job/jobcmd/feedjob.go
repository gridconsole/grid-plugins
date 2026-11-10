package jobcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// loadJob reads the Job from plan.md when plan.md names it, and otherwise from
// what the task's Feed holds for it. A task with no plan.md renders the Jobs
// that were posted on its Feed.
func loadJob(task, selector string) (job.Job, error) {
	j, planErr := job.LoadPlan(task, selector)
	if planErr == nil {
		return j, nil
	}
	posted, err := feedJob(task, selector)
	if err != nil {
		return job.Job{}, errors.Join(planErr, err)
	}
	if posted == nil {
		return job.Job{}, planErr
	}
	return *posted, nil
}

// readFeed reads the task's Feed without writing to it. A task with no stable
// id has no Feed this store can read, and answers as a missing one.
func readFeed(task string, filter feed.Filter) (feed.ReadResult, error) {
	id, err := feed.TaskID(task)
	if err != nil {
		return feed.ReadResult{}, os.ErrNotExist
	}
	return feed.Store{TaskDir: task, TaskID: id}.ReadOnly(filter)
}

// feedJob returns the first Job record posted for selector (a Job id or item
// number), as posted, or nil when the Feed holds none.
func feedJob(task, selector string) (*job.Job, error) {
	read, err := readFeed(task, feed.Filter{Type: "job"})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range read.Records {
		if e.Type != "job" || e.Verdict == "stale" {
			continue
		}
		b, err := json.Marshal(e.Body)
		if err != nil {
			return nil, err
		}
		var j job.Job
		if err := json.Unmarshal(b, &j); err != nil {
			return nil, fmt.Errorf("feed Job %s: %w", selector, err)
		}
		if j.ID == "" && e.Job != nil {
			j.ID = *e.Job
		}
		if j.ID != selector && (j.Item == 0 || strconv.Itoa(j.Item) != selector) {
			continue
		}
		j.Defaults()
		return &j, nil
	}
	return nil, nil
}
