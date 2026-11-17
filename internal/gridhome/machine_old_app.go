package gridhome

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// oldAppFolder is where SetAsideOldMachine puts the older desktop app's machine
// folder; a number follows it when that name is taken.
const oldAppFolder = "machine.old-app"

// renameFolder is a variable so a test can fail the move the way Windows does while
// the older app still holds a file in the folder.
var renameFolder = os.Rename

// OldAppMachine reports whether <dir>/machine is a folder: the older desktop app
// kept its config, host.lock, licence and keys there, and this grid keeps the
// machine id in a small file at the same place. A link is not a folder here.
func OldAppMachine(dir string) bool {
	st, err := os.Lstat(filepath.Join(dir, "machine"))
	return err == nil && st.IsDir()
}

// SetAsideOldMachine is the one place every writer of the machine id file calls
// first: when <dir>/machine is the older app's folder it moves it to
// <dir>/machine.old-app (machine.old-app-2 and so on when that exists) and returns
// the new path, so the writer then makes the id file as on a fresh home. It never
// deletes or overwrites, and it touches nothing else: a missing file, an id file, a
// hand-edited file and a link all give "" and no error.
//
// The writers hold different locks, so the move takes its own and looks again under
// it; otherwise a second writer could move the id file the first had just made.
// Moving a folder with open files is safe on macOS and Linux. On Windows the older
// app may still hold host.lock; the error then names the folder and says to close
// that app.
func SetAsideOldMachine(dir string) (string, error) {
	if !OldAppMachine(dir) {
		return "", nil
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".machine-move.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return "", err
	}
	defer fslock.Unlock(lock)
	if !OldAppMachine(dir) { // another process moved it while this one waited
		return "", nil
	}
	from := filepath.Join(dir, "machine")
	for n := 1; n < 1000; n++ {
		to := filepath.Join(dir, oldAppFolder)
		if n > 1 {
			to += "-" + strconv.Itoa(n)
		}
		if _, err := os.Lstat(to); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if err := renameFolder(from, to); err != nil {
			var link *os.LinkError
			if errors.As(err, &link) {
				err = link.Err
			}
			return "", fmt.Errorf("cannot move the older Grid app's folder %s aside (%v); close the older Grid app and run the command again", from, err)
		}
		return to, nil
	}
	return "", fmt.Errorf("%s is the older Grid app's folder and every name to move it to is taken", filepath.Join(dir, "machine"))
}
