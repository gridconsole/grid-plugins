//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package doctor

import "os"

func openBundleLog(path string) (*os.File, error) { return os.Open(path) }

// Without a supported link-count check, omit logs rather than collect a file
// that could alias private data. Bundle publication also requires fslock support.
func singleLinkBundleLog(info os.FileInfo) bool { return false }
