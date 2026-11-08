package cli

import (
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runUsage(args []string, env Env) int {
	fs := newFlags("usage", env)
	id := fs.String("task", "", "task id or folder (otherwise the current task or home)")
	asJSON := fs.Bool("json", false, "print recorded totals as JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "Usage: grid usage [--task T-NN] [--json]")
		return 2
	}
	r, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *id})
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	dir := r.TaskPath
	if dir == "" {
		dir = env.GridHome
	}
	rows, err := (session.Store{TaskDir: dir}).Activities()
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	total := session.FoldUsage(rows)
	if *asJSON {
		return writeJSON(env.Stdout, total)
	}
	fmt.Fprintf(env.Stdout, "Recorded usage: %.0f input tokens, %.0f output tokens, %.0f cache read tokens, %.0f cache write tokens, %.0f total tokens, %.0f ms\n", total.Input, total.Output, total.CacheRead, total.CacheWrite, total.Total, total.Milliseconds)
	return 0
}
