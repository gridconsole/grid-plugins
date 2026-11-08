package cli

// Hooks tolerate every invocation error without emitting diagnostics.
func runSilentHook(args []string, env Env) int {
	if len(args) != 1 {
		return 0
	}
	return runHook(args, env)
}
