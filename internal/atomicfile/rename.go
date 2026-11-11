package atomicfile

import "time"

// On Windows a rename over a file that another process holds open without
// FILE_SHARE_DELETE fails until that process lets go, and Go opens files without
// that share mode, so a reader such as the desktop app polling a record makes the
// write fail for a moment. The loop below waits for the reader; it is untagged so
// every OS compiles and tests it. rename_windows.go names the errors it retries
// and rename_unix.go retries none.
//
// The wait is the sum of the pauses: renamePause, 2*renamePause, ... up to
// renameAttempts-1 of them, 750 ms at most.
const (
	renameAttempts = 6
	renamePause    = 50 * time.Millisecond
)

// retryRename calls rename, and while it fails with an error retryable accepts,
// pauses a little longer each time and calls it again, renameAttempts calls at
// most. It returns the last error, or nil.
func retryRename(rename func() error, retryable func(error) bool, pause func(time.Duration)) error {
	err := rename()
	for attempt := 1; err != nil && attempt < renameAttempts && retryable(err); attempt++ {
		pause(time.Duration(attempt) * renamePause)
		err = rename()
	}
	return err
}

// renameRetrying runs rename under this OS's retry rule; WriteRoot uses it for
// its handle's Rename, which Rename cannot reach.
func renameRetrying(rename func() error) error {
	return retryRename(rename, retryableRename, time.Sleep)
}
