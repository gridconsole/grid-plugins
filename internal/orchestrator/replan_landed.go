package orchestrator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// lwReplanLandedTitle is the title of the orchestrator Job that re-plans the Jobs
// waiting for the accepted target to land. The title is the record that it was
// posted, one for each target.
func lwReplanLandedTitle(target string) string {
	return fmt.Sprintf("Re-plan after %s was accepted: Jobs wait for it to land", target)
}

// replanLandedJobBody is the job-record of the orchestrator Job that re-plans the
// Jobs in waiting, which wait for the accepted target to land. It is the body
// ReplanJobBody builds, with no Job it was discovered from, so guReplanAskOwner
// asks no person when it ends, and a title and acceptance text that name target
// and waiting. Like it, it carries no job: pointer.
func replanLandedJobBody(id, target string, waiting []string) map[string]any {
	list := strings.Join(waiting, ", ")
	text := fmt.Sprintf("%s is accepted and Jobs wait for it to land (%s), but no Job lands it: such a wait is met only by a landed record, which an agent posts as a task write with `grid feed post --type landed --job %s --lease task --lease-epoch \"$GRID_TASK_EPOCH\" --body '{\"main_sha\":\"<sha>\"}'`. Either a new Job lands the change of %s as the project lands work (merged and installed) and then posts that record, so %s go on as they are; or each of %s that does not need the land is cancelled with a reason or replaced by a new Job that does not wait for it, and each Job that waits on one cancelled is put behind its replacement; a result is posted with `grid job result`.",
		target, list, target, target, list, list)
	body := ReplanJobBody(id, "", "", nil)
	body["title"] = lwReplanLandedTitle(target)
	body["discovered_from"] = nil
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "owner"}}
	return body
}

// lwLandWaiters lists, by accepted Job with no landed record, the Posted or Ready
// Jobs that wait for it by an after edge of kind landed (jobgraph Ready). A Held
// Job's landed wait is left to wait, as D-job-hold and jh_hold_tick_test pin it.
func lwLandWaiters(snap Snapshot) map[string][]string {
	out := map[string][]string{}
	for _, id := range ids(snap) {
		if st := snap.State.Jobs[id].Status; st != contract.Posted && st != contract.Ready {
			continue
		}
		for _, it := range snap.Items {
			if it.ID != id {
				continue
			}
			for _, e := range it.After {
				t := snap.State.Jobs[e.Job]
				if e.Kind == jobgraph.Landed && t != nil && t.Status == contract.Accepted && !snap.Landed[e.Job] && !slices.Contains(out[e.Job], id) {
					out[e.Job] = append(out[e.Job], id)
				}
			}
		}
	}
	return out
}

// lwReplanLanded posts one orchestrator Job for each accepted Job that Posted or
// Ready Jobs wait for to land while it has no landed record: no step of a
// plan posts that record by itself, so without the re-plan they wait for ever
// (GridV2 T-12). It fires the tick the Job is accepted, which is right only while
// nothing lands a Job by itself. The re-plan with that title on the Feed is the
// record, so one that ends without a fix is not posted again and no person is
// asked. It posts a Job and nothing more.
func (s Store) lwReplanLanded(snap Snapshot) error {
	waiters := lwLandWaiters(snap)
	targets := make([]string, 0, len(waiters))
	for t := range waiters {
		targets = append(targets, t)
	}
	slices.SortFunc(targets, byJobNumber)
	for _, target := range targets {
		if lwReplanned(snap, target) {
			continue
		}
		next := gkNextJobID(snap)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: replanLandedJobBody(next, target, waiters[target])}, s.Writer); err != nil {
			return err
		}
		// The next target counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// lwReplanned reports whether the Feed holds the re-plan of the Jobs waiting for
// target to land.
func lwReplanned(snap Snapshot, target string) bool {
	title := lwReplanLandedTitle(target)
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) && it.Title == title {
			return true
		}
	}
	return false
}
