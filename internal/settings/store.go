package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// Document contains only explicit overrides on disk, and complete values from Effective.
type Document struct {
	Agents map[string]Agent  `json:"agents"`
	Skills map[string]bool   `json:"skills"`
	Hooks  map[string]string `json:"hooks"`
}

func empty() Document { return Document{map[string]Agent{}, map[string]bool{}, map[string]string{}} }

// Store uses <home>/settings.json for personal values and
// <home>/projects/<name>/settings.json for project overrides. Project can be a
// registered name or id; empty or personal selects personal settings.
type Store struct{ Home, Project string }

func (s Store) directory() (string, error) {
	if s.Home == "" {
		return "", errors.New("cannot find the Grid home; set GRID_HOME")
	}
	home := gridhome.Real(s.Home)
	if s.Project == "" || s.Project == gridhome.Personal {
		return home, nil
	}
	records, err := project.Load(home)
	if err != nil {
		return "", err
	}
	for _, rec := range records {
		if rec.Name == s.Project || rec.ID == s.Project {
			dir := filepath.Join(home, "projects", rec.Name)
			if gridhome.Real(dir) != dir {
				return "", errors.New("project settings directory must not be a symlink")
			}
			return dir, nil
		}
	}
	return "", fmt.Errorf("unknown project %q", s.Project)
}

func read(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", filepath.Base(path))
	}
	return os.ReadFile(path)
}
func readDocument(dir string) (Document, error) {
	b, err := read(filepath.Join(dir, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return Document{}, err
	}
	d, err := decode(b)
	if err != nil {
		return d, fmt.Errorf("settings.json: %w", err)
	}
	return d, nil
}

// Effective overlays defaults, personal values and individual project fields.
func (s Store) Effective() (Document, error) {
	dir, err := s.directory()
	if err != nil {
		return Document{}, err
	}
	doc := empty()
	for _, a := range agents {
		doc.Agents[a.name] = Agent{a.model, a.effort}
	}
	for _, name := range skills {
		doc.Skills[name] = true
	}
	for _, name := range hooks {
		doc.Hooks[name] = defaultLevel(name)
	}
	dirs := []string{gridhome.Real(s.Home)}
	if dir != dirs[0] {
		dirs = append(dirs, dir)
	}
	for _, dir := range dirs {
		d, err := readDocument(dir)
		if err != nil {
			return Document{}, err
		}
		merge(&doc, d)
	}
	return doc, nil
}
func merge(dst *Document, src Document) {
	for name, a := range src.Agents {
		old := dst.Agents[name]
		if a.Model != "" {
			old.Model = a.Model
		}
		if a.Effort != "" {
			old.Effort = a.Effort
		}
		dst.Agents[name] = old
	}
	for name, v := range src.Skills {
		dst.Skills[name] = v
	}
	for name, v := range src.Hooks {
		dst.Hooks[name] = v
	}
}

// ReadHookLevels is the single settings reader used by guards and the CLI.
func ReadHookLevels(home, project string) (map[string]string, error) {
	d, err := (Store{home, project}).Effective()
	return d.Hooks, err
}

// SetAgent changes only supplied fields; omitted fields continue to inherit.
func (s Store) SetAgent(name string, a Agent) error {
	if err := allowed("agent", name, AgentNames()); err != nil {
		return err
	}
	a.Model = strings.ToLower(a.Model)
	if err := validateAgent(a); err != nil {
		return err
	}
	return s.update(func(d *Document) { merge(d, Document{Agents: map[string]Agent{name: a}}) })
}

// SetSkill enables or disables a known skill.
func (s Store) SetSkill(name string, enabled bool) error {
	if err := allowed("skill", name, skills); err != nil {
		return err
	}
	return s.update(func(d *Document) { d.Skills[name] = enabled })
}

// SetHook sets a known hook's level.
func (s Store) SetHook(name, level string) error {
	if err := allowed("hook", name, hooks); err != nil {
		return err
	}
	if err := checkLevel(name, level); err != nil {
		return err
	}
	return s.update(func(d *Document) { d.Hooks[name] = level })
}
func (s Store) update(change func(*Document)) error {
	dir, err := s.directory()
	if err != nil {
		return err
	}
	// Refuse malformed existing data before creating even a lock file.
	if _, err := readDocument(dir); err != nil {
		return err
	}
	return s.locked(func() error {
		d, err := readDocument(dir)
		if err != nil {
			return err
		}
		change(&d)
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		return atomicfile.Write(filepath.Join(dir, "settings.json"), append(b, '\n'), 0o644)
	})
}

// locked serializes read-modify-write across processes with the shared fslock
// implementation. The stable lock inode is never removed or replaced.
func (s Store) locked(write func() error) error {
	home := gridhome.Real(s.Home)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(home, ".settings.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	version, err := gridhome.ReadFormat(home)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if why := gridhome.FormatProblem(version); why != "" {
			return errors.New(why)
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		if err := atomicfile.Write(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion)), 0o644); err != nil {
			return err
		}
	}
	return write()
}
