package cli

import (
	"github.com/gridconsole/grid-plugins/internal/job/jobcmd"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runJob(args []string, env Env) int {
	// cycles and floor are pure graph checks over a JSON file; jobcmd owns the rest.
	if len(args) > 0 && (args[0] == "cycles" || args[0] == "floor") {
		return runJobGraph(args, env)
	}
	return jobcmd.Run(args, jobcmd.Env{Cwd: env.Cwd, Home: env.Home, GridHome: env.GridHome, Machine: env.Machine, Stdout: env.Stdout, Stderr: env.Stderr,
		ResolveTask: func(task string) (string, error) {
			r, e := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: task})
			return r.TaskPath, e
		},
	})
}
