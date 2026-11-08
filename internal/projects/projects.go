// Package projects manages the project registry in the Grid home.
package projects

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Store uses the same machine facts and resolution rules as grid where.
type Store struct{ Resolver where.Resolver }

// Entry is the public JSON shape; repos may repeat a path to retain every remote.
type Entry struct {
	Name    string   `json:"name"`
	ID      string   `json:"id"`
	Folders []Folder `json:"folders"`
	Repos   []Repo   `json:"repos"`
	Keys    []Key    `json:"keys"`
}
type Folder struct {
	Machine string `json:"machine"`
	Path    string `json:"path"`
}
type Repo struct {
	Path   string `json:"path"`
	Remote string `json:"remote"`
}
type Key struct {
	Key     string `json:"key"`
	Machine string `json:"machine,omitempty"`
}

func entry(r project.Record) Entry {
	e := Entry{Name: r.Name, ID: r.ID, Folders: []Folder{}, Repos: []Repo{}, Keys: []Key{}}
	for _, f := range r.Folders {
		e.Folders = append(e.Folders, Folder{f.Machine, f.Path})
	}
	for _, p := range r.Repos {
		e.Repos = append(e.Repos, Repo{p.Path, p.Remote})
	}
	for _, k := range r.Keys {
		e.Keys = append(e.Keys, Key{k.Key, k.Machine})
	}
	return e
}

func Name(name string) error {
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\r\n\x00") || strings.TrimSpace(name) != name {
		return fmt.Errorf("project name must be one non-hidden folder name")
	}
	return nil
}

func (s Store) records() ([]project.Record, error) {
	home := s.Resolver.GridHome
	dir, err := localdata.Safe(home, "projects")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if _, err := localdata.Safe(home, "projects", e.Name(), "project.md"); err != nil {
			return nil, err
		}
	}
	return project.Load(home)
}

func (s Store) List() ([]Entry, error) {
	records, err := s.records()
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, r := range records {
		out = append(out, entry(r))
	}
	return out, nil
}

func (s *Store) machine() error {
	id, err := localdata.Machine(s.Resolver.GridHome)
	if err == nil {
		s.Resolver.Machine = id
	}
	return err
}

func directory(path string) (string, error) {
	path = gridhome.Real(path)
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s is not a directory", path)
	}
	return path, nil
}

// inspect enumerates repos at depth two, never walking into a repo or symlink.
func inspect(folder, machine string) ([]project.Repo, []project.Key, error) {
	roots := []string{}
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if gitx.IsRepoRoot(dir) {
			roots = append(roots, dir)
			return nil
		}
		if depth == 0 {
			return nil
		}
		es, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range es {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				if err := walk(filepath.Join(dir, e.Name()), depth-1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(folder, 2); err != nil {
		return nil, nil, err
	}
	repos := []project.Repo{}
	keys := []project.Key{project.NewKey(machine, "folder:"+folder)}
	for _, root := range roots {
		rel, _ := filepath.Rel(folder, root)
		common, err := gitx.CommonDir(root)
		if err != nil {
			return nil, nil, fmt.Errorf("inspect git repo %s: %w", root, err)
		}
		keys = append(keys, project.NewKey(machine, "common:"+common))
		remotes := map[string]bool{}
		for _, raw := range gitx.Remotes(root) {
			if n := project.NormalizeRemote(raw); n != "" {
				remotes[n] = true
			}
		}
		names := []string{}
		for n := range remotes {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) == 0 {
			repos = append(repos, project.Repo{Path: filepath.ToSlash(rel)})
			if sha := gitx.RootCommit(root); sha != "" {
				keys = append(keys, project.NewKey(machine, "root:"+sha))
			}
		}
		for _, n := range names {
			repos = append(repos, project.Repo{Path: filepath.ToSlash(rel), Remote: n})
			keys = append(keys, project.NewKey(machine, "remote:"+n))
		}
	}
	return repos, keys, nil
}

func (s Store) resolve(path string) (where.Result, error) {
	path, err := directory(path)
	if err != nil {
		return where.Result{}, err
	}
	return s.Resolver.Resolve(where.Query{Cwd: path})
}

// Add explicitly records a project. Existing identities are retained across clones.
func (s Store) Add(path, name string) (Entry, error) {
	if name != "" {
		if err := Name(name); err != nil {
			return Entry{}, err
		}
	}
	if _, err := directory(path); err != nil {
		return Entry{}, err
	}
	unlock, err := localdata.Lock(s.Resolver.GridHome, ".projects.lock")
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	if err := s.machine(); err != nil {
		return Entry{}, err
	}
	return s.add(path, name)
}

func (s Store) add(path, name string) (Entry, error) {
	records, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	res, err := s.resolve(path)
	if err != nil {
		return Entry{}, err
	}
	folder := res.Folder
	// For a remote match, inspect the new clone rather than a stale registered path.
	if root, ok := gitx.FindRoot(gridhome.Real(path), s.Resolver.Ceiling); ok && res.Rule == where.RuleGit {
		folder = root
		if common, err := gitx.CommonDir(root); err == nil && filepath.Base(common) == ".git" {
			folder = filepath.Dir(common)
		}
	}
	personal := res.Rule == where.RulePersonal
	rec := project.Record{Name: res.Project, ID: res.ID}
	if personal {
		rec.Name = where.Personal
		folder = ""
	}
	for _, r := range records {
		if r.Name == rec.Name && (res.Record || personal) {
			rec = r
		}
	}
	if res.Record && res.Rule == where.RuleGit {
		for _, repo := range rec.Repos {
			if repo.Path != "." {
				return Entry{}, fmt.Errorf("repo belongs to multi-repo project %s; relink its containing folder instead", rec.Name)
			}
		}
	}
	if name != "" && name != rec.Name {
		if res.Record || personal {
			return Entry{}, fmt.Errorf("folder belongs to project %s", rec.Name)
		}
		rec.Name = name
	}
	if err := Name(rec.Name); err != nil {
		return Entry{}, err
	}
	if !personal && rec.Name == where.Personal {
		return Entry{}, fmt.Errorf("Personal is reserved for folders outside projects")
	}
	if !res.Record && rec.ID == "" && !personal {
		base := rec.Name
		for n := 2; ; n++ {
			p, err := localdata.Safe(s.Resolver.GridHome, "projects", rec.Name)
			if err != nil {
				return Entry{}, err
			}
			_, err = os.Lstat(p)
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			if err != nil {
				return Entry{}, err
			}
			rec.Name = fmt.Sprintf("%s-%d", base, n)
		}
	}
	if personal { // Personal is a single data project with no machine folder binding.
		for _, r := range records {
			if r.Name == where.Personal {
				rec = r
			}
		}
	}
	if folder != "" {
		repos, keys, err := inspect(folder, s.Resolver.Machine)
		if err != nil {
			return Entry{}, err
		}
		if err := s.conflicts(records, rec, folder, keys); err != nil {
			return Entry{}, err
		}
		found := false
		for _, f := range rec.Folders {
			if f.Machine == s.Resolver.Machine && f.Path == folder {
				found = true
			}
		}
		if !found {
			rec.Folders = append(rec.Folders, project.Folder{Machine: s.Resolver.Machine, Path: folder})
		}
		rec.Repos = repos
		rec.Keys = mergeKeys(rec.Keys, keys)
	}
	if rec.ID == "" {
		rec.ID, err = localdata.ID()
		if err != nil {
			return Entry{}, err
		}
	}
	if err := s.write(rec); err != nil {
		return Entry{}, err
	}
	return entry(rec), nil
}

func mergeKeys(a, b []project.Key) []project.Key {
	for _, k := range b {
		found := false
		for _, old := range a {
			if old == k {
				found = true
			}
		}
		if !found {
			a = append(a, k)
		}
	}
	return a
}
func within(a, b string) bool {
	rel, err := filepath.Rel(a, b)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (s Store) conflicts(records []project.Record, rec project.Record, folder string, keys []project.Key) error {
	stringsKeys := []string{}
	for _, k := range keys {
		stringsKeys = append(stringsKeys, k.Key)
	}
	for _, other := range records {
		if other.Name == rec.Name {
			continue
		}
		conflict := other.HasAny(s.Resolver.Machine, stringsKeys)
		for _, f := range other.FoldersOn(s.Resolver.Machine) {
			conflict = conflict || within(f, folder) || within(folder, f)
		}
		if conflict {
			return fmt.Errorf("folder or repo already belongs to project %s; relink that project or merge it before adding", other.Name)
		}
	}
	return nil
}

func find(records []project.Record, name string) (project.Record, error) {
	var found []project.Record
	for _, r := range records {
		if r.Name == name || r.ID == name {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		return project.Record{}, fmt.Errorf("project %q is missing or ambiguous", name)
	}
	return found[0], nil
}

// Remove unlinks this machine, retaining project identity and all project data.
func (s Store) Remove(name string) (Entry, error) { return s.change(name, "") }

// Relink replaces this machine's folder bindings, retaining the stable project id.
func (s Store) Relink(name, path string) (Entry, error) {
	path, err := directory(path)
	if err != nil {
		return Entry{}, err
	}
	return s.change(name, path)
}
func (s Store) change(name, path string) (Entry, error) {
	unlock, err := localdata.Lock(s.Resolver.GridHome, ".projects.lock")
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	if err := s.machine(); err != nil {
		return Entry{}, err
	}
	records, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	rec, err := find(records, name)
	if err != nil {
		return Entry{}, err
	}
	folders := []project.Folder{}
	for _, f := range rec.Folders {
		if f.Machine != s.Resolver.Machine {
			folders = append(folders, f)
		}
	}
	rec.Folders = folders
	keys := []project.Key{}
	for _, k := range rec.Keys {
		if k.Machine != s.Resolver.Machine {
			keys = append(keys, k)
		}
	}
	rec.Keys = keys
	if path != "" {
		if rec.Name == where.Personal {
			return Entry{}, fmt.Errorf("Personal has no folder to relink")
		}
		res, err := s.resolve(path)
		if err != nil {
			return Entry{}, err
		}
		if res.Rule == where.RulePersonal {
			return Entry{}, fmt.Errorf("cannot relink to a Personal folder")
		}
		repos, keys, err := inspect(path, s.Resolver.Machine)
		if err != nil {
			return Entry{}, err
		}
		if err := s.conflicts(records, rec, path, keys); err != nil {
			return Entry{}, err
		}
		rec.Folders = append(rec.Folders, project.Folder{Machine: s.Resolver.Machine, Path: path})
		rec.Repos = repos
		rec.Keys = mergeKeys(rec.Keys, keys)
	}
	if err := s.write(rec); err != nil {
		return Entry{}, err
	}
	return entry(rec), nil
}

// write stages through project.Write and preserves any prose after front matter.
func (s Store) write(rec project.Record) error {
	target, err := localdata.Safe(s.Resolver.GridHome, "projects", rec.Name, "project.md")
	if err != nil {
		return err
	}
	old, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stage, err := os.MkdirTemp(s.Resolver.GridHome, ".project-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := project.Write(stage, rec); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(stage, "projects", rec.Name, "project.md"))
	if err != nil {
		return err
	}
	// project.Parse has already validated the existing front matter on reads.
	lines := bytes.SplitAfter(old, []byte("\n"))
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			data = append(data, bytes.Join(lines[i+1:], nil)...)
			break
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return localdata.Write(target, data)
}

// Proposal describes a project found under a discovery folder. Scan never writes.
type Proposal struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	Rule       where.Rule `json:"rule"`
	Registered bool       `json:"registered"`
}

// Scan treats each visible direct child as a candidate, keeping multi-repo groups.
func (s Store) Scan(path string) ([]Proposal, error) {
	path, err := directory(path)
	if err != nil {
		return nil, err
	}
	if _, err := s.records(); err != nil {
		return nil, err
	}
	es, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := []Proposal{}
	for _, e := range es {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(path, e.Name())
		res, err := s.resolve(p)
		if err != nil {
			return nil, err
		}
		if res.Rule == where.RulePersonal {
			continue
		}
		if res.Rule == where.RuleRecord && !within(path, res.Folder) {
			continue
		}
		if root, ok := gitx.FindRoot(p, s.Resolver.Ceiling); ok && !within(path, root) {
			continue
		}
		out = append(out, Proposal{Name: res.Project, Path: p, Rule: res.Rule, Registered: res.Record})
	}
	return out, nil
}
