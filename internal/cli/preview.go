package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/task"
)

func runPreview(args []string, env Env) int {
	fs := newFlags("preview", env)
	fs.Usage = func() {
		fmt.Fprintln(env.Stdout, "Usage: grid preview [--task <task>] [--spec spec.md] [--by agent] [--dry-run] [--json]")
		fmt.Fprintln(env.Stdout, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		fmt.Fprintln(env.Stdout, "--spec is a file inside the task folder, relative to it.")
	}
	o := task.PreviewOptions{Cwd: env.Cwd}
	fs.StringVar(&o.Task, "task", "", "task id or folder; defaults to the current task")
	fs.StringVar(&o.Spec, "spec", "", "spec file inside the task folder; defaults to spec.md")
	fs.StringVar(&o.By, "by", "grid", "author of the preview")
	fs.BoolVar(&o.DryRun, "dry-run", false, "print actions without writing")
	asJSON := fs.Bool("json", false, "print JSON")
	args, code, stop := parseMixed(fs, args)
	if stop {
		return code
	}
	if len(args) > 0 {
		fmt.Fprintf(env.Stderr, "preview: unexpected argument %q\n", args[0])
		return 2
	}
	t, err := resolveTaskOperand(env, o.Task)
	if err != nil {
		return taskOperandError(env, "preview", err)
	}
	o.Task = t.ID
	result, err := (task.Store{Resolver: resolver(env)}).RaisePreview(o)
	if err != nil {
		if errors.Is(err, task.ErrEditUsage) {
			fmt.Fprintf(env.Stderr, "preview: %v\n", err)
			return 2
		}
		line := strings.Join(strings.Fields(err.Error()), " ")
		if !strings.HasPrefix(line, "Preview blocked:") {
			line = "Preview blocked: " + line
		}
		fmt.Fprintln(env.Stderr, line)
		return 1
	}
	if *asJSON {
		return writeJSON(env.Stdout, result)
	}
	if o.DryRun {
		for _, action := range result.Actions {
			fmt.Fprintln(env.Stdout, "Would "+strings.ToLower(action[:1])+action[1:])
		}
		return 0
	}
	fmt.Fprintf(env.Stdout, "%s preview raised; preview gate waiting\n", result.Task)
	return 0
}
