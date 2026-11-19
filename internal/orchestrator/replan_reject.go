package orchestrator

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// guReplanRejected posts one orchestrator Job for each Job that was closed
// Rejected while other Jobs still wait on it with a done or children-done edge
// and its continues chain is dead (an open or accepted chain gets none). A rejected Job
// never releases a done edge (jobgraph.Released), and no other pass reacts to
// it, so those Jobs would read "waiting for J-1, which was rejected" for ever.
// It is gkReplan for the Rejected status: the same record says the Job was
// posted (role orchestrator, discovered_from the rejected Job), so it is posted
// once. A Rejected Job is always closed by a control, so there is no
// worker-exit to look for. It posts a Job and nothing more: it refuses, cancels
// and counts nothing.
func (s Store) guReplanRejected(snap Snapshot) error {
	for _, id := range ids(snap) {
		if j := snap.State.Jobs[id]; j.Status != contract.Rejected {
			continue
		}
		waiting := gkDeadWaiting(snap, id)
		if len(waiting) == 0 || gkReplanned(snap, id) {
			continue
		}
		next := gkNextJobID(snap)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: ReplanRejectJobBody(next, id, waiting)}, s.Writer); err != nil {
			return err
		}
		// The next rejected Job counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// ReplanRejectJobBody is the job-record of the orchestrator Job that re-plans
// after a Job was rejected while the Jobs in waiting hold a done edge to it. It
// is the body ReplanJobBody builds for a cancel, with no cause (a rejected Job
// has no worker-exit code) and a title and acceptance text that say rejected.
// Like it, it carries no job: pointer.
func ReplanRejectJobBody(id, rejected string, waiting []string) map[string]any {
	body := ReplanJobBody(id, rejected, "", waiting)
	reason := jobgraph.DoneReason(rejected, contract.Rejected)
	each := make([]string, 0, len(waiting))
	for _, w := range waiting {
		each = append(each, w+" ("+reason+")")
	}
	text := fmt.Sprintf("No Job waits on %s any more: each of %s is cancelled with a reason or replaced by a new Job that does not wait on %s; the work of %s is posted again as a new Job that `grid job lint` passes when the task still needs it; a result is posted with `grid job result`.",
		rejected, strings.Join(each, ", "), rejected, rejected)
	body["title"] = fmt.Sprintf("Re-plan after %s was rejected: %s wait on it", rejected, strings.Join(waiting, ", "))
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "owner"}}
	return body
}
