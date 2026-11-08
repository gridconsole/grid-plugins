package cli

import "github.com/gridconsole/grid-plugins/internal/braincmd"

func runBrain(args []string, env Env) int {
	return braincmd.Run(args, braincmd.Env{
		Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr,
		Cwd: env.Cwd, Home: env.Home, GridHome: env.GridHome, TempRoots: env.TempRoots,
	})
}
