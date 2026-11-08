//go:build !linux && !darwin

package ports

// Unsupported platforms cannot prove that an owner is dead.
func Alive(pid int, group bool) bool { return true }
