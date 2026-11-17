package brain

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Store has two independent namespaces. Project is a folder name below
// GridHome/projects; an empty Project opens only personal knowledge. Reading
// both never copies personal data into the project's generated files.
type Store struct {
	GridHome string
	Project  string
	// ReadLevel optionally restricts queries and generated indexes to one level.
	ReadLevel string
	Now       func() time.Time
	// BrainDir is the brain folder from the Grid config (the workspace's "brain"
	// key). Empty means the key is absent and the brain is read where it is
	// today, under GridHome/knowledge. See Root and vault.go.
	BrainDir string
	// Author is the configured Grid name: the author the listing gives to
	// bytes no Grid record explains. Empty when the config names none.
	Author string
}

// Root is the folder entry paths are relative to and where the vault reader
// walks: the brain folder when the config key is set, else GridHome, where the
// old knowledge/ layout lives. The write paths still use GridHome.
func (s Store) Root() string {
	if s.BrainDir != "" {
		return s.BrainDir
	}
	return s.GridHome
}

// Vault reports that the brain folder key is set, so the reader walks the
// folder as an Obsidian vault instead of the old knowledge/ layout.
func (s Store) Vault() bool { return s.BrainDir != "" }

type Entry struct {
	ID   string `json:"id"`
	Path string `json:"path"` // relative to Root (the Grid home when no brain folder is set), portable across machines
	Note Note   `json:"note"`
	// Dropped is set when a read finds the entry hidden by grid brain drop; it
	// stays out of the JSON when false.
	Dropped bool `json:"dropped,omitempty"`
	// row is the vault reader's view of the file (aliases, tags, edges).
	row *ListRow
}

func (s Store) stamp() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return contract.FormatTime(now)
}

func (s Store) levels() []string {
	if s.ReadLevel != "" {
		return []string{s.ReadLevel}
	}
	if s.Project != "" {
		return []string{"project", "home"}
	}
	return []string{"home"}
}

func (s Store) prefix(level string) (string, error) {
	if level == "home" {
		return ".", nil
	}
	if level != "project" || !ValidName(s.Project) {
		return "", fmt.Errorf("project level needs a valid project name")
	}
	return filepath.Join("projects", s.Project), nil
}

func (s Store) scope(level string, create bool) (*os.Root, error) {
	prefix, err := s.prefix(level)
	if err != nil {
		return nil, err
	}
	if s.GridHome == "" {
		return nil, fmt.Errorf("cannot find the Grid home; set GRID_HOME")
	}
	if create {
		if err := os.MkdirAll(s.GridHome, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := os.OpenRoot(s.GridHome)
	if err != nil {
		return nil, err
	}
	if prefix == "." {
		return root, nil
	}
	defer root.Close()
	if err := safefile.NoSymlinks(root, prefix); err != nil {
		return nil, err
	}
	if create {
		if err := root.MkdirAll(prefix, 0o700); err != nil {
			return nil, err
		}
	}
	return root.OpenRoot(prefix)
}

func randomID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// SessionOrigin creates an author stamp outside the supplied note text: the
// CLI session that proposes the note. Imported text sets the "import" kind.
func SessionOrigin() (Origin, error) {
	id, err := randomID()
	return Origin{Kind: "session", ID: id}, err
}

func atomicWrite(root *os.Root, path string, data []byte) error {
	if err := safefile.NoSymlinks(root, path); err != nil {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), ".brain-"+id+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(tmp, path)
}

// locked runs fn while holding an exclusive advisory lock on a file that stays
// in place. A second writer waits its turn. The kernel drops the lock when the
// holder exits, so a killed writer leaves nothing to clean up; removing the file
// would let two writers lock different inodes.
func locked(root *os.Root, fn func() error) error {
	if err := safefile.NoSymlinks(root, lockName); err != nil {
		return err
	}
	f, err := openLock(root)
	if err != nil {
		return err
	}
	defer f.Close() // closing releases the lock
	if err := fslock.LockExclusive(f); err != nil {
		return fmt.Errorf("brain write: %w", err)
	}
	return fn()
}

// openLock creates the lock file apart from opening it, because concurrent
// O_CREATE opens of a missing file through os.Root can fail with ENOENT on
// Darwin. The opened file must still be the regular file at the lock path.
func openLock(root *os.Root) (*os.File, error) {
	var err error
	for range 20 {
		c, cerr := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if cerr == nil {
			c.Close()
		} else if !errors.Is(cerr, fs.ErrExist) && !errors.Is(cerr, fs.ErrNotExist) {
			return nil, cerr
		}
		var f *os.File
		f, err = root.OpenFile(lockName, os.O_RDWR, 0)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := checkLock(root, f); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}
	return nil, err
}

func checkLock(root *os.Root, f *os.File) error {
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	at, err := root.Lstat(lockName)
	if err != nil || !at.Mode().IsRegular() || !os.SameFile(opened, at) {
		return fmt.Errorf("brain lock must be a regular file")
	}
	return nil
}

func (s Store) entry(n Note, proposal bool) Entry {
	prefix, _ := s.prefix(n.Level)
	path := filepath.Join("knowledge", n.Topic, n.Name+".md")
	if proposal {
		path = filepath.Join("brain-proposals", n.Name+".md")
	}
	return Entry{ID: "brain:" + n.Level + "/" + n.Name, Path: filepath.ToSlash(filepath.Join(prefix, path)), Note: n}
}

// Propose stamps provenance and lifecycle fields rather than trusting incoming
// front matter. Secret-shaped content is never persisted, even as a proposal.
func (s Store) Propose(n Note, origin Origin) (Entry, error) {
	n, err := s.proposalNote(n, origin)
	if err != nil {
		return Entry{}, err
	}
	root, err := s.scope(n.Level, true)
	if err != nil {
		return Entry{}, err
	}
	defer root.Close()
	if _, err := topicAt(root, n.Topic, n.Level); err != nil {
		return Entry{}, err
	}
	secret := hasSecret(n)
	if secret {
		n = redacted(n, "Secret-shaped content was redacted.")
	}
	err = locked(root, func() error {
		path := filepath.Join("brain-proposals", n.Name+".md")
		if _, err := safefile.Read(root, path); !errors.Is(err, fs.ErrNotExist) {
			if err != nil {
				return err
			}
			return fmt.Errorf("proposal %q already exists", n.Name)
		}
		b, err := Encode(n)
		if err != nil {
			return err
		}
		return atomicWrite(root, path, b)
	})
	if err != nil {
		return Entry{}, err
	}
	if secret {
		return s.entry(n, true), fmt.Errorf("proposal rejected: secret-shaped content was redacted")
	}
	return s.entry(n, true), nil
}

// proposalNote stamps the lifecycle fields and validates the note; it is the
// part of Propose that does not depend on where the proposal is written.
func (s Store) proposalNote(n Note, origin Origin) (Note, error) {
	n.Origin, n.Status, n.Reason, n.Updated = origin, "proposed", "", s.stamp()
	n.SupersededBy = ""
	if n.Type == "" {
		n.Type = "reference"
	}
	if n.Links == nil {
		n.Links = []Link{}
	}
	return n, n.Validate()
}

// ProposeTask writes the proposal to <task folder>/brain-proposals/<name>.md,
// where DecideTask and deliver read it, instead of under the
// brain level. The note keeps its level, which is where a decision files it.
// history appends the task's "brain proposed" line after the file is written
// and the task's decide lock is released; if the line cannot be written the
// proposal is removed again, so a failure leaves nothing behind. A
// secret-shaped note is stored redacted and refused, with no history line.
func (s Store) ProposeTask(t TaskRef, n Note, origin Origin, history func(string) error) (Entry, error) {
	n, err := s.proposalNote(n, origin)
	if err != nil {
		return Entry{}, err
	}
	levelRoot, err := s.scope(n.Level, false)
	if err != nil {
		return Entry{}, err
	}
	_, err = topicAt(levelRoot, n.Topic, n.Level)
	levelRoot.Close()
	if err != nil {
		return Entry{}, err
	}
	secret := hasSecret(n)
	if secret {
		n = redacted(n, "Secret-shaped content was redacted.")
	}
	path := filepath.Join("brain-proposals", n.Name+".md")
	tr, err := os.OpenRoot(t.Path)
	if err != nil {
		return Entry{}, err
	}
	defer tr.Close()
	err = func() error {
		if err := safefile.NoSymlinks(tr, ".brain-decide.lock"); err != nil {
			return err
		}
		lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".brain-decide.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err := fslock.LockExclusive(lock); err != nil {
			return err
		}
		if _, err := safefile.Read(tr, path); !errors.Is(err, fs.ErrNotExist) {
			if err != nil {
				return err
			}
			return fmt.Errorf("proposal %q already exists in task %s", n.Name, t.ID)
		}
		b, err := Encode(n)
		if err != nil {
			return err
		}
		return atomicWrite(tr, path, b)
	}()
	if err != nil {
		return Entry{}, err
	}
	entry := s.entry(n, true)
	entry.Path = "task:" + filepath.ToSlash(path)
	if secret {
		return entry, fmt.Errorf("proposal rejected: secret-shaped content was redacted")
	}
	if history != nil {
		if err := history("brain proposed · " + n.Name); err != nil {
			_ = tr.Remove(path)
			return Entry{}, err
		}
	}
	return entry, nil
}

func redacted(n Note, reason string) Note {
	return Note{Name: n.Name, Topic: n.Topic, Type: "reference", Level: n.Level,
		Status: "rejected", Updated: n.Updated, Origin: n.Origin, Source: "redacted",
		Description: "Rejected proposal; content redacted.", Summary: "Content redacted.",
		Reason: reason, Links: []Link{}, Body: "Content redacted.\n"}
}

func (s Store) Proposal(level, name string) (Entry, error) {
	if !ValidName(name) {
		return Entry{}, fmt.Errorf("proposal id must be a plain name")
	}
	root, err := s.scope(level, false)
	if err != nil {
		return Entry{}, err
	}
	defer root.Close()
	n, err := readProposal(root, level, name)
	return s.entry(n, true), err
}

func readProposal(root *os.Root, level, name string) (Note, error) {
	b, err := safefile.Read(root, filepath.Join("brain-proposals", name+".md"))
	if err != nil {
		return Note{}, err
	}
	n, err := Parse(b)
	if err != nil {
		return Note{}, err
	}
	if n.Name != name || n.Level != level {
		return Note{}, fmt.Errorf("proposal name or level does not match its location")
	}
	return n, nil
}

// Accept is a mechanical keeper gate. It never writes config topics, promotes
// guest/external text, or overwrites an existing fact. Repeating a completed
// accept is safe; a failed proposal-status write can be retried after the note
// write. A rejected proposal is terminal.
func (s Store) Accept(level, name string) (Entry, error) {
	return s.decide(level, name, "")
}

// Reject records a keeper decision without deleting the proposal.
func (s Store) Reject(level, name, reason string) (Entry, error) {
	if strings.TrimSpace(reason) == "" || !line(reason) || size(reason) > 300 {
		return Entry{}, fmt.Errorf("rejection needs a reason of 1 to 300 characters on one line")
	}
	return s.decide(level, name, reason)
}

func (s Store) decide(level, name, reject string) (Entry, error) {
	if !ValidName(name) {
		return Entry{}, fmt.Errorf("proposal id must be a plain name")
	}
	root, err := s.scope(level, false)
	if err != nil {
		return Entry{}, err
	}
	defer root.Close()
	var result Entry
	var refusal error
	err = locked(root, func() error {
		n, _, err := readProposalBytes(root, level, name)
		if err != nil {
			return err
		}
		if n.Status == "rejected" {
			return fmt.Errorf("proposal %q is already rejected", name)
		}
		if n.Status == "accepted" && reject != "" {
			return fmt.Errorf("accepted proposals cannot be rejected")
		}
		if reject == "" {
			reject = gate(root, n)
			if reject != "" {
				refusal = fmt.Errorf("proposal rejected: %s", reject)
			}
		}
		if reject != "" {
			n.Status, n.Reason, n.Updated = "rejected", reject, s.stamp()
			if hasSecret(n) {
				n = redacted(n, "Secret-shaped content was redacted.")
			}
			b, err := Encode(n)
			if err != nil {
				return err
			}
			if err := logDecision(root, n, 0, ""); err != nil {
				return err
			}
			if err := atomicWrite(root, filepath.Join("brain-proposals", name+".md"), b); err != nil {
				return err
			}
			result = s.entry(n, true)
			return nil
		}
		n.Status, n.Reason = "accepted", ""
		path := filepath.Join("knowledge", n.Topic, name+".md")
		// Names are unique within a level, including across topics.
		entries, _, err := scan(root, level)
		if err != nil {
			return err
		}
		for _, existing := range entries {
			if existing.Name == name && existing.Topic != n.Topic {
				return fmt.Errorf("note name %q already belongs to topic %s", name, existing.Topic)
			}
		}
		// The newest stored version must hold the published bytes before they
		// are logged. A retry of a completed accept finds them unchanged.
		var published []byte
		publish := false
		if before, err := safefile.Read(root, path); err == nil {
			old, err := Parse(before)
			// The note may have been published before a failed proposal-status
			// write. Preserve that publication time when recovering the write.
			n.Updated = old.Updated
			if err != nil || !reflect.DeepEqual(old, normalized(n)) {
				return fmt.Errorf("note %q already exists with different content", name)
			}
			published = before
		} else if errors.Is(err, fs.ErrNotExist) {
			n.Updated = s.stamp()
			published, err = Encode(n)
			if err != nil {
				return err
			}
			publish = true
		} else {
			return err
		}
		version, hash, err := recordVersion(root, n.Topic, name, published)
		if err != nil {
			return err
		}
		if publish {
			if err := atomicWrite(root, path, published); err != nil {
				return err
			}
		}
		b, err := Encode(n)
		if err != nil {
			return err
		}
		if err := logDecision(root, n, version, hash); err != nil {
			return err
		}
		// Before the proposal is marked accepted, so a stop here is retried.
		if err := supersede(root, level, n, n.Links); err != nil {
			return err
		}
		if err := atomicWrite(root, filepath.Join("brain-proposals", name+".md"), b); err != nil {
			return err
		}
		result = s.entry(n, false)
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, refusal
}

// logDecision records a keeper decision. Version and hash describe the bytes
// it published; a rejection publishes nothing and passes 0 and "".
func logDecision(root *os.Root, n Note, version int, hash string) error {
	return logAudit(root, Change{At: n.Updated, Status: n.Status, Topic: n.Topic, Name: n.Name,
		Version: version, Hash: hash, OriginKind: n.Origin.Kind, OriginID: n.Origin.ID, Reason: n.Reason})
}

// supersede marks the notes that the published note n replaces, under the level
// lock and after n is published and logged. A link with rel "supersedes" to
// brain:<level>/<name> (an #anchor is ignored) names an accepted note at this
// level; that note gets superseded_by brain:<level>/<n.Name> as a new version
// with an audit line, and no other field changes. A target that is missing, at
// the other level, not a note, n itself or already superseded is left as it is
// and the accept does not refuse for it. A superseded n takes no note over, so
// two notes never replace each other. Nothing is deleted. links are the links
// this decision adds: the proposal's, which a merge appends to its target.
func supersede(root *os.Root, level string, n Note, links []Link) error {
	if n.SupersededBy != "" {
		return nil
	}
	var names []string
	for _, l := range links {
		to, _, _ := strings.Cut(l.To, "#")
		name, ok := strings.CutPrefix(to, "brain:"+level+"/")
		if l.Rel == "supersedes" && ok && name != n.Name && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	notes, _, err := scan(root, level)
	if err != nil {
		return err
	}
	for _, old := range notes {
		if slices.Contains(names, old.Name) {
			if err := markSuperseded(root, old, n, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// markSuperseded writes old with superseded_by naming by: the version first,
// then the note, then the audit line, as a decision does. A note already marked
// by another note is not marked again. A note already marked by by is a retry:
// only the audit line may be missing, and logAudit drops a line already there.
// A non-empty why is added to the audit reason after "superseded by <id>"; the
// accept path passes none.
func markSuperseded(root *os.Root, old, by Note, why string) error {
	id := "brain:" + by.Level + "/" + by.Name
	if old.SupersededBy != "" && old.SupersededBy != id {
		return nil
	}
	path := filepath.Join("knowledge", old.Topic, old.Name+".md")
	var version int
	var hash string
	if old.SupersededBy == id {
		current, err := safefile.Read(root, path)
		if err != nil {
			return err
		}
		dir := versionDir(old.Topic, old.Name)
		numbers, err := versionNumbers(root, dir)
		if err != nil || len(numbers) == 0 {
			return err
		}
		newest := numbers[len(numbers)-1]
		stored, err := safefile.Read(root, filepath.Join(dir, versionName(newest)))
		if err != nil {
			return err
		}
		if !bytes.Equal(stored, current) {
			return nil // edited since; the mark was logged with the version it made
		}
		version, hash = newest, auditHash(current)
	} else {
		old.SupersededBy = id
		data, err := Encode(old)
		if err != nil {
			return err
		}
		if version, hash, err = recordVersion(root, old.Topic, old.Name, data); err != nil {
			return err
		}
		if err := atomicWrite(root, path, data); err != nil {
			return err
		}
	}
	reason := "superseded by " + id
	if why != "" {
		reason += ": " + why
	}
	return logAudit(root, Change{At: by.Updated, Status: "superseded", Topic: old.Topic, Name: old.Name,
		Version: version, Hash: hash, OriginKind: by.Origin.Kind, OriginID: by.Origin.ID, Reason: reason})
}

func normalized(n Note) Note {
	n.Body = strings.TrimRight(n.Body, "\n") + "\n"
	if n.Links == nil {
		n.Links = []Link{}
	}
	return n
}

func gate(root *os.Root, n Note) string {
	if hasSecret(n) {
		return "Secret-shaped content was redacted."
	}
	if err := n.Validate(); err != nil {
		return err.Error()
	}
	topic, err := topicAt(root, n.Topic, n.Level)
	if err != nil {
		return err.Error()
	}
	if topic.Kind != "knowledge" {
		return "Config topics must be changed through their own commands."
	}
	if n.Origin.Kind == "import" {
		return "Imported proposals are quarantined from accepted knowledge."
	}
	return ""
}
