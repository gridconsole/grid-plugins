//go:build unix

package workflow

import (
	"errors"
	"os"
	"syscall"
)

func linkCount(f *os.File) (uint64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no hard link count for " + info.Name())
	}
	return uint64(st.Nlink), nil
}
