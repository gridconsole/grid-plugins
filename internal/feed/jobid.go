package feed

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// ErrJobExists marks a Job post whose Job id the Feed already holds.
var ErrJobExists = errors.New("job exists")

// JobExistsError refuses a second Job record for one Job id: the fold keeps the
// first one and would ignore this one. It is also an invalid record (exit 3).
type JobExistsError struct{ Job, Post string }

func (e *JobExistsError) Error() string {
	return fmt.Sprintf("Job %s already exists (post %s); revisions need a new Job", e.Job, e.Post)
}

func (e *JobExistsError) Is(target error) bool {
	return target == ErrJobExists || target == ErrInvalidRecord
}

func jobID(r Record) string {
	if r.Type != "job" {
		return ""
	}
	return str(r.Body["id"])
}

// jobLock is held by Post across the Job id check and the append. Lock order in
// Post: wake.lock, the lease file (shared), then gate.lock or job.lock (never
// both: a record is a gate question or a Job), then the tenure file.
// .orchestrator.lock is taken only by callers, outside Post. job.lock is taken
// nowhere else and Read takes no file locks, so there is no cycle.
func (s Store) jobLock() (*os.File, error) {
	dir := filepath.Join(s.Dir(), ".cache")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "job.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// checkJobFree refuses r when a counted or held Job record already holds its Job
// id, unless it is the same request (same record id and body) retried. A held
// record (its after or re target has not arrived, for example a dependency still
// syncing from another machine) counts: it becomes the record the fold keeps once
// its target lands, so a second writer must not post the same Job id beside it.
// Read puts a spilled body back, so the comparison sees the whole stored record;
// a record held for its spilled body has no Job id to read yet and cannot match.
func (s Store) checkJobFree(r Record, id string) error {
	existing, err := s.ReadOnly(Filter{Type: "job"})
	if err != nil {
		return err
	}
	for _, entry := range existing.Records {
		if (entry.Verdict != "counted" && entry.Verdict != "held") || jobID(entry.Record) != id {
			continue
		}
		if entry.ID == r.ID && sameRequest(entry.Record, r) {
			continue
		}
		return &JobExistsError{Job: id, Post: entry.ID}
	}
	return nil
}
