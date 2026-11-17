package board

import (
	"errors"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// PerPost is the board over a task's per-post files (docs/research/brain-board.md
// B1, the Go API): the Feed facade with its Board set. Post publishes one file under
// the board lock in the machine folder; Claim looks for its key and publishes in one
// hold of that lock.
type PerPost struct{ Feed feed.Store }

var _ Board = PerPost{}

var errNotPerPost = errors.New("a per-post board needs a Feed with its Board set")

func (b PerPost) Post(in feed.Input, w feed.Writer) (feed.Record, error) {
	if b.Feed.Board == nil {
		return feed.Record{}, errNotPerPost
	}
	return b.Feed.Post(in, w)
}

func (b PerPost) Read(filter feed.Filter) (feed.ReadResult, error) {
	if b.Feed.Board == nil {
		return feed.ReadResult{}, errNotPerPost
	}
	return b.Feed.Read(filter)
}

// Claim holds the board lock across the look through the board for a post with the
// key and the publish of the new one (feed.Store.PostUnless), so two callers of one
// key, in one process or in two on this machine, write one post. Across machines one
// attempt names one machine:login; two copies that meet after a sync fold the second
// ignored, "duplicate claim" (contract.FoldBoard).
func (b PerPost) Claim(key Key, in feed.Input, w feed.Writer) (feed.Record, error) {
	if b.Feed.Board == nil {
		return feed.Record{}, errNotPerPost
	}
	if b.Feed.TaskID == "" {
		id, err := feed.TaskID(b.Feed.TaskDir)
		if err != nil {
			return feed.Record{}, err
		}
		b.Feed.TaskID = id
	}
	if why := key.fits(b.Feed.TaskID, in); why != "" {
		return feed.Record{}, errors.New(why)
	}
	r, held, err := b.Feed.PostUnless(in, w, func(r feed.Record) bool { return key.holds(r, in.Type) })
	if err != nil {
		return feed.Record{}, err
	}
	if held != "" {
		return feed.Record{}, fmt.Errorf("%w: %s %s attempt %d is %s", ErrClaimed, in.Type, key.Job, key.Attempt, held)
	}
	return r, nil
}
