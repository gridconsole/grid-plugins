package cli

import (
	"errors"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/task"
)

func runHistory(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid history add <line> [--task <task>] [--json]")
		fmt.Fprintln(env.Stdout, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		return 0
	}
	if args[0] != "add" {
		fmt.Fprintln(env.Stderr, "history: expected add")
		return 2
	}
	fs := newFlags("history add", env)
	fs.Usage = func() {
		out := usageOut(args, env)
		fmt.Fprintln(out, "Usage: grid history add <line> [--task <task>] [--json]")
		fmt.Fprintln(out, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		fs.SetOutput(out)
		fs.PrintDefaults()
	}
	id := fs.String("task", "", "task id or folder; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	args, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	if len(args) != 1 {
		fmt.Fprintln(env.Stderr, "history add: expected one line")
		return 2
	}
	t, err := resolveTaskOperand(env, *id)
	if err != nil {
		return taskOperandError(env, "history", err)
	}
	if _, err := taskWriter(env).Note(writeOptions(env, t.ID, "note", args[0])); err != nil {
		if errors.Is(err, task.ErrEditUsage) {
			fmt.Fprintf(env.Stderr, "history: %v\n", err)
			return 2
		}
		return commandError(env, "history", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, map[string]string{"line": args[0]})
	}
	fmt.Fprintln(env.Stdout, "History added")
	return 0
}
