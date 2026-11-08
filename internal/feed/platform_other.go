//go:build !linux && !darwin

package feed

import (
	"fmt"
	"os"
)

func lock(*os.File, bool) error {
	return fmt.Errorf("Feed locking is supported only on Linux and macOS")
}
func unlock(*os.File)              {}
func syncFile(*os.File) error      { return fmt.Errorf("Feed durability is unsupported on this platform") }
func checkFilesystem(string) error { return fmt.Errorf("Feed is supported only on Linux and macOS") }
