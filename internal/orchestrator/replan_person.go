package orchestrator

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guReplanPersonCancelled posts one re-plan Job for each person Job that was
// withdrawn (a control with status cancelled, `grid job cancel`) while other Jobs
// still wait on it with a done edge. gkReplan reads only the Jobs a worker
// cancelled (ExitReason "cancelled"), and a person Job is never assigned to a
// worker (person.go), so it has no worker-exit and the Jobs behind it would wait
// for ever: "waiting for J-1, which was cancelled". The re-plan Job is the one
// gkReplan posts, with no cause: a person Job has no worker-exit, and its title and
// acceptance text already say it was cancelled. It is posted once: the re-plan Job
// already on the Feed (role orchestrator, discovered_from the person Job) is the
// record that says so, read from the snapshot under the task lock. An agent Job
// cancelled by a control alone still gets nothing (gkReplan's rule), and a person
// Job that was answered is Accepted, not Cancelled. It posts a Job and nothing
// more: it refuses, cancels and counts nothing.
func (s Store) guReplanPersonCancelled(snap Snapshot) error {
	for _, it := range snap.Items {
		if !it.IsPerson() {
			continue
		}
		if j := snap.State.Jobs[it.ID]; j == nil || j.Status != contract.Cancelled {
			continue
		}
		waiting := gkWaiting(snap, it.ID)
		if len(waiting) == 0 || gkReplanned(snap, it.ID) {
			continue
		}
		next := gkNextJobID(snap)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: ReplanJobBody(next, it.ID, "", waiting)}, s.Writer); err != nil {
			return err
		}
		// The next withdrawn Job counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}
