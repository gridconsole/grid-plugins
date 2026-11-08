package jobcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/task"
	"go.yaml.in/yaml/v4"
)

func runVerify(args []string, f flags, e Env) int {
	fail := func(err error, code int) int { return output(e, f, report{Error: err.Error()}, code) }
	if len(args) != 1 || !job.ValidID(args[0]) || f.budget != 0 {
		return fail(errors.New("verify requires J-N [--task T-NN] [--dir path] [--json]"), 2)
	}
	taskPath, err := resolveTask(f.task, e)
	if err != nil {
		return fail(err, 1)
	}
	j, err := loadVerifyJob(taskPath, args[0])
	if err != nil {
		return fail(err, 1)
	}
	dir := e.Cwd
	if f.dir != "" {
		dir = absolute(e.Cwd, f.dir)
	} else if len(j.Worktree) > 0 {
		if len(j.Worktree) > 1 {
			return fail(errors.New("multiple worktrees; specify --dir"), 1)
		}
		for repo, path := range j.Worktree {
			if filepath.IsAbs(path) {
				dir = path
				continue
			}
			if !filepath.IsLocal(path) {
				return fail(errors.New("invalid recorded worktree path; specify --dir"), 1)
			}
			record, err := task.Read(taskPath)
			if err != nil {
				return fail(err, 1)
			}
			if record.Project == "" || filepath.Base(record.Project) != record.Project {
				return fail(errors.New("recorded worktree needs a project; specify --dir"), 1)
			}
			dir = filepath.Join(e.GridHome, "projects", record.Project, "worktrees")
			if repo != "" {
				if filepath.Base(repo) != repo || repo == "." || repo == ".." {
					return fail(errors.New("invalid worktree repository"), 1)
				}
				dir = filepath.Join(dir, repo)
			}
			dir = filepath.Join(dir, path)
		}
	}
	// Task proof pointers are explicit; unprefixed paths refer to the selected dir.
	for i, path := range j.Verify.Paths {
		if strings.HasPrefix(path, "task:") {
			j.Verify.Paths[i], err = job.Within(taskPath, strings.TrimPrefix(path, "task:"))
			if err != nil {
				return fail(err, 1)
			}
		}
	}
	if strings.HasPrefix(j.Verify.Path, "task:") {
		j.Verify.Path, err = job.Within(taskPath, strings.TrimPrefix(j.Verify.Path, "task:"))
		if err != nil {
			return fail(err, 1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r, err := job.RunVerify(ctx, j, dir)
	code := 0
	if !r.Pass || err != nil {
		code = 1
	}
	if j.Verify.Kind == "reviewer-check" && err == nil {
		code = 3
	}
	if f.json {
		if err := json.NewEncoder(e.Stdout).Encode(r); err != nil {
			return 1
		}
	} else {
		total := "unknown"
		if r.Tests != nil {
			total = fmt.Sprint(*r.Tests)
		}
		if _, err := fmt.Fprintf(e.Stdout, "exit: %d\ntests: %s\ncommit: %s\npass: %t\n", r.ExitCode, total, r.Commit, r.Pass); err != nil {
			return 1
		}
		if r.Reason != "" {
			if _, err := fmt.Fprintln(e.Stdout, "reason: "+r.Reason); err != nil {
				return 1
			}
		}
	}
	return code
}

func loadVerifyJob(taskPath, id string) (job.Job, error) {
	path, err := job.Within(taskPath, filepath.Join("jobs", id, "job.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return job.LoadPlan(taskPath, id)
	}
	if err != nil {
		return job.Job{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return job.Job{}, err
	}
	var raw map[string]any
	if err = yaml.Load(b, &raw); err != nil {
		return job.Job{}, err
	}
	var j job.Job
	if err = frontmatter.Into(raw, &j); err != nil {
		return j, err
	}
	if j.ID != id {
		return j, errors.New("saved Job id does not match requested Job")
	}
	return j, nil
}
