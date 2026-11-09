//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package task

import (
	"errors"
	"os"
	"syscall"
)

func linkCount(_ string, info os.FileInfo) (uint64, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no link count for " + info.Name())
	}
	return uint64(st.Nlink), nil
}
