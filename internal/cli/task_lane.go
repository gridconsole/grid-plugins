package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// runTaskLane is grid task lane <task> <lane>: it records the board lane a
// person moved a task to (the task page's state menu) in task.md.
func runTaskLane(args []string, env Env) int {
	fs := newFlags("task lane", env)
	asJSON := fs.Bool("json", false, "print JSON")
	project := fs.String("project", "", "project name or id; the task must be in it")
	fs.Usage = func() {
		fmt.Fprint(env.Stderr, taskUsage)
		fs.PrintDefaults()
	}
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 2 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	// An unregistered project is answered first, as for task list and task new.
	if *project != "" {
		if err := taskWriter(env).CheckProject(env.Cwd, *project); err != nil {
			env.recordError(err)
			fmt.Fprintf(env.Stderr, "grid task lane: %v\n", err)
			return 1
		}
	}
	if !task.ValidLane(pos[1]) {
		fmt.Fprintf(env.Stderr, "grid task lane: lane must be one of %s, not %q\n", strings.Join(task.Lanes, ", "), pos[1])
		return 2
	}
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid task lane", err)
	}
	result, err := taskWriter(env).SetLane(task.LaneOptions{
		WriteOptions: writeOptions(env, resolved.ID, "lane", ""), Lane: pos[1], Project: *project})
	if err != nil {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid task lane: %v\n", err)
		switch {
		case errors.Is(err, task.ErrEditUsage):
			return 2
		case errors.Is(err, task.ErrEditRejected):
			return 3
		default:
			return 1
		}
	}
	if *asJSON {
		type laneTask struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Lane   string `json:"lane"`
		}
		return writeJSON(env.Stdout, struct {
			State string   `json:"state"`
			Task  laneTask `json:"task"`
		}{"applied", laneTask{result.Task, result.ToState, result.Lane}})
	}
	fmt.Fprintf(env.Stdout, "%s lane: %s (%s)\n", result.Task, result.Lane, result.Note)
	return 0
}
