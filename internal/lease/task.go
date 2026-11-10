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
	// BeforeTaskGrant runs under the task lease lock once the new fence is chosen and
	// before the tenure is written; an error leaves the old tenure in the lease file.
	BeforeTaskGrant func(next, old Tenure) error
}

func (s TaskStore) now() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return contract.FormatTime(now)
}
func (s TaskStore) open(name string, flags int) (*os.File, error) {
	if name != "task" && !job.ValidID(name) {
		return nil, errors.New("invalid lease name")
	}
	dir := filepath.Join(s.Feed.Dir(), "lease")
	if flags&os.O_CREATE != 0 {
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
	return fslock.OpenNoFollow(filepath.Join(dir, name+".json"), flags, 0o600)
}
func readTenure(f *os.File) (Tenure, error) {
	var t Tenure
	b, err := io.ReadAll(f)
	if err != nil || len(bytes.TrimSpace(b)) == 0 {
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
	f, err := s.open(name, os.O_RDONLY)
	if err != nil {
		return Tenure{}, err
	}
	defer f.Close()
	if err := fslock.LockShared(f); err != nil {
		return Tenure{}, err
	}
	defer fslock.Unlock(f)
	t, err := readTenure(f)
	if err == nil && name == "task" && t.Lease.Lease == "" {
		// A first grant can crash after creating the file but before writing it.
		t.Lease.Lease = name
	}
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
	// Coordinate with task folder moves, including the first lease when no
	// lease file exists. Never recreate a task directory after waiting on a
	// lock that moved to the trash; every waiter must still name this inode.
	lockPath := filepath.Join(s.Feed.TaskDir, ".task.lock")
	lock, err := fslock.OpenNoFollow(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return Tenure{}, err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return Tenure{}, err
	}
	defer fslock.Unlock(lock)
	held, err := lock.Stat()
	if err != nil {
		return Tenure{}, err
	}
	current, err := os.Lstat(lockPath)
	if err != nil {
		return Tenure{}, err
	}
	if !os.SameFile(held, current) {
		return Tenure{}, errors.New("task folder changed while acquiring its lease")
	}
	policy := TaskTake{}
	if len(options) > 0 {
		policy = options[0]
	}
	return s.grant(Tenure{Lease: feed.Lease{Lease: "task", Holder: &w.Worker, Gen: w.Gen}, Since: s.now(), PID: &pid, PIDStart: &start}, func(old Tenure) error {
		if old.Holder == nil {
			return nil
		}
		if policy.Adopt == policy.Takeover || policy.Gen != old.Gen || policy.Fence != old.Fence {
			return taskHeld(old)
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
	}, s.BeforeTaskGrant)
}

// ErrTaskHeld is wrapped by every refusal that names the tenure holding the task lease.
var ErrTaskHeld = errors.New("task lease held")

type taskHeldError struct{ tenure Tenure }

func (e taskHeldError) Error() string {
	holder := "nobody"
	if e.tenure.Holder != nil {
		holder = *e.tenure.Holder
	}
	return fmt.Sprintf("task lease held by %s since %s (epoch %d.%d)", holder, e.tenure.Since, e.tenure.Gen, e.tenure.Fence)
}
func (taskHeldError) Unwrap() error { return ErrTaskHeld }

func taskHeld(t Tenure) error { return taskHeldError{t} }

// ErrTaskFree is returned by ReleaseTask when no tenure holds the task lease.
var ErrTaskFree = errors.New("no task lease is held")

// AdoptTask replaces only a provably dead holder on the writer's host. The
// identity check and probe run while the task lease is exclusively locked.
func (s TaskStore) AdoptTask(w feed.Writer, pid int, start string, probe func(session.Live) session.ProcessState) (Tenure, Tenure, error) {
	var old Tenure
	next, err := s.grant(Tenure{Lease: feed.Lease{Lease: "task", Holder: &w.Worker, Gen: w.Gen}, Since: s.now(), PID: &pid, PIDStart: &start}, func(current Tenure) error {
		old = current
		if current.Holder == nil {
			return taskHeld(current)
		}
		machine := w.Worker[strings.LastIndexByte(w.Worker, '@'):]
		if !strings.HasSuffix(*current.Holder, machine) || current.PID == nil || current.PIDStart == nil || probe == nil {
			return taskHeld(current)
		}
		live := session.Live{Lease: "task", Holder: *current.Holder, Fence: uint64(current.Fence), PID: *current.PID, PIDStart: *current.PIDStart}
		if probe(live) != session.Dead {
			return taskHeld(current)
		}
		return nil
	}, s.BeforeTaskGrant)
	return next, old, err
}

// Acquired says how AcquireTask got the task lease.
type Acquired string

const (
	// AcquiredTaken: the lease was free and has a new fence.
	AcquiredTaken Acquired = "taken"
	// AcquiredReused: the tenure already named this holder, pid and pid start; nothing was written.
	AcquiredReused Acquired = "reused"
	// AcquiredAdopted: a holder provably gone on this host was replaced, with a new, higher fence.
	AcquiredAdopted Acquired = "adopted"
)

// Acquisition is what AcquireTask did: the tenure now in force, how it came to
// be, and the tenure it replaced when adopted.
type Acquisition struct {
	Tenure Tenure
	How    Acquired
	From   Tenure
}

// AcquireTask gives the writer the task lease for the process pid/start, for a
// supervisor that keeps one tenure across restarts of its own streams:
//   - a free lease is taken;
//   - a tenure that already names this holder, pid and pid start is reused, and
//     no fence is granted;
//   - a holder on this host whose process is provably gone is adopted, with a
//     new, higher fence from the per-task counter (not the old fence plus one);
//   - any other holder keeps the lease, and the error wraps ErrTaskHeld without
//     a write.
//
// w.Gen is replaced by the generation already on the lease file, as grant
// refuses a regression.
func (s TaskStore) AcquireTask(w feed.Writer, pid int, start string, probe func(session.Live) session.ProcessState) (Acquisition, error) {
	// The machine is read off the worker id below; refuse an id without one before any read or write.
	if !strings.Contains(w.Worker, "@") {
		return Acquisition{}, fmt.Errorf("worker id %q must be member@machine", w.Worker)
	}
	cur, err := s.Read("task")
	if errors.Is(err, os.ErrNotExist) {
		cur, err = Tenure{}, nil
	}
	if err != nil {
		return Acquisition{}, err
	}
	if cur.Fence > 0 {
		w.Gen = cur.Gen
	}
	if cur.Holder == nil {
		t, err := s.TakeTask(w, pid, start)
		return Acquisition{Tenure: t, How: AcquiredTaken}, err
	}
	if *cur.Holder == w.Worker && cur.PID != nil && *cur.PID == pid && cur.PIDStart != nil && *cur.PIDStart == start {
		return Acquisition{Tenure: cur, How: AcquiredReused}, nil
	}
	// Probe before taking the exclusive lock, so a live holder is never locked out
	// of its own file once a second.
	machine := w.Worker[strings.LastIndexByte(w.Worker, '@'):]
	if !strings.HasSuffix(*cur.Holder, machine) || cur.PID == nil || cur.PIDStart == nil || probe == nil ||
		probe(session.Live{Lease: "task", Holder: *cur.Holder, Fence: uint64(cur.Fence), PID: *cur.PID, PIDStart: *cur.PIDStart}) != session.Dead {
		return Acquisition{}, taskHeld(cur)
	}
	next, old, err := s.AdoptTask(w, pid, start, probe)
	return Acquisition{Tenure: next, How: AcquiredAdopted, From: old}, err
}

// ReleaseTask ends a tenure only for its exact holder, generation and fence.
// A free task lease is ErrTaskFree and nothing is created.
func (s TaskStore) ReleaseTask(w feed.Writer) error {
	f, err := s.open("task", os.O_RDWR)
	if errors.Is(err, os.ErrNotExist) {
		return ErrTaskFree
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	old, err := readTenure(f)
	if err != nil {
		return err
	}
	if old.Holder == nil {
		return ErrTaskFree
	}
	if w.Lease != "task" || *old.Holder != w.Worker || old.Gen != w.Gen || old.Fence != w.Fence {
		return taskHeld(old)
	}
	if err := s.end(old); err != nil {
		return err
	}
	old.Holder, old.PID, old.PIDStart = nil, nil, nil
	b, err := json.Marshal(old)
	if err == nil {
		err = contract.Validate("lease", b)
	}
	if err != nil {
		return err
	}
	return rewriteTenure(f, append(b, '\n'))
}

// GrantJob requires the current task holder throughout the grant and snapshot.
func (s TaskStore) GrantJob(task feed.Writer, id, to string, attempt int) (t Tenure, err error) {
	err = s.Feed.WithLease(task, nil, func() error {
		var e error
		t, e = s.grant(Tenure{Lease: feed.Lease{Lease: id, Holder: &to, Gen: task.Gen, Attempt: &attempt}, Since: s.now()}, nil, nil)
		return e
	})
	return
}
func (s TaskStore) grant(next Tenure, check func(Tenure) error, prepare func(next, old Tenure) error) (Tenure, error) {
	// Validate before creating anything or ending the old tenure.
	next.Fence = 1
	b, err := json.Marshal(next)
	if err == nil {
		err = contract.Validate("lease", b)
	}
	if err != nil {
		return Tenure{}, err
	}
	f, err := s.open(next.Lease.Lease, os.O_RDWR|os.O_CREATE)
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
	if check != nil {
		if err := check(old); err != nil {
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
	if prepare != nil {
		if err := prepare(next, old); err != nil {
			return Tenure{}, err
		}
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
