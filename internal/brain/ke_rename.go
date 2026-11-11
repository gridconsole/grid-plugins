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

// ErrRenameNotVersioned means the entry cannot be renamed because it has no
// stored history to move: its topic holds settings, or the newest stored version
// is not the bytes on disk (a note placed by hand). It is the rename's own
// sentinel.
var ErrRenameNotVersioned = errors.New("not versioned")

// keRnWriteHook is nil in production. A test sets it to fail one relink: it is
// called with the note's path below the level ("knowledge/<topic>/<name>.md")
// before that note's version, file and audit line are written.
var keRnWriteHook func(rel string) error

// RenameResult is what RenameWith did. From and To are "<topic>/<name>"; Version
// and Rev are the renamed note's newest version and the rev of its new bytes.
// Linked lists the notes whose links were rewritten, Kept the notes whose
// relink failed and which stay as they were, and Skipped the dropped notes that
// link to the old name and are not rewritten.
type RenameResult struct {
	Status  string   `json:"status"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	Version int      `json:"version"`
	Rev     string   `json:"rev"`
	Linked  []string `json:"linked"`
	Kept    []string `json:"kept"`
	Skipped []string `json:"skipped"`
}

func keRnNotVersioned(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRenameNotVersioned, fmt.Sprintf(format, args...))
}

// keRnRecord is recordVersion with an undo: when the call stored a new copy, undo
// removes it again, so a failed write leaves no copy that the file does not hold.
func keRnRecord(root *os.Root, topic, name string, data []byte) (int, string, func(), error) {
	dir := versionDir(topic, name)
	numbers, err := versionNumbers(root, dir)
	if err != nil {
		return 0, "", nil, err
	}
	last := 0
	if len(numbers) > 0 {
		last = numbers[len(numbers)-1]
	}
	v, hash, err := recordVersion(root, topic, name, data)
	if err != nil {
		return 0, "", nil, err
	}
	undo := func() {}
	if v > last {
		undo = func() { _ = root.Remove(filepath.Join(dir, versionName(v))) }
	}
	return v, hash, undo, nil
}

// RenameWith renames one accepted, versioned knowledge note under the brain
// lock, if expect is its current rev, and relinks the other notes of the level
// that name it. Checks come first and write nothing: ErrNoSuchNote, a
// *ConflictError, ErrRenameNotVersioned and a *RefusedError. Then the new bytes
// are stored as a version of the new name, written as the new file, the old
// file is removed and one "renamed" line is logged; a failure before that line
// puts the old file back. From there the rename is applied: every other
// accepted note whose links or superseded_by name the old id gets the new id as
// a new version with a "linked" line. A note whose relink fails is left as it
// was and named in Kept; the call still returns a nil error. The old name's
// versions stay where they are. The index is not rebuilt here; Store.Rename
// does that after the lock. An error with a result that has a Status means the
// rename is applied and only the listing of the notes to relink failed.
func RenameWith(root *os.Root, level, topic, name, newName, expect string, origin Origin, reason string) (RenameResult, error) {
	if level != "home" && level != "project" {
		return RenameResult{}, fmt.Errorf("level must be home or project")
	}
	if !ValidName(topic) || !ValidName(name) {
		return RenameResult{}, fmt.Errorf("topic and name must be plain names")
	}
	var res RenameResult
	err := locked(root, func() error {
		var err error
		res, err = keRnApply(root, level, topic, name, newName, expect, origin, reason)
		return err
	})
	if err != nil && res.Status == "" {
		return RenameResult{}, err
	}
	return res, err
}

func keRnApply(root *os.Root, level, topic, name, newName, expect string, origin Origin, reason string) (RenameResult, error) {
	if topic == versionsTopic {
		return RenameResult{}, hyDrRefuse("topic %s does not hold knowledge entries", topic)
	}
	// A settings topic has no versions, at either level.
	for _, bt := range BuiltinTopics() {
		if bt.Name == topic && bt.Kind != "knowledge" {
			return RenameResult{}, keRnNotVersioned("topic %s holds settings, not knowledge entries", topic)
		}
	}
	t, err := topicAt(root, topic, level)
	if errors.Is(err, fs.ErrNotExist) {
		return RenameResult{}, fmt.Errorf("%s/: topic is not defined: %w", topic, ErrNoSuchNote)
	}
	if err != nil {
		return RenameResult{}, err
	}
	if t.Kind != "knowledge" {
		return RenameResult{}, keRnNotVersioned("topic %s holds settings, not knowledge entries", topic)
	}
	if name == "TOPIC" {
		return RenameResult{}, hyDrRefuse("TOPIC is a topic definition, not an entry")
	}
	oldRel := filepath.Join("knowledge", topic, name+".md")
	data, err := safefile.Read(root, oldRel)
	if errors.Is(err, fs.ErrNotExist) {
		return RenameResult{}, fmt.Errorf("%s/%s: %w", topic, name, ErrNoSuchNote)
	}
	if err != nil {
		return RenameResult{}, err
	}
	if rev := NoteRev(data); rev != expect {
		return RenameResult{}, &ConflictError{Rev: rev}
	}
	dir := versionDir(topic, name)
	numbers, err := versionNumbers(root, dir)
	if err != nil {
		return RenameResult{}, err
	}
	if len(numbers) == 0 {
		return RenameResult{}, keRnNotVersioned("%s/%s has no stored version", topic, name)
	}
	stored, err := safefile.Read(root, filepath.Join(dir, versionName(numbers[len(numbers)-1])))
	if err != nil {
		return RenameResult{}, err
	}
	if !bytes.Equal(stored, data) {
		return RenameResult{}, keRnNotVersioned("the newest version of %s/%s is not the note on disk", topic, name)
	}
	n, err := Parse(data)
	if err == nil {
		err = n.Validate()
	}
	if err != nil {
		return RenameResult{}, fmt.Errorf("%s/%s is not a valid entry: %w", topic, name, err)
	}
	if n.Name != name || n.Topic != topic || n.Level != level {
		return RenameResult{}, fmt.Errorf("%s/%s does not match its location", topic, name)
	}
	states, err := hyDrStates(root)
	if err != nil {
		return RenameResult{}, err
	}
	switch {
	case n.Status != "accepted":
		return RenameResult{}, hyDrRefuse("note is %s, not accepted", n.Status)
	case n.SupersededBy != "":
		return RenameResult{}, hyDrRefuse("note is superseded by %s", n.SupersededBy)
	case states[topic+"/"+name].status == "dropped":
		return RenameResult{}, hyDrRefuse("note is dropped")
	case n.Type == "workflow":
		return RenameResult{}, hyDrRefuse("a workflow note is named by its tasks and cannot be renamed")
	case !ValidName(newName):
		return RenameResult{}, hyDrRefuse("new name must be a plain name")
	case newName == "TOPIC":
		return RenameResult{}, hyDrRefuse("TOPIC is a topic definition, not an entry")
	case newName == name:
		return RenameResult{}, hyDrRefuse("new name is the current name")
	}
	newRel := filepath.Join("knowledge", topic, newName+".md")
	if _, err := root.Lstat(newRel); err == nil {
		return RenameResult{}, hyDrRefuse("note name %q already exists in topic %s", newName, topic)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return RenameResult{}, err
	}
	if other, err := topicHolding(root, topic, newName); err != nil {
		return RenameResult{}, err
	} else if other != "" {
		return RenameResult{}, hyDrRefuse("note name %q already belongs to topic %s", newName, other)
	}
	renamed := n
	renamed.Name = newName
	newData, err := Encode(renamed)
	if err == nil {
		err = renamed.Validate()
	}
	if err != nil {
		return RenameResult{}, fmt.Errorf("the renamed note is not valid: %w", err)
	}

	// Nothing has been written so far. The copy under the new name exists before
	// the file does, and the file before the log line, as a decision orders them.
	version, hash, undoVersion, err := keRnRecord(root, topic, newName, newData)
	if err != nil {
		return RenameResult{}, err
	}
	if err := atomicWrite(root, newRel, newData); err != nil {
		undoVersion()
		return RenameResult{}, err
	}
	if err := root.Remove(oldRel); err != nil {
		_ = root.Remove(newRel)
		undoVersion()
		return RenameResult{}, err
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	why := "from " + topic + "/" + name
	if r := strings.Join(strings.Fields(reason), " "); r != "" {
		why += ": " + r
	}
	err = logAudit(root, Change{At: contract.FormatTime(now), Status: "renamed", Topic: topic, Name: newName,
		Version: version, Hash: hash, OriginKind: origin.Kind, OriginID: origin.ID, Reason: why})
	if err != nil {
		// No line, so the rename is not applied: put the old file back.
		if rb := atomicWrite(root, oldRel, data); rb != nil {
			return RenameResult{}, errors.Join(err, fmt.Errorf("the old file could not be put back: %w", rb))
		}
		_ = root.Remove(newRel)
		undoVersion()
		return RenameResult{}, err
	}

	res := RenameResult{Status: "renamed", From: topic + "/" + name, To: topic + "/" + newName, Version: version,
		Rev: NoteRev(newData), Linked: []string{}, Kept: []string{}, Skipped: []string{}}
	notes, _, err := scan(root, level)
	if err != nil {
		return res, fmt.Errorf("renamed %s to %s, but the notes that link to it could not be listed: %w", res.From, res.To, err)
	}
	oldID, newID := "brain:"+level+"/"+name, "brain:"+level+"/"+newName
	for _, m := range notes {
		if m.Topic == topic && m.Name == newName {
			continue
		}
		links, changed := keRnLinks(m.Links, oldID, newID)
		superseded := m.SupersededBy == oldID
		if !changed && !superseded {
			continue
		}
		key := m.Topic + "/" + m.Name
		if states[key].status == "dropped" {
			res.Skipped = append(res.Skipped, key)
			continue
		}
		m.Links = links
		if superseded {
			m.SupersededBy = newID
		}
		if err := keRnRelink(root, m, newID, res.From, res.To, origin, now); err != nil {
			res.Kept = append(res.Kept, key)
			continue
		}
		res.Linked = append(res.Linked, key)
	}
	return res, nil
}

// keRnLinks returns links with every target oldID, with or without an anchor,
// moved to newID, the anchor kept. The input slice is not changed.
func keRnLinks(links []Link, oldID, newID string) ([]Link, bool) {
	out := make([]Link, len(links))
	copy(out, links)
	changed := false
	for i, l := range out {
		base, anchor, anchored := strings.Cut(l.To, "#")
		if base != oldID {
			continue
		}
		out[i].To = newID
		if anchored {
			out[i].To += "#" + anchor
		}
		changed = true
	}
	return out, changed
}

// keRnRelink writes the relinked note m as a new version, as its file and as a
// "linked" line. When any step fails the file and the version list are as they
// were before, and the error says which step.
func keRnRelink(root *os.Root, m Note, newID, from, to string, origin Origin, now time.Time) error {
	rel := filepath.Join("knowledge", m.Topic, m.Name+".md")
	if hook := keRnWriteHook; hook != nil {
		if err := hook(filepath.ToSlash(rel)); err != nil {
			return err
		}
	}
	before, err := safefile.Read(root, rel)
	if err != nil {
		return err
	}
	data, err := Encode(m)
	if err == nil {
		err = m.Validate()
	}
	if err != nil {
		return err
	}
	version, hash, undoVersion, err := keRnRecord(root, m.Topic, m.Name, data)
	if err != nil {
		return err
	}
	if err := atomicWrite(root, rel, data); err != nil {
		undoVersion()
		return err
	}
	err = logAudit(root, Change{At: contract.FormatTime(now), Status: "linked", Topic: m.Topic, Name: m.Name,
		Version: version, Hash: hash, OriginKind: origin.Kind, OriginID: origin.ID, Reason: "renamed " + from + " to " + to + ", now " + newID})
	if err != nil {
		if rb := atomicWrite(root, rel, before); rb != nil {
			return errors.Join(err, rb)
		}
		undoVersion()
		return err
	}
	return nil
}

// Rename is RenameWith on one level of the store, then the index rebuild, which
// runs under its own lock after the note's lock is released. An error with a
// result that has a Status means the rename is applied and something after it
// failed.
func (s Store) Rename(level, topic, name, newName, expect string, origin Origin, reason string) (RenameResult, error) {
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return RenameResult{}, fmt.Errorf("%s/%s: %w", topic, name, ErrNoSuchNote)
	}
	if err != nil {
		return RenameResult{}, err
	}
	res, err := RenameWith(root, level, topic, name, newName, expect, origin, reason)
	root.Close()
	if err != nil && res.Status == "" {
		return RenameResult{}, err
	}
	idx := s
	idx.ReadLevel = level
	if _, ierr := idx.RebuildIndex(); ierr != nil {
		return res, errors.Join(err, fmt.Errorf("%s %s to %s, but %w: %w", res.Status, res.From, res.To, ErrIndexRebuild, ierr))
	}
	return res, err
}
