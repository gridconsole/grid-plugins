//go:build !windows

package atomicfile

import "os"

// Rename renames from over to; the rename is atomic and a reader holding the
// target open does not stop it.
func Rename(from, to string) error { return os.Rename(from, to) }

// retryableRename is false: a failed rename here is not one that waiting fixes.
func retryableRename(error) bool { return false }
