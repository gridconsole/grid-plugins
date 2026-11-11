package brain

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

var (
	// ErrNotVersioned means the target is not a versioned knowledge note: its
	// topic is not a knowledge topic, or the file on disk has no stored copy of its
	// exact bytes (a hand-placed file). Nothing is written.
	ErrNotVersioned = errors.New("not versioned")
	// ErrWorkflowNote means the new text or the note on disk is `type: workflow`
	// and the caller may not write workflow notes.
	ErrWorkflowNote = errors.New("workflow note")
	// ErrInvalidNote means the bytes are not a note scan would list at the place
	// they are written to: a plain refusal before the lock, for the caller to fix.
	ErrInvalidNote = errors.New("invalid note")
	// ErrLinkedTarget means the topic folder or the note file is a link or not a
	// regular file.
	ErrLinkedTarget = errors.New("linked or non-regular target")
)

// keWrMaxNote is the size safefile.Read accepts. A larger note would make every
// scan of the level fail, so WriteWith refuses it up front.
const keWrMaxNote = 1 << 20

// WriteResult is what WriteWith did: the status (created or edited), the stored
// version number of the new bytes and the rev of those bytes.
type WriteResult struct {
	Status  string `json:"status"`
	Version int    `json:"version"`
	Rev     string `json:"rev"`
}

// IsWorkflowNote reports whether a note is a workflow note: `type: workflow`,
// compared without case or surrounding space so a spelling cannot slip past.
func IsWorkflowNote(n Note) bool {
	return strings.EqualFold(strings.TrimSpace(n.Type), "workflow")
}

func keWrInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidNote, fmt.Sprintf(format, args...))
}

// keWrRevForm reports whether expect is "none" or a rev as the app writes it.
func keWrRevForm(expect string) bool {
	if expect == "none" {
		return true
	}
	if len(expect) != 16 {
		return false
	}
	for _, c := range expect {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// keWrKnowledge is nil when topic is a knowledge topic at level. Any other
// answer of topicAt (a config topic, a topic with no definition, a reserved
// name) means the folder has no versions, so the note cannot be written.
func keWrKnowledge(root *os.Root, topic, level string) error {
	t, err := topicAt(root, topic, level)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) && !errors.Is(err, fs.ErrNotExist) {
			return err // a read error, not a verdict on the topic
		}
		return fmt.Errorf("%s: %w: %v", topic, ErrNotVersioned, err)
	}
	if t.Kind != "knowledge" {
		return fmt.Errorf("%s: %w: topic %s holds settings, not knowledge entries", topic, ErrNotVersioned, topic)
	}
	return nil
}

// keWrNoLink refuses a path with a link on it. A failure to look is a store
// error; only a link is ErrLinkedTarget.
func keWrNoLink(root *os.Root, path string) error {
	err := safefile.NoSymlinks(root, path)
	if err == nil {
		return nil
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return err
	}
	return fmt.Errorf("%s: %w", path, ErrLinkedTarget)
}

// WriteWith publishes the caller's bytes as the knowledge note topic/name of one
// level, under the brain lock. expect is the note's current rev (16 lower-case
// hex digits, NoteRev) or "none" for a note that must not exist yet.
//
// Before the lock it refuses bytes that scan would not list (ErrInvalidNote).
// Under the lock, in this order: no link on the topic folder or the note
// (ErrLinkedTarget); the topic is a knowledge topic and the note is not TOPIC;
// the rev (a *ConflictError carrying the current rev, or "none"); the current
// file has a stored copy of its exact bytes (ErrNotVersioned); the file on disk
// is not a workflow note unless allowWorkflow (ErrWorkflowNote); the name is not
// held by another topic of the level (a *RefusedError). Then it stores the new
// version, publishes the note atomically and appends one audit line with the
// status created or edited, the version, the hash, origin and reason. A refusal
// writes nothing. The index is not rebuilt here; Store.Write does that.
func WriteWith(root *os.Root, level, topic, name, expect string, data []byte, origin Origin, reason string, allowWorkflow bool) (WriteResult, error) {
	if !ValidName(topic) || !ValidName(name) {
		return WriteResult{}, fmt.Errorf("topic and name must be plain names")
	}
	if level != "home" && level != "project" {
		return WriteResult{}, fmt.Errorf("level must be home or project")
	}
	if !keWrRevForm(expect) {
		return WriteResult{}, fmt.Errorf("expect must be none or a rev of 16 lower-case hex digits")
	}
	if len(data) > keWrMaxNote {
		return WriteResult{}, keWrInvalid("a note is at most 1 MiB")
	}
	// A topic that is not knowledge is not versioned, whatever the bytes say.
	if err := keWrKnowledge(root, topic, level); err != nil {
		return WriteResult{}, err
	}
	n, err := Parse(data)
	if err == nil {
		err = n.Validate()
	}
	if err != nil {
		return WriteResult{}, keWrInvalid("%v", err)
	}
	if n.Topic != topic || n.Name != name || n.Level != level {
		return WriteResult{}, keWrInvalid("the note says %s/%s at %s level, not %s/%s at %s", n.Topic, n.Name, n.Level, topic, name, level)
	}
	if n.Status != "accepted" {
		return WriteResult{}, keWrInvalid("status must be accepted, not %s", n.Status)
	}
	if why := gate(root, n); why != "" {
		return WriteResult{}, keWrInvalid("%s", why)
	}
	if IsWorkflowNote(n) && !allowWorkflow {
		return WriteResult{}, ErrWorkflowNote
	}
	if origin.Kind == "" || !line(origin.ID) || strings.TrimSpace(origin.ID) == "" {
		return WriteResult{}, fmt.Errorf("the audit line needs an origin")
	}

	var res WriteResult
	err = locked(root, func() error {
		dirPath := filepath.Join("knowledge", topic)
		path := filepath.Join(dirPath, name+".md")
		if err := keWrNoLink(root, dirPath); err != nil {
			return err
		}
		if err := keWrNoLink(root, path); err != nil {
			return err
		}
		if err := keWrKnowledge(root, topic, level); err != nil {
			return err
		}
		if name == "TOPIC" {
			return hyDrRefuse("TOPIC is a topic definition, not an entry")
		}
		var current []byte
		exists := false
		info, err := root.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return err
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s: %w", path, ErrLinkedTarget)
		default:
			if current, err = safefile.Read(root, path); err != nil {
				return err
			}
			exists = true
		}
		switch {
		case !exists && expect != "none":
			return &ConflictError{Rev: "none"}
		case exists && expect == "none":
			return &ConflictError{Rev: NoteRev(current)}
		case exists && expect != NoteRev(current):
			return &ConflictError{Rev: NoteRev(current)}
		}
		dir := versionDir(topic, name)
		if exists {
			// The newest stored version must hold the bytes on disk, or replacing
			// them would lose them (the rule revert applies to a no-op).
			numbers, err := versionNumbers(root, dir)
			if err != nil {
				return err
			}
			if len(numbers) == 0 {
				return fmt.Errorf("%s/%s: %w: no stored version", topic, name, ErrNotVersioned)
			}
			stored, err := safefile.Read(root, filepath.Join(dir, versionName(numbers[len(numbers)-1])))
			if err != nil {
				return err
			}
			if !bytes.Equal(stored, current) {
				return fmt.Errorf("%s/%s: %w: the newest stored version differs from the note", topic, name, ErrNotVersioned)
			}
			if old, perr := Parse(current); perr == nil && IsWorkflowNote(old) && !allowWorkflow {
				return ErrWorkflowNote
			}
		}
		if other, err := topicHolding(root, topic, name); err != nil {
			return err
		} else if other != "" {
			return hyDrRefuse("note name %q already belongs to topic %s", name, other)
		}
		// Same order as a revert: the copy exists before the note is published,
		// and the note is published before it is logged.
		v, hash, err := recordVersion(root, topic, name, data)
		if err != nil {
			return err
		}
		if err := atomicWrite(root, path, data); err != nil {
			return err
		}
		status := "edited"
		if !exists {
			status = "created"
		}
		at := contract.FormatTime(time.Now().UTC().Truncate(time.Millisecond))
		err = logAudit(root, Change{At: at, Status: status, Topic: topic, Name: name,
			Version: v, Hash: hash, OriginKind: origin.Kind, OriginID: origin.ID, Reason: reason})
		if err != nil {
			return err
		}
		res = WriteResult{Status: status, Version: v, Rev: NoteRev(data)}
		return nil
	})
	if err != nil {
		return WriteResult{}, err
	}
	return res, nil
}

// Write is WriteWith on one level of the store, then the index rebuild, which
// runs under its own lock after the note's lock is released. An error with a
// result that has a Status means the note and its line are written and only the
// rebuild failed.
func (s Store) Write(level, topic, name, expect string, data []byte, origin Origin, reason string, allowWorkflow bool) (WriteResult, error) {
	if !ValidName(topic) || !ValidName(name) {
		return WriteResult{}, fmt.Errorf("topic and name must be plain names")
	}
	root, err := s.scope(level, true)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := WriteWith(root, level, topic, name, expect, data, origin, reason, allowWorkflow)
	root.Close()
	if err != nil {
		return WriteResult{}, err
	}
	idx := s
	idx.ReadLevel = level
	if _, err := idx.RebuildIndex(); err != nil {
		return res, fmt.Errorf("%s %s/%s, but rebuilding the index failed: %w", res.Status, topic, name, err)
	}
	return res, nil
}
