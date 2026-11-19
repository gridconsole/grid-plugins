package brain

import (
	"fmt"
	"os"
	"path/filepath"
)

// AcceptHomeInProject files a waiting home proposal in the store's project: it
// moves the proposal there, accepts it at project level, and moves it back home
// when the accept fails, so the curator's brain:home id keeps working.
func (s Store) AcceptHomeInProject(name string) (Entry, error) {
	if err := s.moveProposal(name, "home", "project", true); err != nil {
		return Entry{}, err
	}
	e, err := s.Accept("project", name)
	if err != nil {
		if p, perr := s.readProject(name); perr == nil && p.Status == "proposed" {
			if berr := s.moveProposal(name, "project", "home", false); berr != nil {
				return e, fmt.Errorf("%w (and moving the proposal back home failed: %v)", err, berr)
			}
		}
	}
	return e, err
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
func (s Store) moveProposal(name, fromLevel, toLevel string, check bool) error {
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
				if n.Status != "proposed" {
					return fmt.Errorf("proposal %q is %s and cannot be moved", name, n.Status)
				}
				if err := acceptable(to, toLevel, n); err != nil {
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

// acceptable runs the checks decide runs before it publishes n at level.
func acceptable(root *os.Root, level string, n Note) error {
	if reason := gate(root, n); reason != "" {
		return fmt.Errorf("proposal rejected: %s", reason)
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
	if before, err := os.ReadFile(filepath.Join(root.Name(), path)); err == nil {
		old, err := Parse(before)
		a := n
		a.Status, a.Reason, a.Updated = "accepted", "", old.Updated
		if err != nil || !sameFiled(old, a) {
			return fmt.Errorf("note %q already exists with different content", n.Name)
		}
	}
	return nil
}
