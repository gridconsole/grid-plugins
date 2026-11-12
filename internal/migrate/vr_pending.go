package migrate

import "path/filepath"

// Pending reports whether a migration of home is open: its journal, beside the home,
// exists. Between the two renames of a migration there is no home, and a grid writer
// that makes one then leaves recovery with folders that match no step. Callers that
// would write in the Grid home ask first and say to run grid migrate --apply, which
// finishes or undoes the run from what the disk shows.
//
// The journal is looked for with Lstat, so any file type counts, and its content is
// not read: grid migrate is what judges it. An empty home is never pending.
func Pending(home string) bool {
	if home == "" {
		return false
	}
	return exists(journalPath(filepath.Clean(home)))
}
