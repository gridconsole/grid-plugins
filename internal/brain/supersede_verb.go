package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SupersedeResult is what Supersede did. Old and New are the note ids, Versions
// the newest stored version of each file, and Wrote is true when a file or an
// audit line was written, false when the pair was already in place.
type SupersedeResult struct {
	Old      string `json:"old"`
	New      string `json:"new"`
	Versions struct {
		Old int `json:"old"`
		New int `json:"new"`
	} `json:"versions"`
	Wrote bool `json:"wrote"`
	// Proposed lists the drafts in Proposals/ a vault write made instead of a change
	// that could not be redone on a person's edit.
	Proposed []string `json:"proposed,omitempty"`
}

// Supersede marks the accepted note oldID as replaced by the accepted note
// newID after the fact: the old note gets superseded_by (through markSuperseded,
// the helper accept uses) and the new note gets a link {to: oldID, rel:
// supersedes, why: reason} unless it has one already. Each changed file gets a
// new version and one audit line, the new note's first, as a decision orders
// them, and the index is rebuilt afterwards. Nothing is deleted.
//
// An id is brain:<level>/<name>, or a bare name at the store's default level:
// the read level, else project when the store has one, else home. Both notes
// must sit at one level. A reason of 8 to 160 characters on one line, more than
// one word, is required. The pair is refused, with nothing written, when a note
// is not an accepted note, the two are the same note, the new note is itself
// superseded, or the old note is superseded by another note. Repeating a
// completed call writes nothing and reports Wrote false.
func (s Store) Supersede(oldID, newID, reason string, origin Origin) (SupersedeResult, error) {
	if s.Vault() {
		return s.vaultSupersede(oldID, newID, reason, origin)
	}
	oldLevel, oldName, err := s.noteRef(oldID)
	if err != nil {
		return SupersedeResult{}, err
	}
	newLevel, newName, err := s.noteRef(newID)
	if err != nil {
		return SupersedeResult{}, err
	}
	if oldLevel != newLevel {
		return SupersedeResult{}, fmt.Errorf("brain:%s/%s and brain:%s/%s are at different levels; a note can only be superseded by a note at its own level", oldLevel, oldName, newLevel, newName)
	}
	level := oldLevel
	oldRef, newRef := "brain:"+level+"/"+oldName, "brain:"+level+"/"+newName
	reason = strings.TrimSpace(reason)
	if !line(reason) || size(reason) < 8 || size(reason) > 160 || len(strings.Fields(reason)) < 2 {
		return SupersedeResult{}, fmt.Errorf("reason must say why in 8 to 160 characters on one line, in more than one word")
	}
	if oldName == newName {
		return SupersedeResult{}, fmt.Errorf("%s cannot supersede itself", oldRef)
	}
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return SupersedeResult{}, fmt.Errorf("accepted note %q not found", oldRef)
	}
	if err != nil {
		return SupersedeResult{}, err
	}
	defer root.Close()
	res := SupersedeResult{Old: oldRef, New: newRef}
	err = locked(root, func() error {
		notes, _, err := scan(root, level)
		if err != nil {
			return err
		}
		old, ok := findNote(notes, oldName)
		if !ok {
			return fmt.Errorf("accepted note %q not found", oldRef)
		}
		next, ok := findNote(notes, newName)
		if !ok {
			return fmt.Errorf("accepted note %q not found", newRef)
		}
		if next.SupersededBy != "" {
			return fmt.Errorf("%s is itself superseded by %s; supersede by that note instead", newRef, next.SupersededBy)
		}
		if old.SupersededBy != "" && old.SupersededBy != newRef {
			return fmt.Errorf("%s is already superseded by %s", oldRef, old.SupersededBy)
		}
		linked := slices.ContainsFunc(next.Links, func(l Link) bool {
			to, _, _ := strings.Cut(l.To, "#")
			return l.Rel == "supersedes" && to == oldRef
		})
		if linked && old.SupersededBy == newRef {
			return setSupersedeVersions(root, &res, old, next)
		}
		at := s.stamp()
		// The new note's link first and the old note's mark second, so a stop
		// between them leaves a link to a live note and a retry finishes the mark.
		if !linked {
			link := Link{To: oldRef, Rel: "supersedes", Summary: old.Summary, Why: reason}
			if err := link.Validate(); err != nil {
				return fmt.Errorf("reason does not make a supersedes link: %w", err)
			}
			edited := next
			edited.Links = append(slices.Clone(next.Links), link)
			edited.Updated = at
			if hasSecret(edited) {
				return fmt.Errorf("reason holds secret-shaped content")
			}
			data, err := Encode(edited)
			if err != nil {
				return err
			}
			v, hash, err := recordVersion(root, edited.Topic, edited.Name, data)
			if err != nil {
				return err
			}
			if err := atomicWrite(root, filepath.Join("knowledge", edited.Topic, edited.Name+".md"), data); err != nil {
				return err
			}
			err = logAudit(root, Change{At: at, Status: "linked", Topic: edited.Topic, Name: edited.Name, Version: v, Hash: hash,
				OriginKind: origin.Kind, OriginID: origin.ID, Reason: "supersedes " + oldRef + ": " + reason})
			if err != nil {
				return err
			}
			res.Wrote = true
		}
		if old.SupersededBy == "" {
			by := Note{Level: level, Name: newName, Updated: at, Origin: origin}
			if err := markSuperseded(root, old, by, reason); err != nil {
				return err
			}
			res.Wrote = true
		}
		return setSupersedeVersions(root, &res, old, next)
	})
	if err != nil {
		return SupersedeResult{}, err
	}
	if res.Wrote {
		// The index is rebuilt under its own lock, after this one is released.
		idx := s
		idx.ReadLevel = level
		if _, err := idx.RebuildIndex(); err != nil {
			return res, fmt.Errorf("%s is superseded by %s, but %w: %w", oldRef, newRef, ErrIndexRebuild, err)
		}
	}
	return res, nil
}

// noteRef splits a note id into its level and name; a bare name takes the
// store's default level.
func (s Store) noteRef(id string) (level, name string, err error) {
	if level, name, err = splitID(id); err != nil || level != "" {
		return level, name, err
	}
	switch {
	case s.ReadLevel != "":
		level = s.ReadLevel
	case s.Project != "":
		level = "project"
	default:
		level = "home"
	}
	return level, name, nil
}

func findNote(notes []Note, name string) (Note, bool) {
	i := slices.IndexFunc(notes, func(n Note) bool { return n.Name == name })
	if i < 0 {
		return Note{}, false
	}
	return notes[i], true
}

// setSupersedeVersions reports the newest stored version of both notes.
func setSupersedeVersions(root *os.Root, res *SupersedeResult, old, next Note) error {
	for _, c := range []struct {
		n   Note
		dst *int
	}{{old, &res.Versions.Old}, {next, &res.Versions.New}} {
		numbers, err := versionNumbers(root, versionDir(c.n.Topic, c.n.Name))
		if err != nil {
			return err
		}
		if len(numbers) > 0 {
			*c.dst = numbers[len(numbers)-1]
		}
	}
	return nil
}
