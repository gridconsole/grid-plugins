package cli

import (
	"fmt"
	"time"

	"github.com/gridconsole/grid-plugins/internal/task"
)

func runHistory(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid history add <line> [--task T-NN] [--json]")
		return 0
	}
	if args[0] != "add" {
		fmt.Fprintln(env.Stderr, "history: expected add")
		return 2
	}
	fs := newFlags("history add", env)
	id := fs.String("task", "", "task id; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	args, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	if len(args) != 1 {
		fmt.Fprintln(env.Stderr, "history add: expected one line")
		return 2
	}
	t, err := (task.Store{Resolver: resolver(env)}).Show(env.Cwd, *id)
	if err != nil {
		return commandError(env, "history", err)
	}
	if err := task.AppendHistory(t.Path, args[0], time.Now()); err != nil {
		return commandError(env, "history", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, map[string]string{"line": args[0]})
	}
	fmt.Fprintln(env.Stdout, "History added")
	return 0
}
