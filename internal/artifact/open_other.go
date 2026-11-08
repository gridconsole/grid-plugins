//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package artifact

import "os"

// openSource opens path; the caller checks on the opened handle that it is a regular file.
func openSource(path string) (*os.File, error) { return os.Open(path) }
