package workflow

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// Answer is the workflows read response. Empty collections encode as arrays.
type Answer struct {
	Files     []File        `json:"files"`
	Templates []Template    `json:"templates"`
	Errors    []Problem     `json:"errors"`
	Policy    []PolicyEntry `json:"policy,omitempty"`
}

// Read returns the workspace file and registered project files, or just the
// workspace and selected project (name or ID). File and parse failures remain
// in Errors; home resolution and project selection failures return an error.
// The caller filters discovery before project records or workflows are opened.
func Read(r where.Resolver, project string) (Answer, error) {
	return read(r, project, false)
}

// ReadPolicy adds validated effective policy without changing the legacy reader.
// Invalid policy fails the entire response before any output is published.
func ReadPolicy(r where.Resolver, project string) (Answer, error) {
	return read(r, project, true)
}

func read(r where.Resolver, project string, withPolicy bool) (Answer, error) {
	files, problems, err := readFiles(r, project)
	if err != nil {
		return Answer{}, err
	}
	out := Answer{Files: files, Templates: []Template{}, Errors: []Problem{}}
	if withPolicy {
		out.Policy, err = effectivePolicy(files, problems)
		if err != nil {
			return Answer{}, err
		}
	}
	failed := make(map[int]error, len(problems))
	for _, p := range problems {
		failed[p.index] = p.err
	}
	for i, file := range files {
		project := ""
		if file.Project != nil {
			project = *file.Project
		}
		if err := failed[i]; err != nil {
			out.Errors = append(out.Errors, readProblem(project, err))
			continue
		}
		if file.Rev == nil {
			continue
		}
		parsed := Parse(file.Text, project)
		out.Templates = append(out.Templates, parsed.Templates...)
		out.Errors = append(out.Errors, parsed.Errors...)
	}
	return out, nil
}

// readProblem is the public problem for a file that could not be read. Only
// fileReason's text is shown. A link is also kept as the cause, so the error
// problemError makes from it still satisfies errors.Is(err, ErrLinkedFile) and
// task creation can refuse it. Other file errors keep no cause.
func readProblem(project string, err error) Problem {
	p := Problem{File: labelOf(project), Message: fileReason(err)}
	return p
}

// parseFrontmatter reads task-local workflow policy using the shared YAML dialect.
func parseFrontmatter(raw []byte) (map[string]any, error) {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("workflow.md needs YAML front matter")
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return nil, fmt.Errorf("workflow.md front matter is not closed")
	}
	var front map[string]any
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
		return nil, fmt.Errorf("workflow.md front matter: %w", err)
	}
	return front, nil
}
