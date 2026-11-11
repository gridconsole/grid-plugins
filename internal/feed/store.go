package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

var ErrStaleLease = errors.New("stale lease")

// RecordIDUsedError is the cause of the refusal of a post whose record id a
// record of another writer tenure already has on the Feed. It is returned inside
// an InvalidRecordError, so it exits 3 like the other reuse refusals and keeps
// their text; the CLI names it record_id_used.
type RecordIDUsedError struct{ ID string }

func (e *RecordIDUsedError) Error() string { return "record id already used for a different post" }

var ErrDiskFull = errors.New("disk full")

// Store is one task's Feed. TaskID is the stable ULID, not its display number.
// Post initializes a missing TaskID from task.md under the task lock.
type Store struct {
	TaskDir, TaskID string
	Now             func() time.Time
	// WithdrawAllowed overrides person withdrawal roles for projections only.
	// Nil reads workflow.md. Writes always check the current policy separately.
	WithdrawAllowed func(string) bool
	// Context stops a Post or a lease check that is waiting on a Feed file lock:
	// the call returns the context's error and writes nothing. Nil means
	// context.Background, which waits as long as the holder keeps the lock.
	Context context.Context
	// Warn receives the warning a fold prints when workflow.md cannot be read
	// ("cannot read task withdrawal policy (workflow.md); using withdraw: []"). A
	// host that runs many calls in one process sets it to the call's own stderr.
	// Nil is the process stderr, as before.
	Warn io.Writer
	// afterGateCheck is a test hook inside the gate lock, before the append.
	afterGateCheck func()
	// afterJobCheck is a test hook inside the job lock, before the append.
	afterJobCheck func()
}

func (s Store) Dir() string { return filepath.Join(s.TaskDir, "feed") }

// CheckFilesystem also serves callers such as doctor. It checks the nearest
// existing ancestor before any directory is created.
func CheckFilesystem(dir string) error {
	for {
		if _, err := os.Stat(dir); err == nil {
			return checkFilesystem(dir)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("cannot find Feed filesystem")
		}
		dir = parent
	}
}

func (s Store) prepare() error {
	if !IsID(s.TaskID) {
		return fmt.Errorf("task id must be a ULID")
	}
	info, err := os.Stat(s.TaskDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("task is not a folder")
	}
	if err := CheckFilesystem(s.Dir()); err != nil {
		return err
	}
	_, statErr := os.Stat(s.Dir())
	if err := os.MkdirAll(s.Dir(), 0o700); err != nil {
		return err
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return syncDir(s.TaskDir)
	}
	return nil
}

// syncDir makes the entries of a folder durable, so a new file or folder in it
// survives a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return syncFile(d)
}

// Lease is the shared interface with the lease commands. Feed never grants a
// lease or decides that a holder is dead.
type Lease struct {
	Lease   string  `json:"lease"`
	Holder  *string `json:"holder"`
	Fence   int64   `json:"fence"`
	Gen     int64   `json:"gen"`
	Attempt *int    `json:"attempt"`
}

type tenureEnd struct {
	File      string `json:"file"`
	EndOffset int64  `json:"end_offset"`
}

func (s Store) leaseLock(w Writer, attempt *int) (*os.File, error) {
	if w.Lease == "" {
		return nil, nil
	}
	f, err := os.Open(filepath.Join(s.Dir(), "lease", w.Lease+".json"))
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read lease: %v", ErrStaleLease, err)
	}
	// The wait owns f from here: a failed or cancelled lock closes it, not us.
	if err := fslock.LockSharedContext(s.Context, f); err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { fslock.Unlock(f); f.Close(); return nil, err }
	var lease Lease
	if err := json.NewDecoder(f).Decode(&lease); err != nil {
		return fail(fmt.Errorf("%w: invalid lease", ErrStaleLease))
	}
	if value(lease.Holder) != w.Worker || lease.Lease != w.Lease || lease.Fence != w.Fence || lease.Gen != w.Gen {
		return fail(ErrStaleLease)
	}
	if w.Lease != "task" && (attempt == nil || lease.Attempt == nil || *attempt != *lease.Attempt) {
		return fail(ErrStaleLease)
	}
	ends, err := s.readEnds()
	if err != nil {
		return fail(err)
	}
	if _, ended := ends[writerFile(w.Worker, w.Fence)]; ended {
		return fail(ErrStaleLease)
	}
	return f, nil
}

func (s Store) readEnds() (map[string]int64, error) {
	ends := map[string]int64{}
	data, err := os.ReadFile(filepath.Join(s.Dir(), "lease", "takeovers.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return ends, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range completeLines(data) {
		var end tenureEnd
		// A malformed complete takeover is unsafe to ignore: do not admit zombies.
		if json.Unmarshal(line, &end) != nil || end.File == "" || filepath.Base(end.File) != end.File || end.EndOffset < 0 {
			return nil, fmt.Errorf("invalid takeover log entry")
		}
		if old, exists := ends[end.File]; !exists || end.EndOffset < old {
			ends[end.File] = end.EndOffset
		}
	}
	return ends, nil
}

func completeLines(data []byte) [][]byte {
	cut := bytes.LastIndexByte(data, '\n')
	if cut < 0 {
		return nil
	}
	return bytes.Split(data[:cut], []byte{'\n'})
}

// Post appends with a file lock held across seq allocation, tail repair, the
// whole-line write and required durability. A lease lock covers that interval.
func (s Store) Post(in Input, w Writer) (Record, error) {
	if err := s.CheckWithdrawalPolicy(in); err != nil {
		return Record{}, err
	}
	if s.TaskID == "" {
		var err error
		s.TaskID, err = EnsureTaskID(s.TaskDir)
		if err != nil {
			return Record{}, err
		}
	}
	r, err := s.record(in, w)
	if err != nil {
		return Record{}, err
	}
	if err := validate(r); err != nil {
		return Record{}, err
	}
	if gateID(r) != "" {
		roles, err := workflow.GatePostRoles(s.TaskDir)
		if err != nil {
			var pathErr *os.PathError
			var syscallErr *os.SyscallError
			if errors.As(err, &pathErr) || errors.As(err, &syscallErr) {
				return Record{}, fmt.Errorf("read gate post policy: %w", err)
			}
			return Record{}, &InvalidRecordError{Err: err}
		}
		kind := str(object(r.Body["gate"])["kind"])
		if !slices.Contains(roles[kind], contract.NormalizeRole(r.Author.Role)) {
			return Record{}, &InvalidRecordError{Err: fmt.Errorf("role %s may not post %s gates", r.Author.Role, kind)}
		}
	}
	if err := s.prepare(); err != nil {
		return Record{}, err
	}
	if r.Wake != nil {
		guard, err := s.wakeLock()
		if err != nil {
			return Record{}, err
		}
		defer guard.Close()
		defer fslock.Unlock(guard)
	}
	lease, err := s.leaseLock(w, r.Attempt)
	if err != nil {
		return Record{}, err
	}
	if lease != nil {
		defer lease.Close()
		defer fslock.Unlock(lease)
	}
	// Gate names belong to the task, not a writer tenure. Hold their lock
	// across the cross-writer check and append, after lease validation and
	// before taking the tenure file lock.
	if gateID(r) != "" {
		guard, err := s.gateLock()
		if err != nil {
			return Record{}, err
		}
		defer guard.Close()
		defer fslock.Unlock(guard)
	}
	if gid := gateID(r); gid != "" {
		existing, err := s.ReadOnly(Filter{Type: "question"})
		if err != nil {
			return Record{}, err
		}
		for _, entry := range existing.Records {
			if gateID(entry.Record) == gid && !(entry.ID == r.ID && sameRequest(entry.Record, r)) {
				return Record{}, &InvalidRecordError{Err: fmt.Errorf("gate id %s already exists (question %s)", gid, entry.ID)}
			}
		}
		if s.afterGateCheck != nil {
			s.afterGateCheck()
		}
	} else if jid := jobID(r); jid != "" {
		// A Job id belongs to the task, not a writer tenure: the fold keeps the
		// first record and ignores the rest. Hold job.lock across the check and the
		// append (lock order at jobLock); a same-request retry falls through to the
		// tenure scan, which returns the stored record.
		guard, err := s.jobLock()
		if err != nil {
			return Record{}, err
		}
		defer guard.Close()
		defer fslock.Unlock(guard)
		if err := s.checkJobFree(r, jid); err != nil {
			return Record{}, err
		}
		if s.afterJobCheck != nil {
			s.afterJobCheck()
		}
	}
	file := filepath.Join(s.Dir(), writerFile(w.Worker, w.Fence))
	f, err := fslock.OpenNoFollow(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return Record{}, diskError(err)
	}
	// The wait owns f from here: a failed or cancelled lock closes it, so the
	// close is deferred only once the lock is held.
	if err := fslock.LockExclusiveContext(s.Context, f); err != nil {
		return Record{}, err
	}
	defer f.Close()
	defer fslock.Unlock(f)
	// Repair on open: a writer that died mid-line left a fragment, and an empty
	// file is new, so its name is made durable before any record in it is acked.
	size, err := repairTail(f)
	if err != nil {
		return Record{}, diskError(err)
	}
	if size == 0 {
		if err := syncDir(s.Dir()); err != nil {
			return Record{}, diskError(err)
		}
	}
	// Numbering and the retry check read only this tenure file, which the
	// exclusive lock keeps stable; the whole Feed is read only by the checks
	// that depend on other tenures.
	raw, err := os.ReadFile(file)
	if err != nil {
		return Record{}, err
	}
	r.Seq = 1
	for _, line := range completeLines(raw) {
		var prev Record
		if json.Unmarshal(line, &prev) != nil || prev.Worker != r.Worker || prev.Fence != r.Fence {
			continue
		}
		if prev.ID == r.ID {
			old, err := s.stored(prev)
			if err != nil {
				return Record{}, err
			}
			if !sameRequest(old, r) {
				return Record{}, &InvalidRecordError{Err: fmt.Errorf("record id already used for a different post")}
			}
			if t, _ := contract.LookupType(r.Type); t.Class == "state" {
				if err := syncFile(f); err != nil {
					return Record{}, diskError(err)
				}
			}
			return old, nil
		}
		if prev.Seq >= r.Seq {
			r.Seq = prev.Seq + 1
		}
	}
	// A record id names one record on the whole Feed. A repeat by this tenure
	// returned above; a record of any other tenure with the id refuses the post.
	onFeed, err := s.ReadOnly(Filter{})
	if err != nil {
		return Record{}, err
	}
	for _, entry := range onFeed.Records {
		if entry.ID == r.ID {
			return Record{}, &InvalidRecordError{Err: &RecordIDUsedError{ID: r.ID}}
		}
	}
	guestAnswer := contract.NormalizeRole(r.Author.Role) == "guest" && r.Type == "answer"
	if r.Wake != nil || r.Type == "ack" || guestAnswer {
		existing, err := s.Read(Filter{})
		if err != nil {
			return Record{}, err
		}
		for _, entry := range existing.Records {
			old := entry.Record
			if old.ID == r.ID {
				return Record{}, &InvalidRecordError{Err: fmt.Errorf("record id already used for a different post")}
			}
			if r.Wake != nil && old.Wake != nil && wakeKey(old) == wakeKey(r) {
				return old, nil
			}
		}
		if r.Type == "ack" {
			matched := false
			state := contract.Fold(posts(existing))
			if job := state.Jobs[value(r.Job)]; job != nil && job.Assign != nil {
				a := job.Assign
				matched = a.To == r.Worker && a.Fence == r.Fence && job.Generation == r.Gen && r.Attempt != nil && a.Attempt == *r.Attempt && slices.Contains(r.After, job.AssignPost)
			}
			if !matched {
				return Record{}, &InvalidRecordError{Err: fmt.Errorf("ack requires its counted assignment in after")}
			}
		}
		if guestAnswer {
			addressed := false
			for _, entry := range existing.Records {
				q := entry.Record
				if q.ID == value(r.Re) && q.Type == "question" && entry.Verdict == "counted" && str(q.Body["who"]) == r.Author.ID {
					addressed = true
				}
				// Or a person Job whose intent.owner is this guest, answered with no lease.
				if q.ID == value(r.Re) && q.Type == "job" && entry.Verdict == "counted" {
					intent, _ := q.Body["intent"].(map[string]any)
					addressed = addressed || contract.GuestAddressee(r.Author.Role, r.Author.ID, r.Lease != nil || r.Fence != 0, str(q.Body["role"]), str(intent["owner"]), str(q.Body["kind"]))
				}
			}
			if !addressed {
				return Record{}, &InvalidRecordError{Err: fmt.Errorf("guest answer must name a question addressed to that guest")}
			}
		}
	}
	if r.Seq == 1 && r.Lease != nil {
		first := "ack"
		if *r.Lease == "task" {
			first = "handover"
		}
		if r.Type != first {
			return Record{}, &InvalidRecordError{Err: fmt.Errorf("first record of a tenure must be %s", first)}
		}
	}
	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, err
	}
	if len(line)+1 > MaxLine {
		body, err := json.Marshal(r.Body)
		if err != nil {
			return Record{}, err
		}
		if err := s.saveBody(r.ID, body); err != nil {
			return Record{}, diskError(err)
		}
		wire := r
		wire.Body = map[string]any{"ref": "bodies/" + r.ID + ".json"}
		line, err = json.Marshal(wire)
		if err != nil {
			return Record{}, err
		}
		if len(line)+1 > MaxLine {
			return Record{}, &InvalidRecordError{Err: fmt.Errorf("envelope exceeds 32 KB even with body reference")}
		}
	}
	line = append(line, '\n')
	t, _ := contract.LookupType(r.Type)
	if err := appendRecord(f, line, writeOnce); err != nil {
		return Record{}, err
	}
	if t.Class == "state" {
		if err := syncFile(f); err != nil {
			return Record{}, diskError(err)
		}
	}
	return r, nil
}

// write is one OS write, not os.File.Write's internal partial-write loop. Repair
// runs before every retry while the same tenure lock remains held.
func appendRecord(f *os.File, line []byte, write func(*os.File, []byte) (int, error)) error {
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if _, err := repairTail(f); err != nil {
				return diskError(err)
			}
		}
		n, err := write(f, line)
		if errors.Is(err, syscall.EINTR) || errors.Is(err, io.ErrShortWrite) {
			continue
		}
		if err != nil {
			return diskError(err)
		}
		if n == len(line) {
			return nil
		}
	}
	return io.ErrShortWrite
}

func diskError(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return fmt.Errorf("%w: %v", ErrDiskFull, err)
	}
	return err
}

// repairTail ends a fragment left by a writer that died mid-line with one
// newline, so the next line never glues onto it, and returns the file's size
// before the repair.
func repairTail(f *os.File) (int64, error) {
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return 0, err
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], info.Size()-1); err != nil {
		return 0, err
	}
	if last[0] != '\n' {
		n, err := f.Write([]byte{'\n'})
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, io.ErrShortWrite
		}
	}
	return info.Size(), nil
}

// stored returns a record as the reader sees it, with a spilled body put back.
func (s Store) stored(r Record) (Record, error) {
	ref := str(r.Body["ref"])
	if len(r.Body) != 1 || ref != "bodies/"+r.ID+".json" {
		return r, nil
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(), ref))
	if err != nil {
		return r, err
	}
	var body map[string]any
	if err := decodeBody(data, &body); err != nil || body == nil {
		return r, fmt.Errorf("body of %s is unreadable", r.ID)
	}
	r.Body = body
	return r, nil
}

func (s Store) record(in Input, w Writer) (Record, error) {
	id := in.ID
	if id == "" {
		var err error
		id, err = NewID()
		if err != nil {
			return Record{}, err
		}
	}
	// Normalize and copy the body to avoid mutating caller data and to apply the
	// same JSON type checks to API and CLI inputs.
	data, err := json.Marshal(in.Body)
	if err != nil {
		return Record{}, &InvalidRecordError{Err: err}
	}
	var body map[string]any
	if err := decodeBody(data, &body); err != nil {
		return Record{}, &InvalidRecordError{Err: err}
	}
	t, _ := contract.LookupType(in.Type)
	after := append([]string{}, in.After...)
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	r := Record{ID: id, Task: s.TaskID, Type: in.Type, TypeV: t.Version, Author: w.Author, Tier: contract.TierFor(w.Author.Role), Worker: w.Worker, Lease: ptr(w.Lease), Gen: w.Gen, Fence: w.Fence, At: now.UTC().Format("2006-01-02T15:04:05.000Z"), Re: ptr(in.Re), After: after, Job: ptr(in.Job), Wake: ptr(in.Wake), Body: body}
	if in.Attempt != 0 {
		r.Attempt = &in.Attempt
	}
	return r, nil
}
func sameRequest(a, b Record) bool {
	a.At, b.At = "", ""
	a.Seq, b.Seq = 0, 0
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(aa, bb)
}
func wakeKey(r Record) string {
	target := value(r.Job)
	if target == "" {
		target = str(object(r.Body["assign"])["job"])
	}
	if target == "" {
		target = str(r.Body["target"])
	}
	if target == "" {
		target = value(r.Re)
	}
	if target == "" {
		target = r.Task
	}
	return strings.Split(value(r.Wake), "/")[0] + "\x00" + r.Type + "\x00" + target
}
func (s Store) saveBody(id string, body []byte) error {
	dir := filepath.Join(s.Dir(), "bodies")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	target := filepath.Join(dir, id+".json")
	if old, err := os.ReadFile(target); err == nil {
		if !bytes.Equal(old, body) {
			return &InvalidRecordError{Err: fmt.Errorf("body id already used")}
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(dir, ".body-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err := syncFile(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Link publishes once, avoiding a partially visible body or replacement.
	if err := os.Link(f.Name(), target); err != nil {
		if old, e := os.ReadFile(target); e != nil || !bytes.Equal(old, body) {
			return err
		}
	}
	return syncDir(dir)
}

func (s Store) wakeLock() (*os.File, error) {
	dir := filepath.Join(s.Dir(), ".cache")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "wake.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	// The wait owns f from here: a failed or cancelled lock closes it, not us.
	if err := fslock.LockExclusiveContext(s.Context, f); err != nil {
		return nil, err
	}
	return f, nil
}
