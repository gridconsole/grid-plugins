package workflow

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// taskStepName is the inline reader's step-name rule (taskfile.WorkflowSteps).
var taskStepName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var sizeLetter = map[string]string{"": "", "auto": "", "small": "S", "normal": "N", "large": "L"}

// TaskSteps resolves the steps a task runs from its task.md text: the inline
// list first, else the named template (the project's before the workspace's),
// else the workspace default when no rule needs a choice. Every failure is an
// error; it never returns an empty list without one.
func TaskSteps(r where.Resolver, project, taskText string) ([]taskfile.WorkflowStep, error) {
	_, steps, err := TaskWorkflow(r, project, taskText)
	return steps, err
}

// TaskWorkflow is TaskSteps with the template the steps come from: nil for an
// inline list. The template comes back even when its steps do not resolve for
// the task's size (size auto with size-only steps), so a caller can check the
// definition the task was made with before the size is known.
func TaskWorkflow(r where.Resolver, project, taskText string) (*Template, []taskfile.WorkflowStep, error) {
	fm := taskfile.ParseFrontmatter(taskText)
	if fm == nil || fm.Odd {
		return nil, nil, errors.New("task.md front matter cannot be read")
	}
	raw := fm.Data["workflow"]
	if _, ok := raw.([]any); ok {
		steps, err := inlineSteps(taskText)
		return nil, steps, err
	}
	name, ok := raw.(string)
	if raw != nil && !ok {
		return nil, nil, errors.New("task.md workflow is not a name or a list")
	}
	size, ok := fm.Data["size"].(string)
	if _, has := fm.Data["size"]; has && !ok {
		return nil, nil, errors.New("task.md size is not a word")
	}
	letter, ok := sizeLetter[size]
	if !ok {
		return nil, nil, fmt.Errorf("task size %q is not small, normal, large or auto", size)
	}
	if project == "" {
		return nil, nil, errors.New("the task has no project")
	}
	scope, err := scopeTemplates(r, project)
	if err != nil {
		return nil, nil, err
	}
	// A link outranks every other problem in scope. Otherwise an earlier problem
	// in the workspace file would hide a linked project file from task creation.
	if p, ok := linkedProblem(scope); ok {
		return nil, nil, problemError(p)
	}
	var t *Template
	if name == "" {
		t, err = defaultTemplate(scope)
	} else {
		t, err = namedTemplate(scope, name)
	}
	if err != nil {
		return nil, nil, err
	}
	steps, err := templateSteps(*t, letter, size)
	return t, steps, err
}

func inlineSteps(taskText string) ([]taskfile.WorkflowStep, error) {
	steps := taskfile.WorkflowSteps(taskText)
	if len(steps) == 0 {
		return nil, errors.New("task.md workflow list is malformed")
	}
	return steps, nil
}

// scopeTemplates reads the workspace file and the task's project file. Personal
// tasks live outside projects/, so they read the workspace file alone.
func scopeTemplates(r where.Resolver, project string) (Answer, error) {
	if !strings.EqualFold(project, where.Personal) {
		return read(r, project, false)
	}
	if !r.PersonalOK() {
		return Answer{}, projectSelectionError(r, project)
	}
	if r.GridHome == "" {
		return Answer{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	out := Answer{Templates: []Template{}, Errors: []Problem{}}
	text, rev, err := readFile(gridhome.Real(r.GridHome), "workflows.md")
	if err != nil {
		out.Errors = append(out.Errors, readProblem("", err))
	} else if rev != nil {
		parsed := Parse(text, "")
		out.Templates, out.Errors = parsed.Templates, parsed.Errors
	}
	return out, nil
}

// problemError reads as the problem; its cause stays in the chain.
func problemError(p Problem) error {
	if p.cause != nil {
		return &resolveError{problem: p}
	}
	return fmt.Errorf("the workflow cannot be resolved: %s", p)
}

// resolveError is a problem with a cause. Its text is the same as a problem's
// without one.
type resolveError struct{ problem Problem }

func (e *resolveError) Error() string {
	return fmt.Sprintf("the workflow cannot be resolved: %s", e.problem)
}

func (e *resolveError) Unwrap() error { return e.problem.cause }

// linkedProblem is the first problem in scope that a link caused.
func linkedProblem(scope Answer) (Problem, bool) {
	for _, p := range scope.Errors {
		if p.cause != nil {
			return p, true
		}
	}
	return Problem{}, false
}

func namedTemplate(scope Answer, name string) (*Template, error) {
	for _, p := range scope.Errors {
		if p.Section == nil || strings.EqualFold(*p.Section, name) {
			return nil, problemError(p)
		}
	}
	// A project template, a copy included, wins over the workspace one.
	for _, workspace := range []bool{false, true} {
		for i, t := range scope.Templates {
			if (t.File == labelOf("")) == workspace && strings.EqualFold(t.Name, name) {
				return &scope.Templates[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no workflow is named %q", name)
}

// defaultTemplate picks the workspace default only when nothing could compete:
// a section that failed to parse may hold a rule, so it needs a choice too.
func defaultTemplate(scope Answer) (*Template, error) {
	needChoice := errors.New("task.md names no workflow and the rules need a choice")
	for _, p := range scope.Errors {
		if p.Section == nil {
			return nil, problemError(p)
		}
	}
	if len(scope.Errors) > 0 {
		return nil, needChoice
	}
	for _, t := range scope.Templates {
		if len(t.Applies) > 0 {
			return nil, needChoice
		}
	}
	for i, t := range scope.Templates {
		if t.Default && t.File == labelOf("") {
			// The project's copy of the default wins, as it does for a named workflow.
			for j, c := range scope.Templates {
				if c.File != labelOf("") && c.Overrides != nil && strings.EqualFold(*c.Overrides, t.Name) {
					return &scope.Templates[j], nil
				}
			}
			return &scope.Templates[i], nil
		}
	}
	return nil, errors.New("the workspace has no default workflow")
}

func templateSteps(t Template, letter, size string) ([]taskfile.WorkflowStep, error) {
	var steps []taskfile.WorkflowStep
	seen := map[string]bool{}
	for _, s := range t.Steps {
		if letter == "" && s.Sizes != "SNL" {
			return nil, errors.New("task size is not set; the workflow's steps depend on it")
		}
		if !strings.Contains(s.Sizes, letter) {
			continue
		}
		step := strings.ToLower(s.Name)
		if !taskStepName.MatchString(step) || seen[step] {
			return nil, fmt.Errorf("workflow %q step %q is not a usable step name", t.Name, s.Name)
		}
		seen[step] = true
		steps = append(steps, taskfile.WorkflowStep{Step: step, Kind: s.Kind})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("workflow %q has no steps for a %s task", t.Name, size)
	}
	return steps, nil
}
