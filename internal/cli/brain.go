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
		Role: env.getenv("GRID_ROLE"), Member: brainMember(env), RecordError: env.recordError,
		Task: env.getenv("GRID_TASK"), Job: env.getenv("GRID_JOB"), Session: brainSession(env),
		SessionTask: brainSessionTask(env),
	})
}

// brainSessionTask is the session task in the order the hooks read it:
// GRID_TASK, else GRID_FEED_TASK.
func brainSessionTask(env Env) string {
	if v := env.getenv("GRID_TASK"); v != "" {
		return v
	}
	return env.getenv("GRID_FEED_TASK")
}

// brainSession is the session id in the order grid task attach reads it:
// GRID_SESSION_ID, else CLAUDE_SESSION_ID.
func brainSession(env Env) string {
	if v := env.getenv("GRID_SESSION_ID"); v != "" {
		return v
	}
	return env.getenv("CLAUDE_SESSION_ID")
}

// brainMember is the person the host runs the plugin for, in the order grid task
// new reads it: GRID_WHO, else GRID_ACTOR, else GRID_MEMBER. The hosted app sets
// all three to the same person.
func brainMember(env Env) string {
	for _, key := range []string{"GRID_WHO", "GRID_ACTOR", "GRID_MEMBER"} {
		if v := env.getenv(key); v != "" {
			return v
		}
	}
	return ""
}
