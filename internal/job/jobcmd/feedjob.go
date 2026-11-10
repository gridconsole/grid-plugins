package jobcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
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
	if !errors.Is(planErr, fs.ErrNotExist) && !errors.Is(planErr, job.ErrJobNotInPlan) {
		return job.Job{}, planErr
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
// id has no Feed this store can read, and answers as a missing one. It is a
// variable so a test can hand back a wrapped error.
var readFeed = func(task string, filter feed.Filter) (feed.ReadResult, error) {
	id, err := feed.TaskID(task)
	if err != nil {
		return feed.ReadResult{}, os.ErrNotExist
	}
	return feed.Store{TaskDir: task, TaskID: id}.ReadOnly(filter)
}

// feedJob returns the newest Job record posted for selector (a Job id or item
// number), as posted, or nil when the Feed holds none.
func feedJob(task, selector string) (*job.Job, error) {
	read, err := readFeed(task, feed.Filter{Type: "job"})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return pickJob(read.Records, selector)
}

// pickJob chooses the newest non-stale Job record for selector and marks every
// older record of that same Job stale, in records, so a reposted Job renders
// from its latest post. Only the chosen record is decoded: a record that is
// malformed fails a render that selects its Job and no other.
func pickJob(records []feed.Entry, selector string) (*job.Job, error) {
	id := ""
	for _, e := range records {
		if e.Type != "job" || e.Verdict == "stale" {
			continue
		}
		jobID, item := identity(e)
		if jobID == selector || (item != 0 && strconv.Itoa(item) == selector) {
			id = jobID
		}
	}
	if id == "" {
		return nil, nil
	}
	newest := -1
	for i, e := range records {
		if e.Type != "job" || e.Verdict == "stale" {
			continue
		}
		if jobID, _ := identity(e); jobID == id {
			newest = i
		}
	}
	for i := range records {
		e := &records[i]
		if i == newest || e.Type != "job" || e.Verdict == "stale" {
			continue
		}
		if jobID, _ := identity(*e); jobID == id {
			e.Verdict = "stale"
			e.Reason = "superseded by " + records[newest].ID
		}
	}
	return decodeJob(records[newest], id)
}

// identity names the Job a record is for from its id and item fields alone. A
// record without a usable id field is the Job its record names.
func identity(e feed.Entry) (id string, item int) {
	id, _ = e.Body["id"].(string)
	if id == "" && e.Job != nil {
		id = *e.Job
	}
	if n, ok := e.Body["item"].(float64); ok {
		item = int(n)
	}
	return id, item
}

// decodeJob reads one Job record. A record that does not decode is reported by
// its record id, the Job it names and the field that is wrong.
func decodeJob(e feed.Entry, id string) (*job.Job, error) {
	b, err := json.Marshal(e.Body)
	if err != nil {
		return nil, fmt.Errorf("feed Job %s record %s: %w", id, e.ID, err)
	}
	var j job.Job
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, fmt.Errorf("feed Job %s record %s: field %s: %s", id, e.ID, badField(e.Body, err), problem(err))
	}
	if j.ID == "" {
		j.ID = id
	}
	j.Defaults()
	return &j, nil
}

// badField names the field a decode failed on: the path json reports, or, when
// the error carries none, the first field that does not decode on its own.
func badField(body map[string]any, err error) string {
	var typed *json.UnmarshalTypeError
	if errors.As(err, &typed) && typed.Field != "" {
		return typed.Field
	}
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		one, merr := json.Marshal(map[string]any{k: body[k]})
		if merr != nil {
			return k
		}
		var j job.Job
		if json.Unmarshal(one, &j) != nil {
			return k
		}
	}
	return "(unknown)"
}

// problem says what is wrong with the field, without the Go struct path.
func problem(err error) string {
	var typed *json.UnmarshalTypeError
	if errors.As(err, &typed) {
		return fmt.Sprintf("want %s, got %s", typed.Type, typed.Value)
	}
	return err.Error()
}
