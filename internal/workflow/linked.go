package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrLinkedFile marks a protected file with more than one hard link. A link
// planted elsewhere could still write that file, so Grid refuses to read or
// replace it until the other links are removed. The task package uses the same
// value for its own protected files.
var ErrLinkedFile = errors.New("linked_file")

// refuseLinked counts the links of an open workflow file. A count it cannot
// read refuses the file too, so every failure keeps the stricter policy.
func refuseLinked(f *os.File, relative string) error {
	n, err := linkCount(f)
	if err != nil {
		return err
	}
	if n > 1 {
		return fmt.Errorf("%w: %s has %d hard links; remove the other links before Grid reads it", ErrLinkedFile, filepath.ToSlash(relative), n)
	}
	return nil
}
