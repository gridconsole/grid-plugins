//go:build !unix

package machinefolder

import "os"

// groupOf is unknown here: Windows has no owning group id to compare.
func groupOf(os.FileInfo) (string, bool) { return "", false }
