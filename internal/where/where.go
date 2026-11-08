// Package where decides which Grid project, and which task, a folder belongs to.
// The rules follow plugin-design.md, "How a session finds its project", in this order:
//
//  1. an explicit task or project wins
//  2. a folder inside a task folder is that task
//  3. the nearest folder above that a project record lists is that project
//  4. a folder outside git that holds git repos (depth 2) is a multi-repo project
//  5. a git repo is a one-repo project, keyed by git-common-dir and remote, or by its
//     root commit when it has no remote
//  6. else the folder itself; home, the root, temp folders and the Grid home outside a
//     task folder are the Personal project
//
// where reads only. A project record is written by the first task, not here.
package where

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// Rule names the rule that decided.
type Rule string

const (
	RuleExplicit  Rule = "explicit"
	RuleTask      Rule = "task"
	RuleRecord    Rule = "record"
	RuleMultiRepo Rule = "multi-repo"
	RuleGit       Rule = "git"
	RuleFolder    Rule = "folder"
	RulePersonal  Rule = "personal"
)

// Personal is the project for home, the root, temp folders and the Grid home itself.
const Personal = "Personal"

// Resolver holds the machine facts a resolution needs, so tests can set them.
// Machine is this machine's id, "" before the first record is written here.
// Ceiling, when set, is the highest folder the walk for a git repo looks at.
type Resolver struct {
	GridHome  string
	Home      string
	TempRoots []string
	Machine   string
	Ceiling   string
}

// Query is what to resolve: the folder, or an explicit task or project.
type Query struct {
	Cwd     string
	Task    string // a task folder path, or a task id such as T-07
	Project string // a project name or id
}

// Result is the answer. Record is false when no project record exists yet.
type Result struct {
	Project  string   `json:"project"`
	ID       string   `json:"id"`
	Rule     Rule     `json:"rule"`
	Folder   string   `json:"folder"`
	Record   bool     `json:"record"`
	Repos    []string `json:"repos"`
	Keys     []string `json:"keys"`
	Task     string   `json:"task"`
	TaskPath string   `json:"task_path"`
}

var taskName = regexp.MustCompile(`^T-\d+(-.*)?$`)

// Resolve applies the six rules.
func (r Resolver) Resolve(q Query) (Result, error) {
	if q.Cwd == "" {
		return Result{}, fmt.Errorf("no folder to resolve")
	}
	if r.GridHome == "" {
		return Result{}, fmt.Errorf("no Grid home")
	}
	gridHome := gridhome.Real(r.GridHome)
	cwd := gridhome.Real(q.Cwd)
	records, err := project.Load(gridHome)
	if err != nil {
		return Result{}, fmt.Errorf("read projects: %w", err)
	}
	if q.Task != "" || q.Project != "" {
		return r.explicit(gridHome, cwd, q, records)
	}
	return r.implicit(gridHome, cwd, records), nil
}

func (r Resolver) implicit(gridHome, cwd string, records []project.Record) Result {
	if proj, task, ok := taskParts(gridHome, cwd); ok {
		return r.taskResult(gridHome, records, RuleTask, proj, task)
	}
	if within(gridHome, cwd) {
		return newResult(RulePersonal, Personal, cwd) // Grid's own data is never a project folder
	}
	if rec, folder, ok := r.nearestRecord(records, cwd); ok {
		return r.fromRecord(rec, RuleRecord, folder)
	}
	ceiling := ""
	if r.Ceiling != "" {
		ceiling = gridhome.Real(r.Ceiling)
	}
	root, inRepo := gitx.FindRoot(cwd, ceiling)
	special := r.special(cwd)
	if !inRepo && !special {
		if repos := childRepos(cwd); len(repos) > 0 {
			res := newResult(RuleMultiRepo, freeName(gridHome, filepath.Base(cwd)), cwd)
			res.Repos = repos
			return res
		}
	}
	if inRepo {
		return r.gitProject(gridHome, records, root)
	}
	if special {
		return newResult(RulePersonal, Personal, cwd)
	}
	return newResult(RuleFolder, freeName(gridHome, filepath.Base(cwd)), cwd)
}

func (r Resolver) explicit(gridHome, cwd string, q Query, records []project.Record) (Result, error) {
	var rec *project.Record
	if q.Project != "" {
		found, ok := findProject(records, q.Project)
		if !ok {
			return Result{}, fmt.Errorf("project %q is not in %s", q.Project, filepath.Join(gridHome, "projects"))
		}
		rec = &found
	}
	if q.Task == "" {
		return r.fromRecord(*rec, RuleExplicit, r.firstFolder(*rec)), nil
	}
	proj, task, err := r.explicitTask(gridHome, cwd, q.Task, rec, records)
	if err != nil {
		return Result{}, err
	}
	if rec != nil && rec.Name != proj {
		return Result{}, fmt.Errorf("task %s belongs to project %s, not %s", task, proj, rec.Name)
	}
	return r.taskResult(gridHome, records, RuleExplicit, proj, task), nil
}

// explicitTask turns --task into a project and task folder name. A path is refused
// unless it is inside the Grid home; an id is looked up in the explicit project, else
// in the project the folder resolves to.
func (r Resolver) explicitTask(gridHome, cwd, arg string, rec *project.Record, records []project.Record) (string, string, error) {
	if strings.ContainsAny(arg, "/"+string(filepath.Separator)) || strings.HasPrefix(arg, ".") {
		abs := arg
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		real := gridhome.Real(abs)
		if !within(gridHome, real) {
			return "", "", fmt.Errorf("task path %s is outside the Grid home %s", real, gridHome)
		}
		proj, task, ok := taskParts(gridHome, real)
		if !ok {
			return "", "", fmt.Errorf("%s is not inside a task folder of %s", real, gridHome)
		}
		if _, err := os.Stat(filepath.Join(gridHome, "projects", proj, "tasks", task)); err != nil {
			return "", "", fmt.Errorf("task folder %s does not exist", filepath.Join(gridHome, "projects", proj, "tasks", task))
		}
		return proj, task, nil
	}
	name := ""
	if rec != nil {
		name = rec.Name
	} else if res := r.implicit(gridHome, cwd, records); res.Record || res.Task != "" {
		name = res.Project
	}
	if name == "" {
		return "", "", fmt.Errorf("task %s needs --project: this folder is in no project with a record", arg)
	}
	entries, err := os.ReadDir(filepath.Join(gridHome, "projects", name, "tasks"))
	if err != nil {
		return "", "", fmt.Errorf("task %s not found in project %s", arg, name)
	}
	for _, e := range entries {
		if e.IsDir() && (e.Name() == arg || strings.HasPrefix(e.Name(), arg+"-")) {
			return name, e.Name(), nil
		}
	}
	return "", "", fmt.Errorf("task %s not found in project %s", arg, name)
}

// taskParts splits <grid home>/projects/<project>/tasks/<task>[/...].
func taskParts(gridHome, path string) (proj, task string, ok bool) {
	rel, err := filepath.Rel(gridHome, path)
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 4 || parts[0] != "projects" || parts[2] != "tasks" || !taskName.MatchString(parts[3]) {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func within(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (r Resolver) nearestRecord(records []project.Record, cwd string) (project.Record, string, bool) {
	folders := make([][]string, len(records))
	for i, rec := range records {
		folders[i] = rec.FoldersOn(r.Machine)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		for i, rec := range records {
			for _, f := range folders[i] {
				if f == dir {
					return rec, f, true
				}
			}
		}
		if filepath.Dir(dir) == dir {
			return project.Record{}, "", false
		}
	}
}

func findProject(records []project.Record, nameOrID string) (project.Record, bool) {
	for _, rec := range records {
		if rec.Name == nameOrID || (rec.ID != "" && rec.ID == nameOrID) {
			return rec, true
		}
	}
	return project.Record{}, false
}

func byName(records []project.Record, name string) (project.Record, bool) {
	for _, rec := range records {
		if rec.Name == name {
			return rec, true
		}
	}
	return project.Record{}, false
}

func (r Resolver) firstFolder(rec project.Record) string {
	if fs := rec.FoldersOn(r.Machine); len(fs) > 0 {
		return fs[0]
	}
	return ""
}

func newResult(rule Rule, name, folder string) Result {
	return Result{Project: name, Rule: rule, Folder: folder, Repos: []string{}, Keys: []string{}}
}

func (r Resolver) fromRecord(rec project.Record, rule Rule, folder string) Result {
	res := newResult(rule, rec.Name, folder)
	res.ID = rec.ID
	res.Record = true
	res.Keys = append(res.Keys, rec.KeysOn(r.Machine)...)
	return res
}

func (r Resolver) taskResult(gridHome string, records []project.Record, rule Rule, proj, task string) Result {
	res := newResult(rule, proj, "")
	if rec, ok := byName(records, proj); ok {
		res = r.fromRecord(rec, rule, r.firstFolder(rec))
	}
	res.Task = task
	res.TaskPath = filepath.Join(gridHome, "projects", proj, "tasks", task)
	return res
}

// gitProject keys a repo by its git-common-dir and remotes, so a clone, a worktree or a
// moved folder finds the record it already has. The root commit is a key only when the
// repo has no remote: a fork shares it but is another project.
func (r Resolver) gitProject(gridHome string, records []project.Record, root string) Result {
	folder := root
	keys := []string{}
	if common, err := gitx.CommonDir(root); err == nil {
		keys = append(keys, "common:"+common)
		if filepath.Base(common) == ".git" {
			folder = filepath.Dir(common) // a worktree belongs to its main repo's folder
		}
	}
	var remotes []string
	seen := map[string]bool{}
	for _, raw := range gitx.Remotes(root) {
		if n := project.NormalizeRemote(raw); n != "" && !seen[n] {
			seen[n] = true
			remotes = append(remotes, "remote:"+n)
		}
	}
	sort.Strings(remotes)
	keys = append(keys, remotes...)
	if len(remotes) == 0 {
		if sha := gitx.RootCommit(root); sha != "" {
			keys = append(keys, "root:"+sha)
		}
	}
	for _, rec := range records {
		if rec.HasAny(r.Machine, keys) {
			f := r.firstFolder(rec)
			if f == "" {
				f = folder
			}
			res := r.fromRecord(rec, RuleGit, f)
			res.Keys = keys
			return res
		}
	}
	res := newResult(RuleGit, freeName(gridHome, filepath.Base(folder)), folder)
	res.Keys = keys
	return res
}

// childRepos lists git repos one or two levels below dir, never entering a repo.
func childRepos(dir string) []string {
	var repos []string
	var walk func(d string, depth int)
	walk = func(d string, depth int) {
		entries, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(d, e.Name())
			if gitx.IsRepoRoot(p) {
				repos = append(repos, p)
			} else if depth < 2 {
				walk(p, depth+1)
			}
		}
	}
	walk(dir, 1)
	sort.Strings(repos)
	return repos
}

// freeName is the readable project folder name: the base name, with a numeric suffix
// when another project already uses it.
func freeName(gridHome, base string) string {
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "project"
	}
	name := base
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(gridHome, "projects", name)); err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
}

// special reports home, the filesystem root, and the temp folders with all below them.
func (r Resolver) special(dir string) bool {
	if dir == string(filepath.Separator) {
		return true
	}
	if r.Home != "" && dir == gridhome.Real(r.Home) {
		return true
	}
	for _, t := range r.TempRoots {
		if t != "" && within(gridhome.Real(t), dir) {
			return true
		}
	}
	return false
}
