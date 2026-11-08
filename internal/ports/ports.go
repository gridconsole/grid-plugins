// Package ports reserves local ten-port blocks for browser Jobs.
package ports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

const BlockSize = 10

// Range is inclusive; incomplete blocks at its end are never allocated.
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (r Range) Defaults() Range {
	if r == (Range{}) {
		return Range{41000, 41999}
	}
	return r
}
func (r Range) Validate() error {
	if r.Start < 1 || r.End > 65535 || r.End-r.Start+1 < BlockSize {
		return errors.New("port range must contain at least ten ports between 1 and 65535")
	}
	return nil
}

// Record is local allocator metadata, not a Feed or task record. Task is a
// relative path under the Grid home; Lease and Fence name the read-only grant.
// Group means PID is the dev server's process group, otherwise its owner pid.
type Record struct {
	Range
	PID   int       `json:"pid"`
	Group bool      `json:"group"`
	Job   string    `json:"job"`
	Task  string    `json:"task"`
	Lease string    `json:"lease"`
	Fence uint64    `json:"fence"`
	Time  time.Time `json:"time"`
}

type Store struct {
	GridHome string
	Now      func() time.Time
	// Warn receives one line per damaged port file that List skips; nil is stderr.
	Warn io.Writer
}

func (s Store) warn(format string, args ...any) {
	w := s.Warn
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "ports: warning: "+format+"\n", args...)
}

// Locked holds the allocator's fslock while examining or changing port files.
// Close must always be called. Env uses the same transaction to keep startup,
// pid handoff, health failure cleanup and concurrent gc/release indivisible.
type Locked struct {
	store Store
	dir   string
	file  *os.File
}

func (s Store) Lock() (*Locked, error) {
	if s.GridHome == "" {
		return nil, errors.New("Grid home is required")
	}
	dir := filepath.Join(gridhome.Real(s.GridHome), "ports")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("ports must be a real directory")
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, ".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Locked{s, dir, f}, nil
}
func (l *Locked) Close() { fslock.Unlock(l.file); _ = l.file.Close() }
func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Store) Claim(taskDir, id string, pid int, ports Range) (Record, error) {
	l, err := s.Lock()
	if err != nil {
		return Record{}, err
	}
	defer l.Close()
	return l.Claim(taskDir, id, pid, ports)
}

func (l *Locked) taskPath(taskDir string) (string, error) {
	rel, err := filepath.Rel(gridhome.Real(l.store.GridHome), gridhome.Real(taskDir))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("task must be inside the Grid home")
	}
	return filepath.ToSlash(rel), nil
}
func (l *Locked) Find(taskDir, id string) (Record, bool, error) {
	if !job.ValidID(id) {
		return Record{}, false, errors.New("invalid Job id")
	}
	task, err := l.taskPath(taskDir)
	if err != nil {
		return Record{}, false, err
	}
	records, err := l.List()
	if err != nil {
		return Record{}, false, err
	}
	for _, r := range records {
		if r.Task == task && r.Job == id {
			return r, true, nil
		}
	}
	return Record{}, false, nil
}

func (l *Locked) Claim(taskDir, id string, pid int, ports Range) (Record, error) {
	if pid <= 0 {
		return Record{}, errors.New("a positive owner pid is required")
	}
	ports = ports.Defaults()
	if err := ports.Validate(); err != nil {
		return Record{}, err
	}
	old, found, err := l.Find(taskDir, id)
	if err != nil {
		return Record{}, err
	}
	if found {
		return old, nil
	}
	task, _ := l.taskPath(taskDir)
	grant, err := lease.ReadJob(taskDir, id)
	if err != nil {
		return Record{}, err
	}
	for start := ports.Start; start+BlockSize-1 <= ports.End; start += BlockSize {
		r := Record{Range: Range{start, start + BlockSize - 1}, PID: pid, Job: id, Task: task, Lease: id, Fence: grant.Fence, Time: l.store.now()}
		data, err := json.Marshal(r)
		if err != nil {
			return Record{}, err
		}
		// Every writer holds the allocator lock, so the existence check is exclusive.
		// Each file is written whole by rename, so a crash never leaves a torn one.
		var created []string
		var createErr error
		taken := false
		for port := r.Start; port <= r.End; port++ {
			path := filepath.Join(l.dir, strconv.Itoa(port)+".json")
			if _, err := os.Lstat(path); err == nil {
				taken = true
				break
			} else if !errors.Is(err, os.ErrNotExist) {
				createErr = err
				break
			}
			created = append(created, path)
			if createErr = atomicfile.Write(path, append(data, '\n'), 0o600); createErr != nil {
				break
			}
		}
		if createErr == nil && !taken {
			return r, nil
		}
		for _, path := range created {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return Record{}, errors.Join(createErr, err)
			}
		}
		if createErr != nil {
			return Record{}, createErr
		}
	}
	return Record{}, errors.New("no free ten-port block in the environment port range")
}

// SetProcess hands a claimed block to the dev server group. Call only while
// holding the allocator lock and stop the server if any write fails.
func (l *Locked) SetProcess(r Record, pid int) (Record, error) {
	if pid <= 0 {
		return r, errors.New("a positive server pid is required")
	}
	r.PID, r.Group = pid, true
	data, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	for port := r.Start; port <= r.End; port++ {
		// A crash part-way leaves files that disagree; List treats that block as
		// damaged and gc removes it.
		if err := atomicfile.Write(filepath.Join(l.dir, strconv.Itoa(port)+".json"), append(data, '\n'), 0o600); err != nil {
			return r, err
		}
	}
	return r, nil
}

func (s Store) List() ([]Record, error) {
	l, err := s.Lock()
	if err != nil {
		return nil, err
	}
	defer l.Close()
	return l.List()
}

// Damaged is a port file that holds no valid, agreeing, complete reservation.
// Every writer holds the allocator lock and writes whole files, so one is always
// a crash leftover. Records are the parsed reservations of its block, if any.
type Damaged struct {
	Port    int
	Reason  string
	Records []Record
}

// List returns the valid blocks and warns about, but skips, damaged files.
func (l *Locked) List() ([]Record, error) {
	records, damaged, err := l.Scan()
	for _, d := range damaged {
		l.store.warn("skipping damaged reservation file %d.json (%s); grid ports gc removes it", d.Port, d.Reason)
	}
	return records, err
}

func (l *Locked) Scan() ([]Record, []Damaged, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, nil, err
	}
	damaged := []Damaged{}
	byStart := map[int][]int{}
	parsed := map[int]Record{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		f, err := fslock.OpenNoFollow(filepath.Join(l.dir, e.Name()), os.O_RDONLY, 0)
		if err != nil {
			return nil, nil, err
		}
		var r Record
		err = json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&r)
		_ = f.Close()
		if err != nil {
			damaged = append(damaged, Damaged{Port: port, Reason: err.Error()})
			continue
		}
		if r.Range.Validate() != nil || r.End-r.Start+1 != BlockSize || port < r.Start || port > r.End || r.PID <= 0 || !job.ValidID(r.Job) || r.Lease != r.Job || r.Time.IsZero() {
			damaged = append(damaged, Damaged{Port: port, Reason: "invalid reservation", Records: []Record{r}})
			continue
		}
		rel, err := l.taskPath(filepath.Join(l.store.GridHome, filepath.FromSlash(r.Task)))
		if err != nil || rel != r.Task {
			damaged = append(damaged, Damaged{Port: port, Reason: "invalid reservation task path", Records: []Record{r}})
			continue
		}
		parsed[port] = r
		byStart[r.Start] = append(byStart[r.Start], port)
	}
	records := []Record{}
	for start, members := range byStart {
		first := parsed[members[0]]
		reason := ""
		for _, port := range members {
			if parsed[port] != first {
				reason = "files of the block disagree"
			}
		}
		if reason == "" && len(members) != BlockSize {
			reason = "block is incomplete"
		}
		if reason == "" {
			records = append(records, first)
			continue
		}
		var all []Record
		seen := map[Record]bool{}
		for _, port := range members {
			if r := parsed[port]; !seen[r] {
				seen[r] = true
				all = append(all, r)
			}
		}
		for _, port := range members {
			damaged = append(damaged, Damaged{Port: port, Reason: reason + fmt.Sprintf(" (block %d)", start), Records: all})
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Start < records[j].Start })
	sort.Slice(damaged, func(i, j int) bool { return damaged[i].Port < damaged[j].Port })
	return records, damaged, nil
}

func (s Store) Release(taskDir, id string) error {
	l, err := s.Lock()
	if err != nil {
		return err
	}
	defer l.Close()
	r, found, err := l.Find(taskDir, id)
	if err != nil || !found {
		return err
	}
	if r.Group && Alive(r.PID, true) {
		return errors.New("dev server is still running; use grid env down")
	}
	return l.Remove(r)
}

// Remove frees this block while the caller owns the allocator lock.
func (l *Locked) Remove(r Record) error {
	var errs []error
	for port := r.Start; port <= r.End; port++ {
		err := os.Remove(filepath.Join(l.dir, strconv.Itoa(port)+".json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// GC reclaims dead owners immediately. Expired leases are reclaimed only when
// no TCP listener remains in the block (go-architecture.md:245).
func (s Store) GC() ([]Record, error) {
	l, err := s.Lock()
	if err != nil {
		return nil, err
	}
	defer l.Close()
	records, damaged, err := l.Scan()
	if err != nil {
		return nil, err
	}
	removed := []Record{}
	for _, d := range damaged {
		if err := os.Remove(filepath.Join(l.dir, strconv.Itoa(d.Port)+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		s.warn("removed damaged reservation file %d.json (%s)", d.Port, d.Reason)
	}
	for _, r := range records {
		reclaim := !Alive(r.PID, r.Group)
		if !reclaim {
			status, err := lease.ReadJob(filepath.Join(s.GridHome, filepath.FromSlash(r.Task)), r.Lease)
			if err != nil {
				return removed, err
			}
			reclaim = status.Expired(r.Fence, s.now()) && !listening(r.Range)
		}
		if reclaim {
			if err := l.Remove(r); err != nil {
				return removed, err
			}
			removed = append(removed, r)
		}
	}
	return removed, nil
}

// Available checks both TCP address families before env starts a server, so an
// unrelated listener cannot satisfy the new server's health check. This is a
// probe, not an OS socket reservation; other programs can still bind later.
func (r Range) Available() bool { return r.Validate() == nil && !listening(r) }

func listening(r Range) bool {
	for port := r.Start; port <= r.End; port++ {
		if busy(strconv.Itoa(port)) {
			return true
		}
	}
	return false
}

// busy reports a listener on one port. On BSD and macOS a wildcard bind succeeds
// while 127.0.0.1:p or [::1]:p is listening (Go sets SO_REUSEADDR), so a loopback
// dev server is found by connecting to it and by binding its exact address; the
// wildcard bind still finds a listener on any other address of the machine.
func busy(port string) bool {
	for _, host := range []string{"127.0.0.1", "::1"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	for _, addr := range []struct{ network, host string }{{"tcp4", "127.0.0.1"}, {"tcp6", "::1"}, {"tcp4", ""}, {"tcp6", ""}} {
		listener, err := net.Listen(addr.network, net.JoinHostPort(addr.host, port))
		if err != nil {
			// An address family the machine lacks has no listeners. Any other bind
			// failure is insufficient evidence that the port is free.
			if networkUnsupported(err) {
				continue
			}
			return true
		}
		_ = listener.Close()
	}
	return false
}
