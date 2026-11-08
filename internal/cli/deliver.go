package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/task"
)

type deliveryFiles []string

func (f *deliveryFiles) String() string         { return strings.Join(*f, ", ") }
func (f *deliveryFiles) Set(value string) error { *f = append(*f, value); return nil }

func runDeliver(args []string, env Env) int {
	fs := newFlags("deliver", env)
	fs.Usage = func() {
		fmt.Fprintln(env.Stdout, "Usage: grid deliver --outcome <text> [files...] [--file path] [--task T-NN] [--by agent] [--dry-run] [--json]")
	}
	o := task.DeliverOptions{Cwd: env.Cwd}
	var files deliveryFiles
	fs.StringVar(&o.Task, "task", "", "task id; defaults to the current task")
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
	result, err := (task.Store{Resolver: resolver(env)}).Deliver(o)
	if err != nil {
		if errors.Is(err, task.ErrEditUsage) {
			fmt.Fprintf(env.Stderr, "deliver: %v\n", err)
			return 2
		}
		line := strings.Join(strings.Fields(err.Error()), " ")
		if !strings.HasPrefix(line, "Deliver blocked:") {
			line = "Deliver blocked: " + line
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
	fmt.Fprintf(env.Stdout, "%s delivered; ship gate waiting\n", result.Task)
	return 0
}
