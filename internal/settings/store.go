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
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Document contains only explicit overrides on disk, and complete values from Effective.
type Document struct {
	Agents map[string]Agent  `json:"agents"`
	Skills map[string]bool   `json:"skills"`
	Hooks  map[string]string `json:"hooks"`
	// Budget names the preset Apply last wrote; empty when none was applied.
	Budget string `json:"budget,omitempty"`
	// Accounts holds the account records the app's account reads keep in the same
	// settings.json. The plugin owns no part of them: the array is carried through
	// every rewrite as it was read, never merged into Effective and never shown.
	Accounts json.RawMessage `json:"accounts,omitempty"`
	// ezKeysApp holds the seven keys the app's Settings page saves into the same
	// settings.json (the app's check.go field list). Like Accounts, the plugin owns
	// no part of them: each value is carried through every rewrite exactly as it was
	// read, never merged into Effective and never shown by a list.
	ezKeysApp
}

// ezKeysApp is the app's part of settings.json. The embedded struct is written
// inline, so a rewrite puts each key back beside the plugin's own.
type ezKeysApp struct {
	Notifications    json.RawMessage `json:"notifications,omitempty"`
	Models           json.RawMessage `json:"models,omitempty"`
	UsageRules       json.RawMessage `json:"usageRules,omitempty"`
	WindowRules      json.RawMessage `json:"windowRules,omitempty"`
	Workspace        json.RawMessage `json:"workspace,omitempty"`
	AccountSelection json.RawMessage `json:"accountSelection,omitempty"`
	HiddenProjects   json.RawMessage `json:"hiddenProjects,omitempty"`
}

// ezKeysAppNames lists the keys ezKeysApp carries; decode accepts these and no others.
var ezKeysAppNames = []string{"notifications", "models", "usageRules", "windowRules", "workspace", "accountSelection", "hiddenProjects"}

// ezKeysSet keeps the value of one app key verbatim.
func (k *ezKeysApp) ezKeysSet(key string, v json.RawMessage) {
	switch key {
	case "notifications":
		k.Notifications = v
	case "models":
		k.Models = v
	case "usageRules":
		k.UsageRules = v
	case "windowRules":
		k.WindowRules = v
	case "workspace":
		k.Workspace = v
	case "accountSelection":
		k.AccountSelection = v
	case "hiddenProjects":
		k.HiddenProjects = v
	}
}

func empty() Document {
	return Document{Agents: map[string]Agent{}, Skills: map[string]bool{}, Hooks: map[string]string{}}
}

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
	if src.Budget != "" {
		dst.Budget = src.Budget
	}
}

// WithBudget binds the budget operations to this store: the caller-aware one the
// CLI got from settingsStore, which already refused a guest for writes.
func (s Store) WithBudget(o BudgetOptions) BudgetOptions {
	o.GridHome = s.Home
	o.store = budgetStore{directory: s.directory, effective: s.Effective, locked: s.locked}
	return o
}

// SetAgents changes several agents, and optionally the applied preset name, in one
// locked write. An empty budget leaves the recorded preset as it is.
func (s Store) SetAgents(set map[string]Agent, budget string) error {
	for name, a := range set {
		if err := allowed("agent", name, AgentNames()); err != nil {
			return err
		}
		a.Model = strings.ToLower(a.Model)
		if err := validateAgent(a); err != nil {
			return err
		}
		set[name] = a
	}
	if budget != "" {
		if err := allowed("budget preset", budget, budgetPresetNames); err != nil {
			return err
		}
	}
	return s.update(func(d *Document) { merge(d, Document{Agents: set, Budget: budget}) })
}

// ReadHookLevels is the single settings reader used by guards and the CLI.
func ReadHookLevels(home, project string) (map[string]string, error) {
	d, err := (Store{home, project}).Effective()
	return d.Hooks, err
}

// ResolveHookLevel is the runtime level reader for both event hooks and guards.
// It shares project selection with grid where. On errors the returned block
// level keeps guards enforced and informational hooks enabled; callers report
// the error without replacing an existing denial. No hook has a fixed level: a
// guard's default is block, and a setting may lower it.
func ResolveHookLevel(name string, resolver where.Resolver, query where.Query) (string, error) {
	if resolver.GridHome == "" {
		return defaultLevel(name), nil
	}
	if query.Cwd == "" {
		query.Cwd = "."
	}
	if resolver.Machine == "" {
		resolver.Machine = gridhome.Machine(resolver.GridHome)
	}
	r, err := resolver.Resolve(query)
	if err != nil {
		return "block", fmt.Errorf("Cannot resolve hook settings: %w", err)
	}
	project := ""
	if r.Record {
		project = r.Project
	}
	levels, err := ReadHookLevels(resolver.GridHome, project)
	if err != nil {
		return "block", fmt.Errorf("Cannot read hook settings: %w", err)
	}
	return levels[name], nil
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
