package cli

import (
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hook"
)

// runHook answers one hook event. A hook never speaks into a session, so a wrong
// call (no event, an extra argument or flag) prints nothing and exits 0.
func runHook(args []string, env Env) int {
	if len(args) > 0 && (args[0] == "level" || args[0] == "levels" || args[0] == "--help" || args[0] == "-h") {
		return runSettings("hook", args, env)
	}
	if len(args) > 0 && args[0] == "events" {
		return runHookEvents(args[1:], env)
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return 0
	}
	return hook.Run(hook.Options{
		Event: args[0], Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr,
		Resolver: resolver(env), Cwd: env.Cwd, Getenv: env.getenv,
	})
}
