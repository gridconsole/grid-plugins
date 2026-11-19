package brain

import (
	"fmt"
	"path/filepath"
)

// MoveProposalToProject moves a waiting home proposal into the store's project
// and sets its level, so accepting it files the note in that project.
func (s Store) MoveProposalToProject(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("proposal id must be a plain name")
	}
	from, err := s.scope("home", false)
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := s.scope("project", true)
	if err != nil {
		return err
	}
	defer to.Close()
	return locked(from, func() error {
		return locked(to, func() error {
			n, _, err := readProposalBytes(from, "home", name)
			if err != nil {
				return err
			}
			dst := filepath.Join("brain-proposals", name+".md")
			if _, err := to.Stat(dst); err == nil {
				return fmt.Errorf("project %q already holds a proposal %q", s.Project, name)
			}
			n.Level = "project"
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
