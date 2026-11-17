package hook

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// hookFeed is the store a hook reads one task's posts from. A task folder with a
// board/ folder is on a per-post board, whose caches are in the machine folder; a
// task with only feed/ keeps its JSONL Feed, so a home that is not migrated reads as
// before. ok is false when the task has neither, or has no stable id: it has nothing
// this store can read.
func hookFeed(o Options, dir string) (store feed.Store, ok bool, err error) {
	switch {
	case feed.BoardPresent(dir):
	case dirExists(filepath.Join(dir, "feed")):
	default:
		return feed.Store{}, false, nil
	}
	id, err := feed.TaskID(dir)
	if err != nil {
		// A task with no stable id has no Feed this store can read.
		return feed.Store{}, false, nil
	}
	store = feed.Store{TaskDir: dir, TaskID: id}
	if feed.BoardPresent(dir) {
		getenv := o.Getenv
		if getenv == nil {
			getenv = os.Getenv
		}
		home := o.Resolver.Home
		if home == "" {
			// jobHasResult has no options: it is the process's own environment.
			home, _ = os.UserHomeDir()
		}
		store.Machine = machinefolder.Resolve(machinefolder.Env{Getenv: getenv, Home: home})
		store = store.Boarded("", "")
	}
	return store, true, nil
}

// dirExists reports that path is not missing, whatever it is: the hooks asked only
// that feed/ was not absent, and went on when it could not be told.
func dirExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}
