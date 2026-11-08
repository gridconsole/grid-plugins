package lease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// Tenure is the lease-file schema; the shared fencing fields are owned by Feed.
// Validate at the persistence boundary keeps this representation on the contract.
type Tenure struct {
	feed.Lease
	Since    string  `json:"since"`
	PID      *int    `json:"pid"`
	PIDStart *string `json:"pid_start"`
}

type TaskStore struct {
	Feed feed.Store
	Now  func() time.Time
}

func (s TaskStore) now() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return contract.FormatTime(now)
}
func (s TaskStore) open(name string, create bool) (*os.File, error) {
	if name != "task" && !job.ValidID(name) {
		return nil, errors.New("invalid lease name")
	}
	dir := filepath.Join(s.Feed.Dir(), "lease")
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("lease must be a real directory")
	}
	flags := os.O_RDONLY
	if create {
		flags = os.O_RDWR | os.O_CREATE
	}
	return fslock.OpenNoFollow(filepath.Join(dir, name+".json"), flags, 0o600)
}
func readTenure(f *os.File) (Tenure, error) {
	var t Tenure
	b, err := io.ReadAll(f)
	if err != nil || len(b) == 0 {
		return t, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return t, err
	}
	if err := contract.Validate("lease", raw); err != nil {
		return t, err
	}
	err = json.Unmarshal(raw, &t)
	return t, err
}
func (s TaskStore) Read(name string) (Tenure, error) {
	f, err := s.open(name, false)
	if err != nil {
		return Tenure{}, err
	}
	defer f.Close()
	if err := fslock.LockShared(f); err != nil {
		return Tenure{}, err
	}
	defer fslock.Unlock(f)
	t, err := readTenure(f)
	if err == nil && t.Lease.Lease != name {
		err = errors.New("lease identity mismatch")
	}
	return t, err
}

// TaskTake authorizes replacement of a specific tenure. OfflineSince is a
// server observation supplied by the supervising caller, never a file timestamp.
type TaskTake struct {
	Adopt, Takeover bool
	Gen, Fence      int64
	OfflineSince    time.Time
}

// TakeTask grants the first tenure or checks replacement evidence while holding
// task.json exclusively, so two adopters cannot both replace the same holder.
func (s TaskStore) TakeTask(w feed.Writer, pid int, start string, options ...TaskTake) (Tenure, error) {
	policy := TaskTake{}
	if len(options) > 0 {
		policy = options[0]
	}
	return s.grant(Tenure{Lease: feed.Lease{Lease: "task", Holder: &w.Worker, Gen: w.Gen}, Since: s.now(), PID: &pid, PIDStart: &start}, func(old Tenure) error {
		if old.Holder == nil {
			return nil
		}
		if policy.Adopt == policy.Takeover || policy.Gen != old.Gen || policy.Fence != old.Fence {
			return errors.New("task lease held; adoption or takeover requires its current expected epoch")
		}
		if policy.Adopt {
			host := w.Worker[strings.LastIndexByte(w.Worker, '@')+1:]
			if !strings.HasSuffix(*old.Holder, "@"+host) || old.PID == nil || old.PIDStart == nil || session.ProbeLocal(session.Live{PID: *old.PID, PIDStart: *old.PIDStart}) != session.Dead {
				return errors.New("adoption requires a provably gone process on this host")
			}
			return nil
		}
		now, _ := time.Parse(time.RFC3339Nano, s.now())
		if contract.TierFor(w.Author.Role) != contract.PersonTeam || policy.OfflineSince.IsZero() || now.Sub(policy.OfflineSince) < 15*time.Minute {
			return errors.New("takeover requires a person and 15 minutes of server-observed offline time")
		}
		return nil
	})
}

// GrantJob requires the current task holder throughout the grant and snapshot.
func (s TaskStore) GrantJob(task feed.Writer, id, to string, attempt int) (t Tenure, err error) {
	err = s.Feed.WithLease(task, nil, func() error {
		var e error
		t, e = s.grant(Tenure{Lease: feed.Lease{Lease: id, Holder: &to, Gen: task.Gen, Attempt: &attempt}, Since: s.now()}, nil)
		return e
	})
	return
}
func (s TaskStore) grant(next Tenure, authorize func(Tenure) error) (Tenure, error) {
	// Validate before creating anything or ending the old tenure.
	next.Fence = 1
	b, err := json.Marshal(next)
	if err == nil {
		err = contract.Validate("lease", b)
	}
	if err != nil {
		return Tenure{}, err
	}
	f, err := s.open(next.Lease.Lease, true)
	if err != nil {
		return Tenure{}, err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return Tenure{}, err
	}
	defer fslock.Unlock(f)
	old, err := readTenure(f)
	if err != nil {
		return Tenure{}, err
	}
	if authorize != nil {
		if err := authorize(old); err != nil {
			return Tenure{}, err
		}
	}
	if old.Holder != nil {
		if next.Gen < old.Gen || next.Attempt != nil && old.Attempt != nil && *next.Attempt < *old.Attempt {
			return Tenure{}, errors.New("lease generation or attempt regressed")
		}
		if err := s.end(old); err != nil {
			return Tenure{}, err
		}
	}
	counter, err := fslock.OpenNoFollow(filepath.Join(s.Feed.Dir(), "lease", "seq"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return Tenure{}, err
	}
	defer counter.Close()
	if err := fslock.LockExclusive(counter); err != nil {
		return Tenure{}, err
	}
	defer fslock.Unlock(counter)
	data, err := io.ReadAll(counter)
	if err != nil {
		return Tenure{}, err
	}
	n := int64(0)
	if len(data) > 0 {
		n, err = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return Tenure{}, err
		}
	}
	n = max(n, old.Fence)
	if n < 0 || n == 1<<63-1 {
		return Tenure{}, errors.New("lease fence exhausted")
	}
	next.Fence = n + 1
	if err := rewriteTenure(counter, []byte(strconv.FormatInt(next.Fence, 10)+"\n")); err != nil {
		return Tenure{}, err
	}
	b, err = json.Marshal(next)
	if err == nil {
		err = rewriteTenure(f, append(b, '\n'))
	}
	return next, err
}
func rewriteTenure(f *os.File, b []byte) error {
	n, err := f.WriteAt(b, 0)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	if err := f.Truncate(int64(len(b))); err != nil {
		return err
	}
	return feed.Sync(f)
}
func (s TaskStore) end(old Tenure) error {
	name := fmt.Sprintf("%s.%d.jsonl", *old.Holder, old.Fence)
	f, err := fslock.OpenNoFollow(filepath.Join(s.Feed.Dir(), "lease", "takeovers.jsonl"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	cut := bytes.LastIndexByte(data, '\n') + 1
	for _, line := range bytes.Split(data[:cut], []byte{'\n'}) {
		var v map[string]any
		if json.Unmarshal(line, &v) == nil && v["file"] == name {
			return nil
		}
	}
	size := int64(0)
	info, err := os.Stat(filepath.Join(s.Feed.Dir(), name))
	if err == nil {
		size = info.Size()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := json.Marshal(map[string]any{"writer": *old.Holder, "file": name, "fence": old.Fence, "end_offset": size, "at": s.now()})
	if err == nil {
		err = contract.Validate("takeover", b)
	}
	if err != nil {
		return err
	}
	if err := f.Truncate(int64(cut)); err != nil {
		return err
	}
	if _, err := f.Seek(int64(cut), 0); err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := f.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return feed.Sync(f)
}
