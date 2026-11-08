package cli

import "github.com/gridconsole/grid-plugins/internal/question"

func runQuestion(args []string, env Env) int {
	return question.Run(args, question.Env{Cwd: env.Cwd, Resolver: resolver(env), Stdout: env.Stdout, Stderr: env.Stderr})
}
