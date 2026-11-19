package orchestrator

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jhReplanHeldTitle is the title of the orchestrator Job that re-plans the Held
// Job held whose job wait on wait can no longer be met. The title is the record
// that the re-plan was posted: it names the held Job and the wait it was posted
// for, which is never the re-plan of a dead Job (gkReplanned, discovered_from).
func jhReplanHeldTitle(held, wait string) string {
	return fmt.Sprintf("Re-plan %s: its wait on %s can no longer be met", held, wait)
}

// ReplanHeldJobBody is the job-record of the orchestrator Job that re-plans the
// Held Job held, whose job wait on wait (row, may be empty) can no longer be met:
// wait and every Job that continues it were rejected or cancelled, end the last
// of them with endStatus. It is the body ReplanJobBody builds, with no Job it was
// discovered from (wait may already have a re-plan of its own) and a title and
// acceptance text that name held, wait and end. Like it, it carries no job:
// pointer.
func ReplanHeldJobBody(id, held, wait, row, end string, endStatus contract.Status) map[string]any {
	on := wait
	if row != "" {
		on += " (row " + row + ")"
	}
	why := fmt.Sprintf("%s was %s with no Job continuing it", wait, endStatus)
	if end != wait {
		why = fmt.Sprintf("%s and every Job that continues it ended: the last, %s, was %s", wait, end, endStatus)
	}
	text := fmt.Sprintf("%s is held on %s, and %s, so the wait can no longer be met: %s is cancelled with a reason or replaced by a new Job that does not wait on %s; the work of %s is posted again as a new Job that `grid job lint` passes when the task still needs it; a result is posted with `grid job result`.",
		held, on, why, held, wait, wait)
	body := ReplanJobBody(id, "", "", nil)
	body["title"] = jhReplanHeldTitle(held, wait)
	body["discovered_from"] = nil
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "owner"}}
	return body
}

// jhReplanOf is the id of the re-plan Job of the Held Job held's wait on wait,
// empty when the Feed holds none.
func jhReplanOf(snap Snapshot, held, wait string) string {
	title := jhReplanHeldTitle(held, wait)
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) && it.Title == title {
			return it.ID
		}
	}
	return ""
}

// jhReplanHeld posts one orchestrator Job for each Held Job and each of its job
// waits that can no longer be met (heldDeadWaits), whoever cancelled the chain:
// a hold is released only by its wait, so without it the Job holds for ever. It
// is posted once per Held Job and wait, the re-plan already on the Feed with
// that title being the record, however the chain ended or later grows. It posts
// a Job and nothing more: it refuses, cancels and counts nothing.
func (s Store) jhReplanHeld(snap Snapshot) error {
	for _, id := range ids(snap) {
		for _, w := range heldDeadWaits(snap, snap.State.Jobs[id]) {
			if jhReplanOf(snap, id, w.Ref) != "" {
				continue
			}
			next := gkNextJobID(snap)
			body := ReplanHeldJobBody(next, id, w.Ref, w.Row, w.end, snap.State.Jobs[w.end].Status)
			if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: body}, s.Writer); err != nil {
				return err
			}
			// The next wait counts this Job when it picks its id, and a second row on
			// the same wait finds it.
			var err error
			if snap, err = Load(s.Feed); err != nil {
				return err
			}
		}
	}
	return nil
}

// jhAskOwnerHeld posts one person Job to the task's owner for each re-plan Job of
// a Held Job's dead wait (jhReplanHeld) that ended cancelled or rejected while
// the Job is still held on that wait, as guReplanAskOwner does for the re-plan of
// a dead Job: no pass re-plans a re-plan. It is posted once: the person Job
// discovered_from the re-plan Job is the record (guAskedOwner). With no owner it
// posts nothing.
func (s Store) jhAskOwnerHeld(snap Snapshot) error {
	if strings.TrimSpace(s.Owner) == "" {
		return nil
	}
	for _, id := range ids(snap) {
		for _, w := range heldDeadWaits(snap, snap.State.Jobs[id]) {
			replan := jhReplanOf(snap, id, w.Ref)
			st := snap.State.Jobs[replan]
			if st == nil || st.Status != contract.Cancelled && st.Status != contract.Rejected || guAskedOwner(snap, replan) {
				continue
			}
			next := gkNextJobID(snap)
			body := ReplanOwnerJobBody(next, s.Owner, replan, w.Ref, st.Status, snap.State.Jobs[w.Ref].Status, []string{id})
			if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: body}, s.Writer); err != nil {
				return err
			}
			var err error
			if snap, err = Load(s.Feed); err != nil {
				return err
			}
		}
	}
	return nil
}
