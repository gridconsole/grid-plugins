package brain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// The two operations DropWith runs. A dropped entry stays on disk, with its
// versions, and leaves search, the index and job sources; the audit log in
// knowledge/log.md is the only thing either operation writes.
const (
	OpDrop    = "drop"
	OpRestore = "restore"
)

// DropResult is what DropWith did: the new status (dropped or restored), the
// newest stored version of the entry and the rev of its bytes, which neither
// operation changes.
type DropResult struct {
	Status  string `json:"status"`
	Version int    `json:"version"`
	Rev     string `json:"rev"`
	// Proposed is the draft in Proposals/ a vault drop made instead of deleting a
	// note a person changed since expect.
	Proposed string `json:"proposed,omitempty"`
}

var (
	// ErrDropRefused means the entry's state does not allow the operation.
	ErrDropRefused = errors.New("refused")
	// ErrNoSuchNote means there is no entry of that topic and name.
	ErrNoSuchNote = errors.New("no such note")
	// ErrConflict means the caller's rev is not the entry's current rev.
	ErrConflict = errors.New("stale rev")
)

// RefusedError carries the reason for an ErrDropRefused.
type RefusedError struct{ Reason string }

func (e *RefusedError) Error() string        { return e.Reason }
func (e *RefusedError) Is(target error) bool { return target == ErrDropRefused }

// ConflictError carries the entry's current rev for an ErrConflict.
type ConflictError struct{ Rev string }

func (e *ConflictError) Error() string        { return "stale rev; the entry is at " + e.Rev }
func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

func hyDrRefuse(format string, args ...any) error {
	return &RefusedError{Reason: fmt.Sprintf(format, args...)}
}

// NoteRev is the rev of an entry's bytes: the first 8 bytes of their SHA-256 as
// 16 lower-case hex digits, as the app computes it.
func NoteRev(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

type hyDrState struct {
	status string
	at     time.Time
}

// hyDrStates reads the log once and returns, per "topic/name", the last
// accepted, dropped or restored line. The log is append-only, so file order is
// time order. A missing log is no state. Old five-field lines have no topic and
// are skipped.
func hyDrStates(root *os.Root) (map[string]hyDrState, error) {
	states := map[string]hyDrState{}
	f, err := openLog(root)
	if errors.Is(err, fs.ErrNotExist) {
		return states, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for sc.Scan() {
		c, at, ok := parseAudit(sc.Text())
		if !ok || c.Topic == "" {
			continue
		}
		switch c.Status {
		case "accepted", "dropped", "restored":
			states[c.Topic+"/"+c.Name] = hyDrState{status: c.Status, at: at}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("brain log: %w", err)
	}
	return states, nil
}

// Dropped lists the entries of one level that are dropped now, keyed
// "topic/name", with one read of the log.
func Dropped(root *os.Root) (map[string]bool, error) {
	states, err := hyDrStates(root)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for key, st := range states {
		if st.status == "dropped" {
			out[key] = true
		}
	}
	return out, nil
}

// hyDrKnowledgeTopic refuses a topic that does not hold knowledge entries. The
// level of the root is not known here, so a custom topic counts when it is a
// knowledge topic at either level.
func hyDrKnowledgeTopic(root *os.Root, topic string) error {
	for _, t := range BuiltinTopics() {
		if t.Name == topic {
			if t.Kind != "knowledge" {
				return hyDrRefuse("topic %s holds settings, not knowledge entries", topic)
			}
			return nil
		}
	}
	var last error
	for _, level := range []string{"home", "project"} {
		t, err := topicAt(root, topic, level)
		if err == nil {
			if t.Kind != "knowledge" {
				return hyDrRefuse("topic %s holds settings, not knowledge entries", topic)
			}
			return nil
		}
		last = err
	}
	if topic == versionsTopic || topic == "skills" || topic == "workflows" || topic == "design" || topic == "research" || topic == "incidents" {
		return hyDrRefuse("topic %s does not hold knowledge entries", topic)
	}
	if errors.Is(last, fs.ErrNotExist) {
		return fmt.Errorf("%s/: topic is not defined: %w", topic, ErrNoSuchNote)
	}
	return last
}

// DropWith hides (op "drop") or brings back (op "restore") one knowledge entry
// under the brain lock, if expect is the entry's current rev. It writes one
// audit line with the status "dropped" or "restored", the entry's newest version
// and hash, the origin and the reason, and nothing else: the note and its
// versions stay as they are. Errors: ErrNoSuchNote, ErrConflict (a
// *ConflictError carrying the current rev) and ErrDropRefused (a *RefusedError
// carrying the reason); any other error is a store error. The index is not
// rebuilt here; Store.Drop does that after the lock.
func DropWith(root *os.Root, topic, name, op, expect string, origin Origin, reason string) (DropResult, error) {
	var status string
	switch op {
	case OpDrop:
		status = "dropped"
	case OpRestore:
		status = "restored"
	default:
		return DropResult{}, fmt.Errorf("unknown operation %q", op)
	}
	if !ValidName(topic) || !ValidName(name) {
		return DropResult{}, fmt.Errorf("topic and name must be plain names")
	}
	var res DropResult
	err := locked(root, func() error {
		if err := hyDrKnowledgeTopic(root, topic); err != nil {
			return err
		}
		if name == "TOPIC" {
			return hyDrRefuse("TOPIC is a topic definition, not an entry")
		}
		data, err := safefile.Read(root, filepath.Join("knowledge", topic, name+".md"))
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s/%s: %w", topic, name, ErrNoSuchNote)
		}
		if err != nil {
			return err
		}
		rev := NoteRev(data)
		if rev != expect {
			return &ConflictError{Rev: rev}
		}
		n, err := Parse(data)
		if err == nil {
			err = n.Validate()
		}
		if err != nil {
			return fmt.Errorf("%s/%s is not a valid entry: %w", topic, name, err)
		}
		if n.Name != name || n.Topic != topic {
			return fmt.Errorf("%s/%s does not match its location", topic, name)
		}
		states, err := hyDrStates(root)
		if err != nil {
			return err
		}
		last := states[topic+"/"+name]
		switch op {
		case OpDrop:
			switch {
			case n.Status != "accepted":
				return hyDrRefuse("note is %s, not accepted", n.Status)
			case n.SupersededBy != "":
				return hyDrRefuse("note is superseded by %s", n.SupersededBy)
			case last.status == "dropped":
				return hyDrRefuse("note is already dropped")
			}
		case OpRestore:
			if last.status != "dropped" {
				return hyDrRefuse("note is not dropped")
			}
		}
		version := 0
		numbers, err := versionNumbers(root, versionDir(topic, name))
		if err != nil {
			return err
		}
		if len(numbers) > 0 {
			version = numbers[len(numbers)-1]
		}
		// logAudit drops a line that is already in the log's tail, as a retry.
		// Each line for this entry is later than its last state line, so a drop,
		// a restore and a drop in one millisecond still write three lines.
		now := time.Now().UTC().Truncate(time.Millisecond)
		if !last.at.IsZero() && !now.After(last.at) {
			now = last.at.Add(time.Millisecond)
		}
		err = logAudit(root, Change{At: contract.FormatTime(now), Status: status, Topic: topic, Name: name,
			Version: version, Hash: auditHash(data), OriginKind: origin.Kind, OriginID: origin.ID, Reason: reason})
		if err != nil {
			return err
		}
		res = DropResult{Status: status, Version: version, Rev: rev}
		return nil
	})
	if err != nil {
		return DropResult{}, err
	}
	return res, nil
}

// Drop is DropWith on one level of the store, then the index rebuild, which runs
// under its own lock after the entry's lock is released. An error with a result
// that has a Status means the line is written and only the rebuild failed.
func (s Store) Drop(level, topic, name, op, expect string, origin Origin, reason string) (DropResult, error) {
	if s.Vault() {
		return s.vaultDrop(level, topic, name, op, expect, origin, reason)
	}
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return DropResult{}, fmt.Errorf("%s/%s: %w", topic, name, ErrNoSuchNote)
	}
	if err != nil {
		return DropResult{}, err
	}
	res, err := DropWith(root, topic, name, op, expect, origin, reason)
	root.Close()
	if err != nil {
		return DropResult{}, err
	}
	idx := s
	idx.ReadLevel = level
	if _, err := idx.RebuildIndex(); err != nil {
		return res, fmt.Errorf("%s %s/%s, but rebuilding the index failed: %w", res.Status, topic, name, err)
	}
	return res, nil
}
