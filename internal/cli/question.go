package cli

import (
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func runQuestion(args []string, env Env) int {
	if len(args) > 0 && args[0] == "answer" {
		return runAnswer(args[1:], env)
	}
	return question.Run(args, questionEnv(env))
}

func questionEnv(env Env) question.Env {
	return question.Env{AgentShell: func() bool { return agentShell(env) }, Cwd: env.Cwd, Resolver: resolver(env), Stdout: env.Stdout, Stderr: env.Stderr, Writer: func(control bool) (feed.Writer, error) {
		if control && env.getenv("GRID_LEASE") != "task" && agentShell(env) {
			// Preserve the withdrawal refusal before feedWriter checks a worker
			// identity that an agent shell cannot use for a person withdrawal.
			role := env.getenv("GRID_ROLE")
			if role == "" {
				role = "member"
			}
			return feed.Writer{Author: feed.Author{ID: env.getenv("GRID_MEMBER"), Role: role}}, nil
		}
		if control {
			return feedWriter(env, env.getenv("GRID_LEASE"), env.getenv("GRID_LEASE_EPOCH"))
		}
		return feedWriter(env, "", "")
	}}
}
func runAnswer(args []string, env Env) int { return question.RunAnswer(args, questionEnv(env)) }
