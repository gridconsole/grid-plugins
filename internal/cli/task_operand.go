package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

var errTaskOperand = errors.New("invalid task operand")
var taskOperandName = regexp.MustCompile(`^(T-[0-9]+)(?:-.+)?$`)

// resolveTaskOperand preserves home-wide ID lookup, but requires an explicit
// folder to name exactly one task directory. Enumeration and final resolution
// both use the existing caller-policy choke point.
func resolveTaskOperand(env Env, operand string) (task.Task, error) {
	if env.GridHome == "" {
		return task.Task{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	r := resolver(env)
	for _, part := range strings.FieldsFunc(operand, func(c rune) bool { return c == '/' || c == '\\' }) {
		if part == ".." {
			return task.Task{}, fmt.Errorf("%w: parent traversal is not allowed", errTaskOperand)
		}
	}
	dirs, err := r.TasksDirs()
	if err != nil {
		return task.Task{}, err
	}
	path := ""
	match := taskOperandName.FindStringSubmatch(operand)
	switch {
	case operand == "":
		scope, err := r.Resolve(where.Query{Cwd: env.Cwd})
		if err != nil {
			return task.Task{}, err
		}
		path = scope.TaskPath
		if path == "" {
			return task.Task{}, errors.New("give a task id or folder, or run inside a task folder")
		}
	case match != nil && match[1] == operand:
		t, err := taskWriter(env).Show(env.Cwd, operand)
		if err != nil {
			return task.Task{}, err
		}
		path = t.Path
	case match != nil:
		for _, dir := range dirs {
			candidate := filepath.Join(dir, operand)
			if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return task.Task{}, err
			}
			if path != "" {
				return task.Task{}, errors.New("task has more than one folder in the Grid home")
			}
			path = candidate
		}
		if path == "" {
			return task.Task{}, errors.New("task not found")
		}
	case !strings.ContainsAny(operand, `/\`) && !strings.HasPrefix(operand, "."):
		return taskWriter(env).Show(env.Cwd, operand)
	default:
		path = operand
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return task.Task{}, fmt.Errorf("%w: invalid folder path", errTaskOperand)
	}
	// Check both the lexical parent and the canonical parent. In particular,
	// resolving a symlink must not move the task into another project's root.
	var expectedRoot string
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return task.Task{}, err
		}
		rel, err := filepath.Rel(r.GridHome, dir)
		if err != nil {
			return task.Task{}, err
		}
		canonical := filepath.Join(gridhome.Real(r.GridHome), rel)
		if filepath.Dir(path) == absDir || filepath.Dir(path) == canonical {
			expectedRoot = canonical
			break
		}
	}
	if expectedRoot == "" {
		return task.Task{}, fmt.Errorf("%w: folder must be directly inside an accessible project's tasks directory", errTaskOperand)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return task.Task{}, err
	}
	if filepath.Dir(real) != expectedRoot || !taskOperandName.MatchString(filepath.Base(real)) {
		return task.Task{}, fmt.Errorf("%w: folder escapes its tasks directory", errTaskOperand)
	}
	scope, err := r.Resolve(where.Query{Cwd: env.Cwd, Task: real})
	if err != nil {
		return task.Task{}, err
	}
	// Reuse Show's record validation and duplicate-ID check, so a folder cannot
	// choose a different target from the ID subsequently passed to a writer.
	id := taskOperandName.FindStringSubmatch(scope.Task)[1]
	t, err := taskWriter(env).Show(env.Cwd, id)
	if err != nil {
		return task.Task{}, err
	}
	if gridhome.Real(t.Path) != real || t.Project != scope.Project {
		return task.Task{}, errors.New("task folder does not match the resolved task")
	}
	return t, nil
}

func taskOperandError(env Env, command string, err error) int {
	fmt.Fprintf(env.Stderr, "%s: %v\n", command, err)
	if errors.Is(err, errTaskOperand) {
		return 2
	}
	return 1
}
