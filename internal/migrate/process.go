package migrate

// ProcessStart returns the existing platform process identity used by migration
// recovery, or an empty string when it cannot be established. Terminal sessions
// use the same identity so a reused PID is never mistaken for the old process.
func ProcessStart(pid int) string { return pidStart(pid) }
