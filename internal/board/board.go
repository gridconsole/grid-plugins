// Package board is where a task's posts are written and read. Board is the
// interface; JSONL is its adapter over the per-writer JSONL Feed (internal/feed).
// The per-post board adapter is added by the board store lane (brain-board.md B1).
package board

import (
	"errors"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// ErrClaimed is the error of a Claim whose key a post on the board already has,
// whoever wrote it and from whichever process.
var ErrClaimed = errors.New("claim key already exists")

// Key names one claim: a Job's attempt in a task, plus the firing of a recurring
// Job when given (run-jobs-launcher.md Q1 "Claim").
type Key struct {
	Task, Job string
	Attempt   int
	Firing    string
}

// Board is one task's posts.
type Board interface {
	// Post writes one post.
	Post(in feed.Input, w feed.Writer) (feed.Record, error)
	// Read returns the posts with their reader verdicts.
	Read(filter feed.Filter) (feed.ReadResult, error)
	// Claim writes the post only when no post with its key exists: create only
	// if absent. A post with the key is one of the same type, task, Job and
	// attempt, and the same body firing when the key names one. Otherwise it
	// returns an error that wraps ErrClaimed and names that post.
	Claim(key Key, in feed.Input, w feed.Writer) (feed.Record, error)
}

// fits says why a claim post would not carry its key, or "" when it does. A post
// that does not carry its key could never be found by it, so a second Claim of
// the key would succeed.
func (k Key) fits(task string, in feed.Input) string {
	switch {
	case k.Task == "" || k.Job == "" || k.Attempt < 1 || in.Type == "":
		return "a claim needs a task, a Job, an attempt and a post type"
	case k.Task != task:
		return fmt.Sprintf("claim key names task %s, the board is task %s", k.Task, task)
	case in.Job != k.Job || in.Attempt != k.Attempt:
		return fmt.Sprintf("claim post names %s attempt %d, the key %s attempt %d", in.Job, in.Attempt, k.Job, k.Attempt)
	case k.Firing != "" && in.Body["firing"] != k.Firing:
		return fmt.Sprintf("claim post body has no firing %s", k.Firing)
	}
	return ""
}

// holds says whether the post r of type typ has this key.
func (k Key) holds(r feed.Record, typ string) bool {
	if r.Type != typ || r.Task != k.Task || r.Job == nil || *r.Job != k.Job || r.Attempt == nil || *r.Attempt != k.Attempt {
		return false
	}
	return k.Firing == "" || r.Body["firing"] == k.Firing
}
