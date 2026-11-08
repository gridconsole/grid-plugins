package lease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
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

var jobName = regexp.MustCompile(`^J-[1-9][0-9]*(\.[1-9][0-9]*)*$`)

func (s TaskStore) now() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return contract.FormatTime(now)
}
func (s TaskStore) open(name string, create bool) (*os.File, error) {
	if name != "task" && !jobName.MatchString(name) {
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
	if err := dec.Decode(&t); err != nil {
		return t, err
	}
	b, err = json.Marshal(t)
	if err == nil {
		err = contract.Validate("lease", b)
	}
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
	return readTenure(f)
}

// TakeTask creates the first task tenure. A live or unknown holder is never
// replaced here; adoption requires a separate, proven process-identity decision.
func (s TaskStore) TakeTask(w feed.Writer, pid int, start string) (Tenure, error) {
	return s.grant(Tenure{Lease: feed.Lease{Lease: "task", Holder: &w.Worker, Gen: w.Gen}, Since: s.now(), PID: &pid, PIDStart: &start}, true)
}

// GrantJob requires the current task holder throughout the grant and snapshot.
func (s TaskStore) GrantJob(task feed.Writer, id, to string, attempt int) (t Tenure, err error) {
	err = s.Feed.WithLease(task, nil, func() error {
		var e error
		t, e = s.grant(Tenure{Lease: feed.Lease{Lease: id, Holder: &to, Gen: task.Gen, Attempt: &attempt}, Since: s.now()}, false)
		return e
	})
	return
}
func (s TaskStore) grant(next Tenure, first bool) (Tenure, error) {
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
	if first && old.Holder != nil {
		return Tenure{}, errors.New("task lease held")
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
