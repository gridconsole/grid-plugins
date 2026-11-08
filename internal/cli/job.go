package cli

import (
	"github.com/gridconsole/grid-plugins/internal/job/jobcmd"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runJob(args []string, env Env) int {
	return jobcmd.Run(args, jobcmd.Env{Cwd: env.Cwd, Home: env.Home, GridHome: env.GridHome, Stdout: env.Stdout, Stderr: env.Stderr,
		ResolveTask: func(task string) (string, error) {
			r, e := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: task})
			return r.TaskPath, e
		},
	})
}
