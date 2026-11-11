package cli

import (
	"github.com/gridconsole/grid-plugins/internal/job/jobcmd"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runJob(args []string, env Env) int {
	// cycles is a pure graph check over a JSON file; jobcmd owns the rest.
	if len(args) > 0 && args[0] == "cycles" {
		return runJobGraph(args, env)
	}
	r := resolver(env)
	// Local CLI sessions have no host role; make their member context explicit.
	if r.Caller.Role == "" {
		r.Caller.Role = "member"
		if r.Caller.Member == "" {
			r.Caller.Role = "owner"
		}
	}
	return jobcmd.Run(args, jobcmdEnv(env, r))
}

// jobcmdEnv is the environment grid job commands run in, for resolver r.
func jobcmdEnv(env Env, r where.Resolver) jobcmd.Env {
	return jobcmd.Env{Cwd: env.Cwd, Home: env.Home, GridHome: env.GridHome, Machine: env.Machine, Stdout: env.Stdout, Stderr: env.Stderr,
		Resolver: r,
		ResolveTask: func(task string) (string, error) {
			result, err := r.Resolve(where.Query{Cwd: env.Cwd, Task: task})
			return result.TaskPath, err
		},
	}
}
