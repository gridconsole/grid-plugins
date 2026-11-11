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

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// Record is the front matter of task.md. An empty workflow leaves selection to
// the workflow rules; creation does not start work or infer a small task.
type Record struct {
	ID      string `yaml:"id" json:"id"`
	Title   string `yaml:"title" json:"title"`
	Project string `yaml:"project" json:"project"`
	// Owner is the person who made the task. OwnerRole is the role job render
	// trusts intent.md by (member, owner, lead or guest); a task without a known
	// role is external there. Tasks made before these fields have neither.
	Owner     string `yaml:"owner,omitempty" json:"owner,omitempty"`
	OwnerRole string `yaml:"owner_role,omitempty" json:"owner_role,omitempty"`
	Size      string `yaml:"size" json:"size"`
	Workflow  string `yaml:"workflow" json:"workflow"`
	State     string `yaml:"state" json:"state"`
	Created   string `yaml:"created" json:"created"`
}

// Task folds the task record and the person's unmodified words into one view.
type Task struct {
	Record `yaml:",inline"`
	Path   string `json:"path"`
	Intent string `json:"intent"`
	// Unreadable marks a list placeholder; only ID, Project and Path are known.
	Unreadable bool `json:"unreadable,omitempty" yaml:"-"`
}

// Store uses the existing project resolver. Now is optional and defaults to time.Now.
type Store struct {
	Resolver where.Resolver
	Now      func() time.Time
}

// NewOptions contains only choices supplied by the caller. Who and Role come
// from GRID_WHO and GRID_ROLE; with no Who the task belongs to the machine's
// owner, so a new task never has an empty owner.
type NewOptions struct {
	Cwd, Words, Project, Size, Workflow, Who, Role string
	// Inputs are files copied into the new task's inputs/ folder, each under
	// its base name. New fails before it writes anything when one is missing or
	// is not a file.
	Inputs []string
}

// IntentHead precedes the person's words in intent.md. The Request heading is
// the anchor intent.md#request that Jobs cite for the owner's own words.
const IntentHead = "# Intent\n\n## Request {#request}\n\n"

// Validate checks creation choices before any files are written.
func (o NewOptions) Validate() error {
	if err := checkTaskText("words", o.Words, true); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"workflow", o.Workflow}, {"who", o.Who}, {"role", o.Role},
	} {
		if err := checkTaskText(field.name, field.value, false); err != nil {
			return err
		}
	}

	if strings.TrimSpace(o.Words) == "" {
		return errors.New("task words must not be empty")
	}
	switch o.Size {
	case "", "auto", "small", "normal", "large":
	default:
		return fmt.Errorf("invalid size %q: use auto, small, normal or large", o.Size)
	}
	if o.Workflow != "" && strings.TrimSpace(o.Workflow) == "" {
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

// resolve delegates project selection and caller policy to where.
func (s Store) resolve(cwd, proj string) (where.Result, error) {
	if s.Resolver.GridHome == "" {
		return where.Result{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	if s.Resolver.Machine == "" {
		s.Resolver.Machine = gridhome.Machine(s.Resolver.GridHome)
	}
	r, err := s.Resolver.Resolve(where.Query{Cwd: cwd, Project: proj})
	if errors.Is(err, where.ErrProjectUnregistered) {
		return where.Result{}, where.ErrProjectUnregistered
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
	var doc yaml.Node
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &doc); err != nil {
		return Task{}, fmt.Errorf("task.md front matter: %w", err)
	}
	dropWorkflowList(&doc)
	var front struct {
		Record  `yaml:",inline"`
		Display string `yaml:"display"`
		ULID    string `yaml:"ulid"`
	}
	if err := doc.Load(&front); err != nil {
		return Task{}, fmt.Errorf("task.md front matter: %w", err)
	}
	rec := front.Record
	n, err := number(filepath.Base(path))
	if err != nil {
		return Task{}, err
	}
	id := fmt.Sprintf("T-%02d", n)
	if err := foldID(&rec, front.Display, front.ULID, id); err != nil {
		return Task{}, err
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
	// Intent is the person's words, without the heading New puts before them.
	return Task{Record: rec, Path: path, Intent: strings.TrimPrefix(intent, IntentHead)}, nil
}

// foldID sets rec.ID to the folder's display id (T-NN). The migrated shape
// (contract/README.md, Task IDs) has id: <ULID> with display: T-NN; the legacy
// shape has id: T-NN, or no id, with an optional ulid:. A display or ulid that
// disagrees with the folder or the id is refused.
func foldID(rec *Record, display, stable, folder string) error {
	if feed.IsID(rec.ID) {
		if stable != "" && stable != rec.ID {
			return errors.New("task.md has conflicting stable task ids")
		}
		if display != folder {
			return errors.New("task.md needs a display matching its folder")
		}
		rec.ID = display
		return nil
	}
	if rec.ID == "" {
		rec.ID = folder
	}
	if display != "" && display != rec.ID {
		return errors.New("task.md needs a display matching its folder")
	}
	return nil
}

// dropWorkflowList removes the old plugin's inline step list, so Record.Workflow
// (a template name) stays empty. A duplicate workflow key keeps its decode error.
func dropWorkflowList(doc *yaml.Node) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return
	}
	m := doc.Content[0]
	at := -1
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Kind == yaml.ScalarNode && m.Content[i].Value == "workflow" {
			if at >= 0 {
				return
			}
			at = i
		}
	}
	if at >= 0 && m.Content[at+1].Kind == yaml.SequenceNode {
		m.Content = append(m.Content[:at], m.Content[at+2:]...)
	}
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

// Show resolves a home-wide display id, or the enclosing task when id is empty.
// It never creates project records or other files.
func (s Store) Show(cwd, id string) (Task, error) {
	if s.Resolver.GridHome == "" {
		return Task{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	if id != "" && !idPattern.MatchString(id) {
		return Task{}, fmt.Errorf("invalid task id %q: use T-NN", id)
	}
	if id == "" {
		r, err := s.resolve(cwd, "")
		if err != nil {
			return Task{}, err
		}
		if r.TaskPath == "" {
			return Task{}, errors.New("give a task id (T-NN), or run inside a task folder")
		}
		return readProject(r.TaskPath, r.Project)
	}
	dirs, err := s.Resolver.TasksDirs()
	if err != nil {
		return Task{}, err
	}
	var found, proj string
	for _, dir := range dirs {
		paths, err := folders(dir)
		if err != nil {
			return Task{}, err
		}
		for _, path := range paths {
			name := filepath.Base(path)
			if name == id || strings.HasPrefix(name, id+"-") {
				if found != "" {
					return Task{}, fmt.Errorf("task %s has more than one folder in the Grid home", id)
				}
				found, proj = path, filepath.Base(filepath.Dir(dir))
			}
		}
	}
	if found == "" {
		if s.Resolver.PersonalOK() {
			return Task{}, &where.LookupError{Kind: where.ErrTaskNotFound, Message: fmt.Sprintf("task %s not found in the Grid home", id)}
		}
		return Task{}, where.ErrTaskNotFound
	}
	return readProject(found, proj)
}

func readProject(path, proj string) (Task, error) {
	t, err := Read(path)
	if err == nil && t.Project != proj {
		return Task{}, fmt.Errorf("task %s has project %q, expected %q", path, t.Project, proj)
	}
	return t, err
}

// List returns tasks in numeric id order, with an optional exact state filter.
// Unreadable folders remain as placeholders regardless of the state filter.
func (s Store) List(cwd, proj, state string) ([]Task, error) {
	r, err := s.resolve(cwd, proj)
	if err != nil {
		return nil, err
	}
	paths, err := folders(gridhome.TasksDir(s.Resolver.GridHome, r.Project))
	if err != nil {
		return nil, err
	}
	out := []Task{}
	seen := map[string]bool{}
	for _, path := range paths {
		t, err := readProject(path, r.Project)
		if err != nil {
			n, err := number(filepath.Base(path))
			if err != nil {
				return nil, err
			}
			t = Task{Record: Record{ID: fmt.Sprintf("T-%02d", n), Project: r.Project}, Path: path, Unreadable: true}
		}
		if seen[t.ID] {
			return nil, fmt.Errorf("task %s has a duplicate id", path)
		}
		seen[t.ID] = true
		if t.Unreadable || state == "" || state == t.State {
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
