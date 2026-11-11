package brain

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// ChangesReadAllowed and RevertAllowed are the replaceable host policies for
// listing the audit log and for putting an entry back, called with the host's
// GRID_ROLE. The host keys are brain_changes_read_roles and brain_revert_roles;
// the defaults allow the empty local role, owner and member. Configure them
// before serving concurrent CLI calls.
var (
	ChangesReadAllowed = func(role string) bool { return ownerOrMember(role) }
	RevertAllowed      = func(role string) bool { return ownerOrMember(role) }
)

func ownerOrMember(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "owner", "member":
		return true
	default:
		return false
	}
}

// RevertResult is what RevertTo did: the entry, the version that now holds its
// published bytes, and whether anything was written.
type RevertResult struct {
	Entry   Entry
	Version int
	// Wrote is true when the entry was published or logged, even if the newest
	// stored copy already held the bytes and no version was added.
	Wrote bool
}

// Revert is RevertTo without the Wrote report: the entry, the version that now
// holds the published bytes, and an error.
func (s Store) Revert(level, topic, name string, to int, origin Origin) (Entry, int, error) {
	res, err := s.RevertTo(level, topic, name, to, origin)
	return res.Entry, res.Version, err
}

// RevertTo publishes version to of one entry again as a new version, so history
// is only ever extended. When the published bytes already are version to (and
// the newest stored copy), nothing is written and Wrote is false, with the
// current version in Version. Wrote is decided under the level lock. An error
// after the publication (the index rebuild) still returns the result.
func (s Store) RevertTo(level, topic, name string, to int, origin Origin) (RevertResult, error) {
	return s.RevertWith(level, topic, name, to, origin, RevertOptions{})
}

// RevertOptions are the optional parts of a revert.
type RevertOptions struct {
	// Reason says why the entry is put back. The audit line reads
	// "revert to vN: <Reason>"; an empty or blank Reason leaves "revert to vN".
	// auditLine folds whitespace and turns "|" into "/" when it is written.
	Reason string
}

// revertReason is the reason logged for a revert to version to.
func (o RevertOptions) revertReason(to int) string {
	why := strings.Join(strings.Fields(o.Reason), " ")
	if why == "" {
		return fmt.Sprintf("revert to v%d", to)
	}
	return fmt.Sprintf("revert to v%d: %s", to, why)
}

// RevertWith is RevertTo with options. A no-op revert writes no audit line,
// whatever the reason.
func (s Store) RevertWith(level, topic, name string, to int, origin Origin, opts RevertOptions) (RevertResult, error) {
	if !ValidName(topic) || !ValidName(name) {
		return RevertResult{}, fmt.Errorf("topic and name must be plain names")
	}
	if to < 1 {
		return RevertResult{}, fmt.Errorf("version must be 1 or more")
	}
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return RevertResult{}, fmt.Errorf("%s/%s has no version %d", topic, name, to)
	}
	if err != nil {
		return RevertResult{}, err
	}
	defer root.Close()
	var res RevertResult
	err = locked(root, func() error {
		// Only a knowledge topic has versions; a planted copy must not reach a config topic.
		t, err := topicAt(root, topic, level)
		if err != nil {
			return err
		}
		if t.Kind != "knowledge" {
			return fmt.Errorf("config topics must be changed through their own commands")
		}
		dir := versionDir(topic, name)
		data, err := safefile.Read(root, filepath.Join(dir, versionName(to)))
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s/%s has no version %d", topic, name, to)
		}
		if err != nil {
			return err
		}
		n, err := Parse(data)
		if err == nil {
			err = n.Validate()
		}
		if err != nil {
			return fmt.Errorf("version %d of %s/%s is not a valid entry: %w", to, topic, name, err)
		}
		if n.Name != name || n.Topic != topic || n.Level != level {
			return fmt.Errorf("version %d of %s/%s does not match its location", to, topic, name)
		}
		// Names are unique within a level, including across topics. scan cannot
		// tell: it drops every copy of an ambiguous name, this entry's too.
		if other, err := topicHolding(root, topic, name); err != nil {
			return err
		} else if other != "" {
			return fmt.Errorf("note name %q already belongs to topic %s", name, other)
		}
		path := filepath.Join("knowledge", topic, name+".md")
		current, err := safefile.Read(root, path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		exists := err == nil
		numbers, err := versionNumbers(root, dir)
		if err != nil {
			return err
		}
		newest := numbers[len(numbers)-1] // version to exists, so the list is not empty
		if exists && bytes.Equal(current, data) {
			// Already published; it is current only if the newest copy holds it too.
			stored := data
			if newest != to {
				if stored, err = safefile.Read(root, filepath.Join(dir, versionName(newest))); err != nil {
					return err
				}
			}
			if bytes.Equal(stored, data) {
				res.Entry, res.Version = s.entry(n, false), newest
				return nil
			}
		}
		// Same order as a decision: the copy exists before the entry is
		// published, and the entry is published before it is logged.
		v, hash, err := recordVersion(root, topic, name, data)
		if err != nil {
			return err
		}
		if !exists || !bytes.Equal(current, data) {
			if err := atomicWrite(root, path, data); err != nil {
				return err
			}
		}
		err = logAudit(root, Change{At: s.stamp(), Status: "reverted", Topic: topic, Name: name,
			Version: v, Hash: hash, OriginKind: origin.Kind, OriginID: origin.ID, Reason: opts.revertReason(to)})
		if err != nil {
			return err
		}
		res.Entry, res.Version, res.Wrote = s.entry(n, false), v, true
		return nil
	})
	if err != nil {
		return RevertResult{}, err
	}
	if res.Wrote {
		// The index is rebuilt under its own lock, after this one is released.
		idx := s
		idx.ReadLevel = level
		if _, err := idx.RebuildIndex(); err != nil {
			return res, fmt.Errorf("reverted to v%d as v%d, but rebuilding the index failed: %w", to, res.Version, err)
		}
	}
	return res, nil
}

// topicHolding names another topic that has a file for the entry name.
func topicHolding(root *os.Root, topic, name string) (string, error) {
	if err := safefile.NoSymlinks(root, "knowledge"); err != nil {
		return "", err
	}
	dirs, err := fs.ReadDir(root.FS(), "knowledge")
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == topic || d.Name() == versionsTopic {
			continue
		}
		_, err := root.Lstat(filepath.Join("knowledge", d.Name(), name+".md"))
		if err == nil {
			return d.Name(), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", nil
}
