package cli

import (
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func runQuestion(args []string, env Env) int {
	return question.Run(args, questionEnv(env))
}

func questionEnv(env Env) question.Env {
	return question.Env{Cwd: env.Cwd, Resolver: resolver(env), Stdout: env.Stdout, Stderr: env.Stderr, Writer: func(control bool) (feed.Writer, error) {
		return feedWriter(env, "", "")
	}}
}
func runAnswer(args []string, env Env) int { return question.RunAnswer(args, questionEnv(env)) }
