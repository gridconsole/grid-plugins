//go:build !unix && !windows

package migrate

// Without a way to ask about other processes, a lock is never taken as stale; the
// lock error names the file to remove by hand.
func pidAlive(pid int) bool   { return true }
func pidStart(pid int) string { return "" }
