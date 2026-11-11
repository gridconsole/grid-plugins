package cli

import (
	"github.com/gridconsole/grid-plugins/internal/braincmd"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func runBrain(args []string, env Env) int {
	return braincmd.Run(args, braincmd.Env{
		ResolveTask:      func(operand string) (task.Task, error) { return resolveTaskOperand(env, operand) },
		TaskOperandError: func(command string, err error) int { return taskOperandError(env, command, err) },
		Resolver:         resolver(env), Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr,
		Cwd: env.Cwd, Home: env.Home, GridHome: env.GridHome, TempRoots: env.TempRoots,
		Role: env.getenv("GRID_ROLE"), RecordError: env.recordError,
	})
}
