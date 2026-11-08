package cli

import "github.com/gridconsole/grid-plugins/internal/evalcmd"

func runEval(args []string, env Env) int {
	return evalcmd.Run(args, evalcmd.Env{Cwd: env.Cwd, Stdout: env.Stdout, Stderr: env.Stderr})
}
