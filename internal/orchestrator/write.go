package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

var ErrDuplicateWake = errors.New("wake write already exists")

// Write serializes command-side checks with assignments, then delegates the
// actual append, fence check and wake deduplication to Feed. The returned record
// on ErrDuplicateWake is the first write, which the CLI prints before refusing.
func Write(store feed.Store, w feed.Writer, in feed.Input) (record feed.Record, err error) {
	if w.Lease == "" {
		return record, feed.ErrStaleLease
	}
	var attempt *int
	if w.Lease != "task" {
		attempt = &in.Attempt
	}
	// Check the lease before creating even the pass lock.
	err = store.WithLease(w, attempt, func() error { return nil })
	if err != nil {
		return record, err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(store.Dir(), ".orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return record, err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return record, err
	}
	defer fslock.Unlock(f)
	snap, err := Load(store)
	if err != nil {
		return record, err
	}
	if in.Type == "control" {
		target := stringValue(in.Body["target"])
		if strings.HasPrefix(target, "J-") && snap.State.Jobs[target] == nil {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("unknown Job %s", target)}
		}
	}
	if w.Lease != "task" {
		j := snap.State.Jobs[in.Job]
		if j == nil || j.Assign == nil || j.Attempt != in.Attempt || j.Assign.Fence != w.Fence || j.Assign.To != w.Worker || j.Generation != w.Gen || j.Status == contract.Cancelled || j.Status == contract.Rejected || j.Status == contract.Accepted {
			return record, feed.ErrStaleLease
		}
		if in.Type == "ack" {
			in.After = []string{j.AssignPost}
		} else if j.Status != contract.Running {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("Job tenure must start with ack")}
		}
		if j.Exited != "" {
			return record, &feed.InvalidRecordError{Err: fmt.Errorf("attempt already exited")}
		}
	}
	if in.ID == "" {
		in.ID, err = feed.NewID()
		if err != nil {
			return record, err
		}
	}
	record, err = store.Post(in, w)
	if err == nil && in.Wake != "" && record.ID != in.ID {
		err = ErrDuplicateWake
	}
	return
}

type Attempt struct {
	Number int    `json:"attempt"`
	Fence  int64  `json:"fence"`
	State  string `json:"state"`
	Exit   string `json:"exit,omitempty"`
}
type TreeJob struct {
	*contract.Job
	Attempts []Attempt `json:"attempts"`
}

func Tree(s Snapshot) []TreeJob {
	out := []TreeJob{}
	for _, id := range ids(s) {
		j := s.State.Jobs[id]
		row := TreeJob{Job: j, Attempts: []Attempt{}}
		for _, event := range j.History {
			r := s.Records[event]
			if a, ok := r.Body["assign"].(map[string]any); ok {
				row.Attempts = append(row.Attempts, Attempt{Number: int(a["attempt"].(float64)), Fence: int64(a["fence"].(float64)), State: "assigned"})
			} else if len(row.Attempts) > 0 {
				a := &row.Attempts[len(row.Attempts)-1]
				switch r.Type {
				case "ack":
					a.State = "running"
				case "worker-exit":
					a.Exit = stringValue(r.Body["reason"])
					if a.Exit == "" {
						a.Exit = "crash"
					}
					a.State = "exited"
				case "control":
					if status := stringValue(r.Body["status"]); status != "" {
						a.State = status
					}
				}
			}
		}
		out = append(out, row)
	}
	return out
}
