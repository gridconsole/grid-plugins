package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// AcceptHomeInProject files a waiting home proposal in the store's project: it
// moves the proposal there, accepts it at project level, and moves it back home
// when the accept fails, so the curator's brain:home id keeps working.
func (s Store) AcceptHomeInProject(name string) (Entry, error) {
	// existed is read under the project lock by the move's own check.
	var existed bool
	if err := s.moveProposal(name, "home", "project", true, &existed); err != nil {
		var g gateRefusal
		var done alreadyAccepted
		switch {
		case errors.As(err, &done):
			// A curator retry: as on base, accepting again changes nothing.
			return s.Accept("home", name)
		case errors.As(err, &g):
			// Record the rejection at home only when the home gate refuses it too;
			// a refusal that is only the project's leaves the proposal untouched.
			if s.homeGateRefuses(name) {
				return s.Accept("home", name)
			}
			return Entry{}, fmt.Errorf("cannot file %q in project %q, nothing was changed: %s", name, s.Project, g.reason)
		}
		if errors.Is(err, fs.ErrNotExist) {
			// A repeated identical accept: the project already holds it as accepted.
			if p, perr := s.readProject(name); perr == nil && p.Status == "accepted" {
				// markSuperseded is safe to run again, so a stop before the first run is retried.
				return s.entry(p, false), s.supersedeHome(p)
			}
		}
		return Entry{}, err
	}
	e, err := s.Accept("project", name)
	if err == nil {
		return e, s.supersedeHome(e.Note)
	}
	if p, perr := s.readProject(name); perr == nil && p.Status == "proposed" {
		if !existed {
			s.removeProjectNote(p)
		}
		if berr := s.moveProposal(name, "project", "home", false, nil); berr != nil {
			return e, fmt.Errorf("%w (and moving the proposal back home failed: %v)", err, berr)
		}
	}
	return e, err
}

func (s Store) homeGateRefuses(name string) bool {
	root, err := s.scope("home", false)
	if err != nil {
		return false
	}
	defer root.Close()
	n, _, err := readProposalBytes(root, "home", name)
	return err == nil && gate(root, n) != ""
}

// removeProjectNote drops a note a failed accept published, so a later home
// accept cannot file it at both levels.
func (s Store) removeProjectNote(n Note) {
	root, err := s.scope("project", false)
	if err != nil {
		return
	}
	defer root.Close()
	path := filepath.Join("knowledge", n.Topic, n.Name+".md")
	_ = locked(root, func() error {
		// Only the note this attempt wrote: one a concurrent decide filed with
		// other content stays.
		b, err := safefile.Read(root, path)
		if err != nil {
			return err
		}
		old, err := Parse(b)
		a := n
		a.Status, a.Reason, a.Updated = "accepted", "", old.Updated
		if err != nil || !sameFiled(old, a) {
			return nil
		}
		return root.Remove(path)
	})
}

// supersedeHome marks the home notes that n supersedes as replaced by n's project id.
func (s Store) supersedeHome(n Note) error {
	var names []string
	for _, l := range n.Links {
		to, _, _ := strings.Cut(l.To, "#")
		if name, ok := strings.CutPrefix(to, "brain:home/"); ok && l.Rel == "supersedes" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	root, err := s.scope("home", false)
	if err != nil {
		return err
	}
	defer root.Close()
	return locked(root, func() error {
		notes, _, err := scan(root, "home")
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
	})
}

func (s Store) readProject(name string) (Note, error) {
	root, err := s.scope("project", false)
	if err != nil {
		return Note{}, err
	}
	defer root.Close()
	n, _, err := readProposalBytes(root, "project", name)
	return n, err
}

// moveProposal moves a proposal between the home and project stores under both
// locks, project first, the order WithKeeperGate takes them. With check set it
// refuses unless the proposal is waiting and the accept would pass its gates.
func (s Store) moveProposal(name, fromLevel, toLevel string, check bool, existed *bool) error {
	if !ValidName(name) {
		return fmt.Errorf("proposal id must be a plain name")
	}
	project, err := s.scope("project", true)
	if err != nil {
		return err
	}
	defer project.Close()
	home, err := s.scope("home", true)
	if err != nil {
		return err
	}
	defer home.Close()
	from, to := home, project
	if fromLevel == "project" {
		from, to = project, home
	}
	return locked(project, func() error {
		return locked(home, func() error {
			n, _, err := readProposalBytes(from, fromLevel, name)
			if err != nil {
				return err
			}
			dst := filepath.Join("brain-proposals", name+".md")
			if _, err := to.Stat(dst); err == nil {
				return fmt.Errorf("%s already holds a proposal %q", toLevel, name)
			}
			n.Level = toLevel
			if check {
				if n.Status == "accepted" {
					return alreadyAccepted{name}
				}
				if n.Status != "proposed" {
					return fmt.Errorf("proposal %q is %s and cannot be moved", name, n.Status)
				}
				if err := acceptable(to, toLevel, n, existed); err != nil {
					return err
				}
			}
			b, err := Encode(n)
			if err != nil {
				return err
			}
			if err := atomicWrite(to, dst, b); err != nil {
				return err
			}
			return from.Remove(dst)
		})
	})
}

// gateRefusal is a content-gate refusal, which accept records as a rejection.
type gateRefusal struct{ reason string }

func (g gateRefusal) Error() string { return "proposal rejected: " + g.reason }

// alreadyAccepted marks a home proposal that is already filed.
type alreadyAccepted struct{ name string }

func (a alreadyAccepted) Error() string {
	return fmt.Sprintf("proposal %q is already accepted", a.name)
}

// acceptable runs the checks decide runs before it publishes n at level.
func acceptable(root *os.Root, level string, n Note, existed *bool) error {
	if reason := gate(root, n); reason != "" {
		return gateRefusal{reason}
	}
	entries, _, err := scan(root, level)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name == n.Name && e.Topic != n.Topic {
			return fmt.Errorf("note name %q already belongs to topic %s", n.Name, e.Topic)
		}
	}
	path := filepath.Join("knowledge", n.Topic, n.Name+".md")
	before, err := safefile.Read(root, path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if existed != nil {
		*existed = true
	}
	old, err := Parse(before)
	a := n
	a.Status, a.Reason, a.Updated = "accepted", "", old.Updated
	if err != nil || !sameFiled(old, a) {
		return fmt.Errorf("note %q already exists with different content", n.Name)
	}
	return nil
}
