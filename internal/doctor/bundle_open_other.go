//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package doctor

import "os"

func openBundleLog(path string) (*os.File, error) { return os.Open(path) }
