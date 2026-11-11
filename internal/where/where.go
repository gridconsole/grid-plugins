// Package where decides which Grid project, and which task, a folder belongs to.
// The rules follow plugin-design.md, "How a session finds its project", in this order:
//
//  1. an explicit task or project wins
//  2. a folder inside a task folder (projects/<p>/tasks/ or personal/tasks/) is that task
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
// Its tasks live in <grid home>/personal/tasks, not under projects/.
const Personal = gridhome.Personal

// Resolver holds the machine facts a resolution needs, so tests can set them.
// Machine is this machine's id, "" before the first record is written here.
// Ceiling, when set, is the highest folder the walk for a git repo looks at.
type Resolver struct {
	GridHome  string
	Home      string
	TempRoots []string
	Machine   string
	Ceiling   string
	Caller    Caller
	// PersonalAllowed overrides DefaultPersonalAllowed when non-nil.
	PersonalAllowed func(Caller) bool
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
var taskULID = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

// Resolve applies the six rules.
func (r Resolver) Resolve(q Query) (Result, error) {
	if q.Cwd == "" {
		return Result{}, fmt.Errorf("no folder to resolve")
	}
	if r.GridHome == "" {
		return Result{}, fmt.Errorf("no Grid home")
	}
	personalOK := r.PersonalOK()
	if !personalOK {
		if strings.EqualFold(q.Project, Personal) {
			return Result{}, unregisteredProject(q.Project)
		}
		if strings.ContainsAny(q.Task, "/"+string(filepath.Separator)) || strings.HasPrefix(q.Task, ".") {
			path := q.Task
			if !filepath.IsAbs(path) {
				path = filepath.Join(q.Cwd, path)
			}
			if within(filepath.Join(r.GridHome, Personal, "tasks"), path) {
				return Result{}, ErrTaskNotFound
			}
		}
	}
	gridHome := gridhome.Real(r.GridHome)
	cwd := gridhome.Real(q.Cwd)
	records, err := r.Projects()
	if err != nil {
		return Result{}, fmt.Errorf("read projects: %w", err)
	}
	var res Result
	if q.Task != "" || q.Project != "" {
		res, err = r.explicit(gridHome, cwd, q, records, personalOK)
		if err != nil {
			return Result{}, err
		}
	} else {
		res = r.implicit(gridHome, cwd, records)
	}
	if res.Project == Personal && !personalOK {
		return Result{}, ErrPersonalRefused
	}
	if !r.GuestProjectAllowed(res.Project) || r.guest() && !res.Record && res.Task == "" {
		return Result{}, ErrProjectUnregistered
	}
	if res.TaskPath != "" && !r.guestTaskPathAllowed(res.TaskPath) {
		return Result{}, ErrTaskNotFound
	}
	return res, nil
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

func (r Resolver) explicit(gridHome, cwd string, q Query, records []project.Record, personalOK bool) (Result, error) {
	var rec *project.Record
	name := ""
	switch {
	case strings.EqualFold(q.Project, Personal):
		name = Personal // Personal has no project record
	case q.Project != "":
		found, ok := findProject(records, q.Project)
		if !ok {
			// A failed lookup with an ID match means that ID was ambiguous.
			for _, candidate := range records {
				if candidate.ID == q.Project {
					return Result{}, fmt.Errorf("%w: ambiguous project id", unregisteredProject(q.Project))
				}
			}
			return Result{}, unregisteredProject(q.Project)
		}
		if found.Name == Personal && !personalOK {
			return Result{}, unregisteredProject(q.Project)
		}
		rec, name = &found, found.Name
	}
	if q.Task == "" {
		if rec == nil {
			return newResult(RuleExplicit, Personal, cwd), nil
		}
		return r.fromRecord(*rec, RuleExplicit, r.firstFolder(*rec)), nil
	}
	proj, task, err := r.explicitTask(gridHome, cwd, q.Task, name, records, personalOK)
	if err != nil {
		return Result{}, err
	}
	if name != "" && name != proj {
		return Result{}, fmt.Errorf("task %s belongs to project %s, not %s", task, proj, name)
	}
	return r.taskResult(gridHome, records, RuleExplicit, proj, task), nil
}

// explicitTask turns --task into a project and task folder name. A path is refused
// unless it is inside the Grid home; an id is looked up in the explicit project
// (name), else in the project the folder resolves to.
func (r Resolver) explicitTask(gridHome, cwd, arg, name string, records []project.Record, personalOK bool) (string, string, error) {
	if strings.ContainsAny(arg, "/"+string(filepath.Separator)) || strings.HasPrefix(arg, ".") {
		abs := arg
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		real := gridhome.Real(abs)
		if !personalOK && within(filepath.Join(gridHome, Personal, "tasks"), real) {
			return "", "", ErrTaskNotFound
		}
		if !within(gridHome, real) {
			// A guest gets the unknown-task answer, not the host's Grid home path.
			if r.guest() {
				return "", "", fmt.Errorf("task not found")
			}
			return "", "", fmt.Errorf("task path %s is outside the Grid home %s", real, gridHome)
		}
		proj, task, ok := taskParts(gridHome, real)
		if !ok || !r.GuestProjectAllowed(proj) {
			// Invalid paths inside Grid home reveal neither its location nor
			// whether a case variant names a private Personal task.
			return "", "", ErrTaskNotFound
		}
		dir := filepath.Join(gridhome.TasksDir(gridHome, proj), task)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			message := fmt.Sprintf("task folder %s does not exist", dir)
			// A regular file where a task folder belongs is answered as a missing
			// folder, as the id form does.
			if err == nil || os.IsNotExist(err) {
				return "", "", &LookupError{Kind: ErrTaskNotFound, Message: message}
			}
			return "", "", fmt.Errorf("%s", message)
		}
		return proj, task, nil
	}
	if name == "" {
		if res := r.implicit(gridHome, cwd, records); res.Record || res.Task != "" {
			if res.Project == Personal && !personalOK {
				return "", "", ErrPersonalRefused
			}
			name = res.Project
		}
	}
	if name == "" && taskULID.MatchString(arg) {
		// A task ULID is unique in the home, so it needs no project.
		names := []string{}
		for _, rec := range records {
			names = append(names, rec.Name)
		}
		if personalOK {
			names = append(names, Personal)
		}
		for _, n := range names {
			if !r.GuestProjectAllowed(n) || !r.guestTaskPathAllowed(gridhome.TasksDir(gridHome, n)) {
				continue
			}
			if task := taskWithID(gridhome.TasksDir(gridHome, n), arg); task != "" {
				return n, task, nil
			}
		}
	}
	if name == "" {
		if !personalOK {
			return "", "", ErrTaskNotFound
		}
		return "", "", fmt.Errorf("task %s needs --project: this folder is in no project with a record", arg)
	}
	if !r.GuestProjectAllowed(name) || !r.guestTaskPathAllowed(gridhome.TasksDir(gridHome, name)) {
		return "", "", ErrTaskNotFound
	}
	entries, err := os.ReadDir(gridhome.TasksDir(gridHome, name))
	if err != nil {
		if !personalOK {
			return "", "", ErrTaskNotFound
		}
		message := fmt.Sprintf("task %s not found in project %s", arg, name)
		if os.IsNotExist(err) {
			return "", "", &LookupError{Kind: ErrTaskNotFound, Message: message}
		}
		return "", "", fmt.Errorf("%s", message)
	}
	for _, e := range entries {
		if e.IsDir() && (e.Name() == arg || strings.HasPrefix(e.Name(), arg+"-")) {
			return name, e.Name(), nil
		}
	}
	// Task IDs are ULIDs (contract/README.md): the worker's spawn names the task by
	// the id in its task.md, so that id selects the task as its folder name does.
	if taskULID.MatchString(arg) {
		if task := taskWithID(gridhome.TasksDir(gridHome, name), arg); task != "" {
			return name, task, nil
		}
	}
	if !personalOK {
		return "", "", ErrTaskNotFound
	}
	return "", "", &LookupError{Kind: ErrTaskNotFound, Message: fmt.Sprintf("task %s not found in project %s", arg, name)}
}

// taskWithID is the folder name of the task in tasksDir whose task.md gives id, or "".
func taskWithID(tasksDir, id string) string {
	entries, _ := os.ReadDir(tasksDir)
	for _, e := range entries {
		if e.IsDir() && hasTaskID(filepath.Join(tasksDir, e.Name(), "task.md"), id) {
			return e.Name()
		}
	}
	return ""
}

// hasTaskID reports whether task.md's frontmatter gives id (as id: or ulid:, the
// keys feed.TaskID reads; feed cannot be imported here).
func hasTaskID(taskMD, id string) bool {
	data, err := os.ReadFile(taskMD)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return false
		}
		key, v, ok := strings.Cut(line, ":")
		if ok && (key == "id" || key == "ulid") && strings.Trim(strings.TrimSpace(v), "\"'") == id {
			return true
		}
	}
	return false
}

// taskParts splits <grid home>/projects/<project>/tasks/<task>[/...], or
// <grid home>/personal/tasks/<task>[/...] for Personal.
func taskParts(gridHome, path string) (proj, task string, ok bool) {
	rel, err := filepath.Rel(gridHome, path)
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 3 && parts[0] == Personal && parts[1] == "tasks" && taskName.MatchString(parts[2]) {
		return Personal, parts[2], true
	}
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
	if rec, ok := byName(records, nameOrID); ok {
		return rec, true
	}
	var found project.Record
	ok := false
	for _, rec := range records {
		if rec.ID != "" && rec.ID == nameOrID {
			if ok {
				return project.Record{}, false
			}
			found, ok = rec, true
		}
	}
	return found, ok
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
	if rec, ok := byName(records, proj); ok && proj != Personal {
		res = r.fromRecord(rec, rule, r.firstFolder(rec))
	}
	res.Task = task
	res.TaskPath = filepath.Join(gridhome.TasksDir(gridHome, proj), task)
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
