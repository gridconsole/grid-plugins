package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/agent"
)

// runAgentCommand adds readiness checks without changing the settings commands.
func runAgentCommand(args []string, env Env) int {
	if len(args) > 0 && args[0] == "check" {
		return runAgentCheck(args[1:], env, agent.Env{PATH: env.getenv("PATH"), Home: env.Home})
	}
	if len(args) > 0 && args[0] == "sync" {
		return runAgentSync(args[1:], env)
	}
	if len(args) > 0 && args[0] == "budget" {
		return runAgentBudget(args[1:], env)
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid agent check [--json]\n       grid agent sync [--json]\n       grid agent budget show|apply|undo")
	}
	return runAgent(args, env)
}

func runAgentCheck(args []string, env Env, probe agent.Env) int {
	fs := newFlags("agent check", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	reports := agent.Check(ctx, probe)
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Agents []agent.Report `json:"agents"`
		}{reports})
	}
	for _, r := range reports {
		line := fmt.Sprintf("%s found=%t", r.Agent, r.Found)
		if r.Path != "" {
			line += fmt.Sprintf(" path=%q", r.Path)
		}
		if r.Version != "" {
			line += " version=" + r.Version
		}
		line += " min=" + r.Min
		if r.SignedIn != nil {
			line += fmt.Sprintf(" signedIn=%t", *r.SignedIn)
		}
		if r.Fix != "" {
			line += fmt.Sprintf(" fix=%q", r.Fix)
		}
		if _, err := fmt.Fprintln(env.Stdout, line); err != nil {
			return exitCode(err)
		}
	}
	return 0
}
