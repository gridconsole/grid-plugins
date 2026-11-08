// Package task creates and reads task folders in the Grid home.
package task

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// Record is the front matter of task.md. An empty workflow leaves selection to
// the workflow rules; creation does not start work or infer a small task.
type Record struct {
	ID       string `yaml:"id" json:"id"`
	Title    string `yaml:"title" json:"title"`
	Project  string `yaml:"project" json:"project"`
	Size     string `yaml:"size" json:"size"`
	Workflow string `yaml:"workflow" json:"workflow"`
	State    string `yaml:"state" json:"state"`
	Created  string `yaml:"created" json:"created"`
}

// Task folds the task record and the person's unmodified words into one view.
type Task struct {
	Record `yaml:",inline"`
	Path   string `json:"path"`
	Intent string `json:"intent"`
}

// Store uses the existing project resolver. Now is optional and defaults to time.Now.
type Store struct {
	Resolver where.Resolver
	Now      func() time.Time
}

// NewOptions contains only choices supplied by the caller.
type NewOptions struct {
	Cwd, Words, Project, Size, Workflow string
}

// Validate checks creation choices before any files are written.
func (o NewOptions) Validate() error {
	if strings.TrimSpace(o.Words) == "" {
		return errors.New("task words must not be empty")
	}
	switch o.Size {
	case "", "auto", "small", "normal", "large":
	default:
		return fmt.Errorf("invalid size %q: use auto, small, normal or large", o.Size)
	}
	if strings.ContainsAny(o.Workflow, "\r\n\x00") || (o.Workflow != "" && strings.TrimSpace(o.Workflow) == "") {
		return errors.New("workflow must be a nonempty name on one line")
	}
	return nil
}

var folderPattern = regexp.MustCompile(`^T-([0-9]+)(?:-.+)?$`)
var idPattern = regexp.MustCompile(`^T-[0-9]+$`)

func number(name string) (uint64, error) {
	m := folderPattern.FindStringSubmatch(name)
	if m == nil {
		return 0, fmt.Errorf("invalid task folder %q", name)
	}
	n, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid task number in %q", name)
	}
	return n, nil
}

func taskDir(home, proj string) string {
	if proj == "personal" {
		return filepath.Join(home, "personal", "tasks")
	}
	return filepath.Join(home, "projects", proj, "tasks")
}

// resolve supplements where with the Personal task layout, without changing
// project resolution. Explicit Personal selection needs no project record.
func (s Store) resolve(cwd, proj string) (where.Result, error) {
	if s.Resolver.GridHome == "" {
		return where.Result{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	if strings.EqualFold(proj, "personal") {
		return where.Result{Project: "personal", Rule: where.RulePersonal}, nil
	}
	if proj == "" {
		base := taskDir(gridhome.Real(s.Resolver.GridHome), "personal")
		rel, err := filepath.Rel(base, gridhome.Real(cwd))
		parts := strings.Split(rel, string(filepath.Separator))
		if err == nil && len(parts) > 0 && folderPattern.MatchString(parts[0]) {
			return where.Result{Project: "personal", Rule: where.RuleTask, Task: parts[0], TaskPath: filepath.Join(base, parts[0])}, nil
		}
	}
	r, err := s.Resolver.Resolve(where.Query{Cwd: cwd, Project: proj})
	if r.Rule == where.RulePersonal {
		r.Project = "personal"
	}
	return r, err
}

// Fold reads YAML front matter, preserving intent byte for byte. Task state is
// the stored state; execution and Feed lifecycle folds belong to their writers.
func Fold(path, text, intent string) (Task, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Task{}, errors.New("task.md has no YAML front matter")
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return Task{}, errors.New("task.md front matter is not closed")
	}
	var rec Record
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &rec); err != nil {
		return Task{}, fmt.Errorf("task.md front matter: %w", err)
	}
	n, err := number(filepath.Base(path))
	if err != nil {
		return Task{}, err
	}
	id := fmt.Sprintf("T-%02d", n)
	if rec.ID == "" {
		rec.ID = id
	}
	if rec.ID != id || strings.TrimSpace(rec.Title) == "" || rec.Project == "" || rec.State == "" {
		return Task{}, errors.New("task.md needs a matching id, title, project and state")
	}
	if err := (NewOptions{Words: rec.Title, Size: rec.Size, Workflow: rec.Workflow}).Validate(); err != nil {
		return Task{}, err
	}
	if rec.Size == "" {
		rec.Size = "auto"
	}
	if _, err := time.Parse(time.RFC3339, rec.Created); err != nil {
		return Task{}, fmt.Errorf("task.md created must be an RFC3339 timestamp: %w", err)
	}
	return Task{Record: rec, Path: path, Intent: intent}, nil
}

// Read folds both files of a published task folder.
func Read(path string) (Task, error) {
	text, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		return Task{}, err
	}
	intent, err := os.ReadFile(filepath.Join(path, "intent.md"))
	if err != nil {
		return Task{}, err
	}
	t, err := Fold(path, string(text), string(intent))
	if err != nil {
		return Task{}, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}

// Show resolves a display id in the current project, or the enclosing task when
// id is empty. It never creates project records or other files.
func (s Store) Show(cwd, id string) (Task, error) {
	if id != "" && !idPattern.MatchString(id) {
		return Task{}, fmt.Errorf("invalid task id %q: use T-NN", id)
	}
	r, err := s.resolve(cwd, "")
	if err != nil {
		return Task{}, err
	}
	if id == "" {
		if r.TaskPath == "" {
			return Task{}, errors.New("give a task id (T-NN), or run inside a task folder")
		}
		return Read(r.TaskPath)
	}
	paths, err := folders(taskDir(s.Resolver.GridHome, r.Project))
	if err != nil {
		return Task{}, err
	}
	var found string
	for _, path := range paths {
		name := filepath.Base(path)
		if name == id || strings.HasPrefix(name, id+"-") {
			if found != "" {
				return Task{}, fmt.Errorf("task %s has more than one folder in project %s", id, r.Project)
			}
			found = path
		}
	}
	if found == "" {
		return Task{}, fmt.Errorf("task %s not found in project %s", id, r.Project)
	}
	return Read(found)
}

// List returns tasks in numeric id order, with an optional exact state filter.
func (s Store) List(cwd, proj, state string) ([]Task, error) {
	r, err := s.resolve(cwd, proj)
	if err != nil {
		return nil, err
	}
	paths, err := folders(taskDir(s.Resolver.GridHome, r.Project))
	if err != nil {
		return nil, err
	}
	out := []Task{}
	seen := map[string]bool{}
	for _, path := range paths {
		t, err := Read(path)
		if err != nil {
			return nil, err
		}
		if t.Project != r.Project || seen[t.ID] {
			return nil, fmt.Errorf("task %s has a mismatched project or duplicate id", path)
		}
		seen[t.ID] = true
		if state == "" || state == t.State {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := number(out[i].ID)
		b, _ := number(out[j].ID)
		return a < b
	})
	return out, nil
}

func folders(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if folderPattern.MatchString(e.Name()) {
			if !e.IsDir() {
				return nil, fmt.Errorf("task %s is not a directory", filepath.Join(dir, e.Name()))
			}
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out, nil
}
