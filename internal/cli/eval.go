package cli

import "github.com/gridconsole/grid-plugins/internal/evalcmd"

func runEval(args []string, env Env) int {
	return evalcmd.Run(args, evalcmd.Env{Context: env.Context, Cwd: env.Cwd, GridHome: env.GridHome, Stdout: env.Stdout, Stderr: env.Stderr})
}
