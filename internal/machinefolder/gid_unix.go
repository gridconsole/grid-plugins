//go:build unix

package machinefolder

import (
	"os"
	"strconv"
	"syscall"
)

// groupOf returns the group id that owns the file, as text.
func groupOf(info os.FileInfo) (string, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return strconv.FormatUint(uint64(st.Gid), 10), true
}
