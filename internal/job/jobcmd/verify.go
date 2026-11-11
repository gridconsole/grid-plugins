package jobcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
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
	var ctx context.Context
	var stop context.CancelFunc
	if e.Context != nil {
		ctx, stop = context.WithCancel(e.Context)
	} else {
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
	defer stop()
	r, err := job.RunVerify(ctx, j, f.dir, job.VerifyOptions{GridHome: e.GridHome, TaskPath: taskPath, Cwd: e.Cwd})
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
