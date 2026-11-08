package lease

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// JobStatus is a read-only view of a Job grant and its supervision heartbeat.
// Missing grants or beats do not prove expiry. This reader never creates files,
// renews leases, or changes the Job's Feed status.
type JobStatus struct {
	Record
	Attempt int  `json:"attempt"`
	Known   bool `json:"-"`
}

// ReadJob reads lease/<id>.json under LOCK_SH, then the matching local .live
// record. The contract schemas validate both wire shapes; Record is only a view.
func ReadJob(taskDir, id string) (JobStatus, error) {
	if !job.ValidID(id) {
		return JobStatus{}, errors.New("invalid Job id")
	}
	dir := filepath.Join(taskDir, "lease")
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return JobStatus{}, nil
	}
	if err != nil {
		return JobStatus{}, err
	}
	if !info.IsDir() {
		return JobStatus{}, errors.New("lease directory must be a real directory")
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, id+".json"), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return JobStatus{}, nil
	}
	if err != nil {
		return JobStatus{}, err
	}
	defer f.Close()
	if err := fslock.LockShared(f); err != nil {
		return JobStatus{}, err
	}
	defer fslock.Unlock(f)
	// A writer can leave an old tail after rewriting in place; decode one value.
	var raw json.RawMessage
	if err := json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&raw); err != nil {
		return JobStatus{}, err
	}
	if err := contract.Validate("lease", raw); err != nil {
		return JobStatus{}, err
	}
	var status JobStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return status, err
	}
	if status.Lease != id {
		return JobStatus{}, errors.New("Job lease identity mismatch")
	}
	status.Known = true
	if status.Holder == nil {
		return status, nil
	}
	liveDir := filepath.Join(taskDir, ".live")
	info, err = os.Lstat(liveDir)
	if errors.Is(err, os.ErrNotExist) {
		return status, nil
	}
	if err != nil {
		return JobStatus{}, err
	}
	if !info.IsDir() {
		return JobStatus{}, errors.New("liveness directory must be a real directory")
	}
	live, err := fslock.OpenNoFollow(filepath.Join(liveDir, *status.Holder+"."+strconv.FormatUint(status.Fence, 10)+".json"), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return status, nil
	}
	if err != nil {
		return JobStatus{}, err
	}
	defer live.Close()
	raw, err = io.ReadAll(io.LimitReader(live, 64<<10))
	if err != nil {
		return JobStatus{}, err
	}
	if err := contract.Validate("heartbeat", raw); err != nil {
		return JobStatus{}, err
	}
	var beat JobStatus
	if err := json.Unmarshal(raw, &beat); err != nil {
		return JobStatus{}, err
	}
	if beat.Lease != id || beat.Holder == nil || *beat.Holder != *status.Holder || beat.Fence != status.Fence || beat.Attempt != status.Attempt {
		return JobStatus{}, fmt.Errorf("Job %s heartbeat does not match its lease", id)
	}
	status.Until = beat.Until
	return status, nil
}

// Expired reports release, a superseded fence, or a matching expired heartbeat.
// With no heartbeat there is no local evidence of time-based expiry.
func (s JobStatus) Expired(fence uint64, now time.Time) bool {
	return s.Known && (s.Holder == nil || (fence != 0 && fence != s.Fence) || (!s.Until.IsZero() && !now.Before(s.Until)))
}
