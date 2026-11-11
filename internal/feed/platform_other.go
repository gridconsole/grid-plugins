//go:build !linux && !darwin && !windows

package feed

import (
	"fmt"
	"os"
)

func syncFile(*os.File) error      { return fmt.Errorf("Feed durability is unsupported on this platform") }
func checkFilesystem(string) error { return fmt.Errorf("Feed is supported only on Linux and macOS") }

func writeOnce(f *os.File, data []byte) (int, error) { return f.Write(data) }
