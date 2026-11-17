package job

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// WorktreePath resolves a recorded worktree for verification and orchestration.
// A named worktree is flat under projects/<project>/worktrees; repository map
// keys do not add a directory. The @sha suffix records a commit, not a folder.
// Absolute paths remain supported for callers with an existing checkout.
func WorktreePath(gridHome, project, recorded string) (string, error) {
	return WorktreePathIn(nil, gridHome, project, recorded)
}

// WorktreePathIn is WorktreePath for a Grid that keeps its worktrees in a machine
// folder: dirFor gives a project's worktrees folder, and the recorded name is a
// folder in it. A nil dirFor is the old place, projects/<project>/worktrees in the
// Grid home, so WorktreePath answers as it always has.
func WorktreePathIn(dirFor func(gridHome, project string) (string, error), gridHome, project, recorded string) (string, error) {
	if filepath.IsAbs(recorded) {
		return recorded, nil
	}
	name, _, _ := strings.Cut(recorded, "@")
	component := func(s string) bool {
		return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`) && filepath.IsLocal(s)
	}
	if !component(name) {
		return "", errors.New("invalid recorded worktree name; specify --dir")
	}
	if gridHome == "" || !component(project) {
		return "", errors.New("recorded worktree needs a Grid home and project; specify --dir")
	}
	if dirFor != nil {
		base, err := dirFor(gridHome, project)
		if err != nil {
			return "", err
		}
		return filepath.Join(base, name), nil
	}
	return filepath.Join(gridHome, "projects", project, "worktrees", name), nil
}

func resolveVerifyPaths(j Job, dir string, o VerifyOptions) (Job, string, error) {
	cwd := o.Cwd
	if cwd == "" {
		cwd = "."
	}
	if dir != "" {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
	} else {
		if len(j.Worktree) > 1 {
			return j, "", errors.New("multiple worktrees; specify --dir")
		}
		dir = cwd
		for _, recorded := range j.Worktree {
			project := ""
			if !filepath.IsAbs(recorded) {
				record, err := task.Read(o.TaskPath)
				if err != nil {
					return j, "", err
				}
				project = record.Project
			}
			var err error
			dir, err = WorktreePathIn(o.WorktreesDir, o.GridHome, project, recorded)
			if err != nil {
				return j, "", err
			}
		}
	}
	proofPath := func(path string) (string, error) {
		if !strings.HasPrefix(path, "task:") {
			return path, nil
		}
		if o.TaskPath == "" {
			return "", errors.New("task: proof path requires a task folder")
		}
		return Within(o.TaskPath, strings.TrimPrefix(path, "task:"))
	}
	// Copy the slice so resolving proof paths never mutates the caller's Job.
	j.Verify.Paths = append([]string(nil), j.Verify.Paths...)
	for i, path := range j.Verify.Paths {
		p, err := proofPath(path)
		if err != nil {
			return j, "", err
		}
		j.Verify.Paths[i] = p
	}
	var err error
	j.Verify.Path, err = proofPath(j.Verify.Path)
	return j, dir, err
}
