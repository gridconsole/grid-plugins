package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/task"
)

type deliveryFiles []string

func (f *deliveryFiles) String() string         { return strings.Join(*f, ", ") }
func (f *deliveryFiles) Set(value string) error { *f = append(*f, value); return nil }

// usageOut is where a flag set prints its usage: stdout when the arguments ask
// for help, stderr on any other parse error so --json gets its error envelope.
func usageOut(args []string, env Env) io.Writer {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "-h", "--h", "-help", "--help":
			return env.Stdout
		}
	}
	return env.Stderr
}

func runDeliver(args []string, env Env) int {
	fs := newFlags("deliver", env)
	fs.Usage = func() {
		out := usageOut(args, env)
		fmt.Fprintln(out, "Usage: grid deliver --outcome <text> [files...] [--file path] [--task <task>] [--by agent] [--dry-run] [--json]")
		fmt.Fprintln(out, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
	}
	o := task.DeliverOptions{Cwd: env.Cwd}
	var files deliveryFiles
	fs.StringVar(&o.Task, "task", "", "task id or folder; defaults to the current task")
	fs.StringVar(&o.Outcome, "outcome", "", "Outcome text")
	fs.StringVar(&o.By, "by", "grid", "delivery author")
	fs.BoolVar(&o.DryRun, "dry-run", false, "print actions without writing")
	fs.Var(&files, "file", "deliverable file to copy; repeatable")
	asJSON := fs.Bool("json", false, "print JSON")
	args, code, stop := parseMixed(fs, args)
	if stop {
		return code
	}
	if o.Outcome == "" && len(args) > 0 {
		o.Outcome = args[0]
		args = args[1:]
	}
	o.Files = append(files, args...)
	if strings.TrimSpace(o.Outcome) == "" {
		fmt.Fprintln(env.Stderr, "deliver: supply --outcome <text>")
		return 2
	}
	t, err := resolveTaskOperand(env, o.Task)
	if err != nil {
		return taskOperandError(env, "deliver", err)
	}
	o.Task = t.ID
	result, err := (task.Store{Resolver: resolver(env)}).Deliver(o)
	if err != nil {
		if errors.Is(err, task.ErrEditUsage) {
			fmt.Fprintf(env.Stderr, "deliver: %v\n", err)
			return 2
		}
		env.recordError(err)
		fmt.Fprintln(env.Stderr, "deliver: "+strings.Join(strings.Fields(err.Error()), " "))
		return 1
	}
	// Waiting proposals get a curator Job on the Feed. The delivery is already
	// written and holds no lock here; a Job that cannot be posted never fails it.
	curator := ""
	if n := len(result.Waiting); n > 0 {
		if o.DryRun {
			if open, err := curatorJobOpen(t.Path); err == nil && !open {
				result.Actions = append(result.Actions, fmt.Sprintf("Post a curator Job for %d proposals", n))
			}
		} else {
			ids := make([]string, 0, n)
			for _, w := range result.Waiting {
				ids = append(ids, w.ID)
			}
			posted, err := postCuratorJob(env, t.Path, result.Task, ids)
			if err != nil {
				fmt.Fprintln(env.Stderr, "deliver: the curator Job was not posted: "+strings.Join(strings.Fields(err.Error()), " "))
			} else if posted != "" {
				curator = posted
				result.Actions = append(result.Actions, fmt.Sprintf("Post curator Job %s for %d proposals", posted, n))
			}
		}
	}
	if *asJSON {
		return writeJSON(env.Stdout, deliverOutput{Delivery: result, CuratorJob: curator})
	}
	if o.DryRun {
		for _, action := range result.Actions {
			fmt.Fprintln(env.Stdout, "Would "+strings.ToLower(action[:1])+action[1:])
		}
		return 0
	}
	fmt.Fprintf(env.Stdout, "%s delivered; waiting for the person\n", result.Task)
	if n := len(result.Waiting); n > 0 {
		fmt.Fprintf(env.Stdout, "%d brain proposals wait for the curator", n)
		if curator != "" {
			fmt.Fprintf(env.Stdout, " (Job %s)", curator)
		}
		fmt.Fprintln(env.Stdout)
	}
	return 0
}
