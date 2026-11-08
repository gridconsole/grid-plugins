package where

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

func (r Resolver) guest() bool {
	return strings.EqualFold(strings.TrimSpace(r.Caller.Role), "guest")
}

// GuestProjectAllowed limits guests to the app's project-roles.json people list.
// Other roles retain their existing access; Personal has its own policy too.
// Missing, unreadable or invalid membership never grants a guest access.
func (r Resolver) GuestProjectAllowed(name string) bool {
	if !r.guest() {
		return true
	}
	member := strings.TrimSpace(r.Caller.Member)
	if member == "" || strings.EqualFold(name, Personal) || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return false
	}
	root, err := os.OpenRoot(r.GridHome)
	if err != nil {
		return false
	}
	defer root.Close()
	data, err := safefile.Read(root, filepath.Join("projects", name, "project-roles.json"))
	if err != nil || !utf8.Valid(data) {
		return false
	}
	var doc struct {
		People []struct {
			ID string `json:"id"`
		} `json:"people"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return false
	}
	for _, person := range doc.People {
		if strings.EqualFold(person.ID, member) {
			return true
		}
	}
	return false
}

// Guest task roots cannot alias another project's data through a symlink.
func (r Resolver) guestTaskPathAllowed(path string) bool {
	if !r.guest() {
		return true
	}
	rel, err := filepath.Rel(gridhome.Real(r.GridHome), path)
	if err != nil {
		return false
	}
	root, err := os.OpenRoot(r.GridHome)
	if err != nil {
		return false
	}
	defer root.Close()
	return safefile.NoSymlinks(root, rel) == nil
}

// Projects reads registered projects within the guest's scope. Filtering before
// reading project.md keeps errors in an uninvited project private as well.
// Non-guests keep the existing registry loader and Personal policy.
func (r Resolver) Projects() ([]project.Record, error) {
	if !r.guest() {
		return project.Load(r.GridHome)
	}
	entries, err := os.ReadDir(filepath.Join(r.GridHome, "projects"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.GridHome)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var records []project.Record
	for _, e := range entries {
		if !e.IsDir() || !r.GuestProjectAllowed(e.Name()) {
			continue
		}
		data, err := safefile.Read(root, filepath.Join("projects", e.Name(), "project.md"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read project record")
		}
		rec, err := project.Parse(e.Name(), string(data))
		if err != nil {
			return nil, fmt.Errorf("cannot parse project record")
		}
		records = append(records, rec)
	}
	return records, nil
}
