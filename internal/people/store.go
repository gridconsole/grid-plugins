// Package people reads the server-synced people config topic.
package people

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// Entry is the default executable schema; the owner may change the policy.
// Body is free-form context, never executable approval policy.
type Entry struct {
	Handle string       `json:"handle" yaml:"handle"`
	Name   string       `json:"name" yaml:"name"`
	Role   string       `json:"role" yaml:"role"`
	Links  []brain.Link `json:"links" yaml:"links"`
	Body   string       `json:"body" yaml:"-"`
}

type Problem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Store struct {
	GridHome string
	Project  string // empty reads home only
}

func (e Entry) Validate() error {
	if !brain.ValidName(e.Handle) || e.Handle == "TOPIC" {
		return fmt.Errorf("person handle must be a plain name")
	}
	if strings.TrimSpace(e.Name) == "" || strings.ContainsAny(e.Name, "\r\n\x00") {
		return fmt.Errorf("person name is required on one line")
	}
	switch e.Role {
	case "owner", "member", "guest":
	default:
		return fmt.Errorf("person role must be owner, member or guest")
	}
	// People link destinations are area identifiers, not necessarily brain node
	// addresses, so brain.Link.Validate's node-address restriction does not apply.
	for _, link := range e.Links {
		if strings.TrimSpace(link.To) == "" || strings.TrimSpace(link.Rel) == "" || strings.ContainsAny(link.To+link.Rel, "\r\n\x00") {
			return fmt.Errorf("person link needs a target and relation on one line")
		}
	}
	return nil
}

func decode(name string, data []byte) (Entry, error) {
	var e Entry
	body, err := brain.DecodeConfig(data, &e)
	if err != nil {
		return e, err
	}
	e.Body = body
	if e.Handle != name {
		return e, fmt.Errorf("person handle does not match its file")
	}
	if e.Links == nil {
		e.Links = []brain.Link{}
	}
	return e, e.Validate()
}

func (s Store) levels() []string {
	if s.Project != "" {
		return []string{"project", "home"}
	}
	return []string{"home"}
}

// Get returns the project record when present, including its decode error;
// an invalid project override must not revive a home approval grant.
// A missing handle returns an error wrapping fs.ErrNotExist.
func (s Store) Get(handle string) (Entry, error) {
	b := brain.Store{GridHome: s.GridHome, Project: s.Project}
	for _, level := range s.levels() {
		data, err := b.ReadConfig(level, "people", handle)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Entry{}, err
		}
		return decode(handle, data)
	}
	return Entry{}, fmt.Errorf("person %q: %w", handle, fs.ErrNotExist)
}

// List returns entries sorted by handle and reports invalid or older schemas
// as problems. Filesystem errors remain errors. A project file shadows the
// same home filename even when invalid, consistent with Get.
func (s Store) List() ([]Entry, []Problem, error) {
	entries, problems := []Entry{}, []Problem{}
	seen := map[string]bool{}
	b := brain.Store{GridHome: s.GridHome, Project: s.Project}
	for _, level := range s.levels() {
		names, err := b.ConfigNames(level, "people")
		if err != nil {
			return nil, nil, err
		}
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			data, err := b.ReadConfig(level, "people", name)
			if err != nil {
				return nil, nil, err
			}
			e, err := decode(name, data)
			if err != nil {
				problems = append(problems, Problem{Name: name, Reason: err.Error()})
				continue
			}
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Handle < entries[j].Handle })
	sort.Slice(problems, func(i, j int) bool { return problems[i].Name < problems[j].Name })
	return entries, problems, nil
}

// Entries is an already-read set of people, keeping selection independent of IO.
type Entries []Entry

// Approvers returns unique sorted owner/member handles with an exact approves
// or decides area link. With no matches it returns the caller's default owner.
func (entries Entries) Approvers(area, owner string) []string {
	set := map[string]bool{}
	for _, e := range entries {
		if e.Role != "owner" && e.Role != "member" {
			continue
		}
		for _, link := range e.Links {
			if link.To == area && (link.Rel == "approves" || link.Rel == "decides") {
				set[e.Handle] = true
			}
		}
	}
	if len(set) == 0 {
		return []string{owner}
	}
	handles := make([]string, 0, len(set))
	for handle := range set {
		handles = append(handles, handle)
	}
	sort.Strings(handles)
	return handles
}
