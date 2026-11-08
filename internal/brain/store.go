package brain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// Store has two independent namespaces. Project is a folder name below
// GridHome/projects; an empty Project opens only personal knowledge. Reading
// both never copies personal data into the project's generated files.
type Store struct {
	GridHome string
	Project  string
	Now      func() time.Time
}

type Entry struct {
	ID   string `json:"id"`
	Path string `json:"path"` // relative to the Grid home, portable across machines
	Note Note   `json:"note"`
}

func (s Store) stamp() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return now.UTC().Format(time.RFC3339)
}

func (s Store) levels() []string {
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
	if err := noSymlinks(root, prefix); err != nil {
		return nil, err
	}
	if create {
		if err := root.MkdirAll(prefix, 0o700); err != nil {
			return nil, err
		}
	}
	return root.OpenRoot(prefix)
}

// Reject symlink aliases as well as escapes: a home knowledge link into a
// project would otherwise mix the private and shared namespaces.
func noSymlinks(root *os.Root, path string) error {
	if !filepath.IsLocal(path) {
		return fmt.Errorf("brain path must stay within its level")
	}
	prefix := ""
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)
		info, err := root.Lstat(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("brain paths cannot be symbolic links")
		}
	}
	return nil
}

func read(root *os.Root, path string) ([]byte, error) {
	if err := noSymlinks(root, path); err != nil {
		return nil, err
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("brain files must be regular files of at most 1 MiB")
	}
	return root.ReadFile(path)
}

func randomID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// SessionOrigin creates a provenance stamp outside the supplied note text. The
// CLI claims only the agent tier; authenticated team/guest tiers belong to the
// server integration, and imported text must explicitly use external.
func SessionOrigin() (Origin, error) {
	id, err := randomID()
	return Origin{Kind: "session", ID: id, Tier: "agent"}, err
}

func atomicWrite(root *os.Root, path string, data []byte) error {
	if err := noSymlinks(root, path); err != nil {
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

func locked(root *os.Root, fn func() error) error {
	f, err := root.OpenFile(".brain.lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("brain is locked; retry after the writer finishes (remove a stale .brain.lock only when no writer is running)")
	}
	if err != nil {
		return err
	}
	defer root.Remove(".brain.lock")
	if err := f.Close(); err != nil {
		return err
	}
	return fn()
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
	n.Origin, n.Status, n.Reason, n.Updated = origin, "proposed", "", s.stamp()
	n.SupersededBy = ""
	if n.Type == "" {
		n.Type = "reference"
	}
	if n.Links == nil {
		n.Links = []Link{}
	}
	if err := n.Validate(); err != nil {
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
		if _, err := read(root, path); !errors.Is(err, fs.ErrNotExist) {
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
	b, err := read(root, filepath.Join("brain-proposals", name+".md"))
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
		n, err := readProposal(root, level, name)
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
		if before, err := read(root, path); err == nil {
			old, err := Parse(before)
			if err != nil || !reflect.DeepEqual(old, normalized(n)) {
				return fmt.Errorf("note %q already exists with different content", name)
			}
		} else if errors.Is(err, fs.ErrNotExist) {
			b, err := Encode(n)
			if err != nil {
				return err
			}
			if err := atomicWrite(root, path, b); err != nil {
				return err
			}
		} else {
			return err
		}
		b, err := Encode(n)
		if err != nil {
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
	if n.Origin.Tier == "external" || n.Origin.Tier == "person-guest" {
		return "Guest and external proposals are quarantined from accepted knowledge."
	}
	return ""
}
