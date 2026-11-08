//go:build !linux && !darwin && !windows

package persongate

import (
	"errors"
	"os"
)

func private(os.FileInfo) error { return errors.New("unsupported person directory") }
func openFile(*os.Root, string, int) (*os.File, error) {
	return nil, errors.New("unsupported person directory")
}
func lockFile(*os.File) error { return errors.New("unsupported person lock") }
func unlockFile(*os.File)     {}

func makeDir(path string) error { return os.Mkdir(path, 0700) }
func checkDir(*os.Root) error   { return nil }
