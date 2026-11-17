package board

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// JSONL is the board over a task's per-writer JSONL Feed.
type JSONL struct{ Feed feed.Store }

var _ Board = JSONL{}

func (b JSONL) Post(in feed.Input, w feed.Writer) (feed.Record, error) { return b.Feed.Post(in, w) }

func (b JSONL) Read(filter feed.Filter) (feed.ReadResult, error) { return b.Feed.Read(filter) }

// Claim holds feed/.cache/claim.lock across the scan of the whole Feed and the
// append, so two callers of one key, in one process or in two, write one post.
// Only Claim takes this lock, and it takes it before any Feed lock.
func (b JSONL) Claim(key Key, in feed.Input, w feed.Writer) (feed.Record, error) {
	if b.Feed.TaskID == "" {
		id, err := feed.EnsureTaskID(b.Feed.TaskDir)
		if err != nil {
			return feed.Record{}, err
		}
		b.Feed.TaskID = id
	}
	if why := key.fits(b.Feed.TaskID, in); why != "" {
		return feed.Record{}, errors.New(why)
	}
	dir := filepath.Join(b.Feed.Dir(), ".cache")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return feed.Record{}, err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "claim.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return feed.Record{}, err
	}
	// The wait owns f from here: a failed or cancelled lock closes it.
	if err := fslock.LockExclusiveContext(b.Feed.Context, f); err != nil {
		return feed.Record{}, err
	}
	defer f.Close()
	defer fslock.Unlock(f)
	read, err := b.Feed.ReadOnly(feed.Filter{})
	if err != nil {
		return feed.Record{}, err
	}
	for _, e := range read.Records {
		if key.holds(e.Record, in.Type) {
			return feed.Record{}, fmt.Errorf("%w: %s %s attempt %d is %s", ErrClaimed, in.Type, key.Job, key.Attempt, e.ID)
		}
	}
	return b.Feed.Post(in, w)
}
