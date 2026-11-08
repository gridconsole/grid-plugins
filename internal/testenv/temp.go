// Package testenv owns temporary storage used by test subprocesses.
package testenv

import "testing"

// Temp gives a child process a temporary directory owned by its parent test.
// exec.Cmd uses the last value for duplicate environment keys. Keep the caller's
// slice intact and cover both Unix TMPDIR and Windows TMP/TEMP conventions.
func Temp(t testing.TB, env []string) []string {
	t.Helper()
	dir := t.TempDir()
	child := append([]string(nil), env...)
	return append(child, "TMPDIR="+dir, "TMP="+dir, "TEMP="+dir)
}
