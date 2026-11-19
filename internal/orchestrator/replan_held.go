package orchestrator

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jhReplanHeldTitle is the title of the orchestrator Job that re-plans the Held
// Job held whose wait w can no longer be met. The title is the record that the
// re-plan was posted, one for each dead end, missing Job or wait cycle: it names
// the held Job, the wait and why (deadWait.tag), and is never the re-plan of a
// dead Job (gkReplanned, discovered_from).
func jhReplanHeldTitle(held string, w deadWait) string {
	return fmt.Sprintf("Re-plan %s: its wait on %s can no longer be met (%s)", held, w.Ref, w.tag())
}

// ReplanHeldJobBody is the job-record of the orchestrator Job that re-plans the
// Held Job held, whose job or landed wait w can no longer be met: w.Ref and every
// Job that continues it were rejected or cancelled (and, for a landed wait, none
// landed), end the last of them with endStatus; or, with no endStatus, w.Ref is
// not on the Feed; or, with a cycle, w.Ref waits on held in turn and cycle is the
// Jobs of that wait cycle. It is the body ReplanJobBody builds, with no Job it was
// discovered from (w.Ref may already have a re-plan of its own) and a title and
// acceptance text that name held, the wait and why. Like it, it carries no job:
// pointer.
func ReplanHeldJobBody(id, held string, w contract.HeldWait, end string, endStatus contract.Status, cycle []string) map[string]any {
	d := deadWait{HeldWait: w, end: end, missing: endStatus == "" && cycle == nil, cycle: cycle}
	on := w.Ref
	if w.Kind == contract.WaitLanded {
		on += " to land"
	}
	if w.Row != "" {
		on += " (row " + w.Row + ")"
	}
	next := fmt.Sprintf("%s is cancelled with a reason or replaced by a new Job that does not wait on %s; the work of %s is posted again as a new Job that `grid job lint` passes when the task still needs it", held, w.Ref, w.Ref)
	var why string
	switch {
	case d.cycle != nil:
		why = fmt.Sprintf("the Jobs %s wait on each other in a cycle, through holds and after edges", strings.Join(cycle, ", "))
		next = "one Job of the cycle is cancelled with a reason or replaced by a new Job that does not wait on the next, so the others can go on"
	case d.missing:
		why = w.Ref + " is not on the Feed"
		next = fmt.Sprintf("%s is cancelled with a reason or replaced by a new Job that does not wait on %s; the work %s waits for is posted as a new Job that `grid job lint` passes when the task still needs it", held, w.Ref, held)
	case end == w.Ref:
		why = fmt.Sprintf("%s was %s with no Job continuing it", w.Ref, endStatus)
	default:
		why = fmt.Sprintf("%s and every Job that continues it ended: the last, %s, was %s", w.Ref, end, endStatus)
	}
	text := fmt.Sprintf("%s is held waiting for %s, and %s, so the wait can no longer be met: %s; a result is posted with `grid job result`.", held, on, why, next)
	body := ReplanJobBody(id, "", "", nil)
	body["title"] = jhReplanHeldTitle(held, d)
	body["discovered_from"] = nil
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "owner"}}
	return body
}

// jhReplanOf is the id of the re-plan Job of the Held Job held's dead wait w,
// empty when the Feed holds none.
func jhReplanOf(snap Snapshot, held string, w deadWait) string {
	title := jhReplanHeldTitle(held, w)
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) && it.Title == title {
			return it.ID
		}
	}
	return ""
}

// jhReplanHeld posts one orchestrator Job for each Held Job and each of its job
// or landed waits that can no longer be met (heldDeadWaits), whoever cancelled
// the chain: a hold is released only by its wait, so without it the Job holds for
// ever. It is posted once per Held Job, wait and dead end, the re-plan already on
// the Feed with that title being the record: a chain that stays dead gets one, a
// chain that a later Job continues and that dies again gets one more. A wait on a
// Job not on the Feed gets one, and a wait cycle one in all, on its lead wait. It
// posts a Job and nothing more: it refuses, cancels and counts nothing.
func (s Store) jhReplanHeld(snap Snapshot) error {
	for _, id := range ids(snap) {
		for _, w := range heldDeadWaits(snap, snap.State.Jobs[id]) {
			if w.cycle != nil && !w.lead || jhReplanOf(snap, id, w) != "" {
				continue
			}
			var endStatus contract.Status
			if end := snap.State.Jobs[w.end]; end != nil {
				endStatus = end.Status
			}
			next := gkNextJobID(snap)
			body := ReplanHeldJobBody(next, id, w.HeldWait, w.end, endStatus, w.cycle)
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
// a Held Job's dead wait (jhReplanHeld) that ended while the Job is still held on
// that wait, for the same reason (deadWait.tag), as guReplanAskOwner does for the re-plan of a
// dead Job: no pass re-plans a re-plan. A cancelled or rejected re-plan asks at
// once. An accepted one asks from the tick after the one that first saw it
// accepted (its memory's HeldAskDue), so the hold its own posts end is gone by
// then. It is posted once: the person Job discovered_from the re-plan Job is the
// record (guAskedOwner). With no owner it posts nothing.
func (s Store) jhAskOwnerHeld(snap Snapshot, m *memory) error {
	if strings.TrimSpace(s.Owner) == "" {
		return nil
	}
	for _, id := range ids(snap) {
		for _, w := range heldDeadWaits(snap, snap.State.Jobs[id]) {
			if w.cycle != nil && !w.lead {
				continue
			}
			replan := jhReplanOf(snap, id, w)
			st := snap.State.Jobs[replan]
			if st == nil || st.Status != contract.Cancelled && st.Status != contract.Rejected && st.Status != contract.Accepted || guAskedOwner(snap, replan) {
				continue
			}
			if st.Status == contract.Accepted {
				mem := m.Jobs[replan]
				if mem == nil {
					mem = s.recoverMemory(snap, st)
					m.Jobs[replan] = mem
				}
				if !mem.HeldAskDue {
					mem.HeldAskDue = true
					continue
				}
			}
			next := gkNextJobID(snap)
			body := replanOwnerJobBody(next, s.Owner, replan, w.Ref, st.Status, w.waiting(snap), []string{id})
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
