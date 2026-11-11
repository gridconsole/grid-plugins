//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly || windows)

package doctor

import "os"

func openBundleLog(path string) (*os.File, error) { return os.Open(path) }

// Without a supported link-count check, omit logs rather than collect a file
// that could alias private data. Bundle publication also requires fslock support.
func singleLinkBundleEntry(path string, info os.FileInfo) bool { return false }

func singleLinkBundleLog(f *os.File, info os.FileInfo) bool { return false }
