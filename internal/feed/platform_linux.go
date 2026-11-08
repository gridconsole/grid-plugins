package feed

import (
	"fmt"
	"os"
	"syscall"
)

func syncFile(f *os.File) error { return f.Sync() }
func checkFilesystem(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}
	// FUSE includes sshfs; refuse the whole family rather than assume it is local.
	switch uint64(stat.Type) {
	case 0x6969, 0xff534d42, 0xfe534d42, 0x65735546, 0x517b:
		return fmt.Errorf("Feed requires a local filesystem")
	}
	return nil
}

func writeOnce(f *os.File, data []byte) (int, error) { return syscall.Write(int(f.Fd()), data) }
