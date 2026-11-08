//go:build !unix && !windows

package migrate

// Without a way to ask about other processes there is no start identity.
func pidStart(pid int) string { return "" }
