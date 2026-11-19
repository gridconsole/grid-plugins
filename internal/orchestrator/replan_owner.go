package orchestrator

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// guReplanAskOwner posts one person Job, for the task owner, for each re-plan
// Job (role orchestrator, discovered_from a Job D) that itself ended Cancelled
// or Rejected while Jobs still wait on D with a done edge. gkReplanned counts a
// re-plan Job on the Feed as the answer for D for good, so once that Job is dead
// nothing posts again and the waiters of D wait for ever. A person decides what
// comes next. The person Job is discovered_from the dead re-plan Job, and that
// record says it was posted: it is posted once, read from the snapshot under the
// task lock. It posts person Jobs and nothing more: never a re-plan Job, and it
// refuses, cancels and counts nothing.
//
// An empty Store.Owner posts nothing: there is no person to ask. The crash-loop
// park refuses the same case (park, events.go), where it returns an error; a pass
// that runs on every tick must not fail the tick for it, so it skips.
func (s Store) guReplanAskOwner(snap Snapshot) error {
	if strings.TrimSpace(s.Owner) == "" {
		return nil
	}
	for _, it := range snap.Items {
		if !strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) || it.DiscoveredFrom == nil {
			continue
		}
		replan := snap.State.Jobs[it.ID]
		if replan == nil || replan.Status != contract.Cancelled && replan.Status != contract.Rejected {
			continue
		}
		dead := *it.DiscoveredFrom
		waiting := gkWaiting(snap, dead)
		if len(waiting) == 0 || guAskedOwner(snap, it.ID) {
			continue
		}
		var deadStatus contract.Status
		if d := snap.State.Jobs[dead]; d != nil {
			deadStatus = d.Status
		}
		next := gkNextJobID(snap)
		body := ReplanOwnerJobBody(next, s.Owner, it.ID, dead, replan.Status, deadStatus, waiting)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: body}, s.Writer); err != nil {
			return err
		}
		// The next dead re-plan Job counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// guAskedOwner reports whether the Feed holds a person Job discovered_from the
// re-plan Job.
func guAskedOwner(snap Snapshot, replan string) bool {
	for _, it := range snap.Items {
		if it.IsPerson() && it.DiscoveredFrom != nil && *it.DiscoveredFrom == replan {
			return true
		}
	}
	return false
}

// ReplanOwnerJobBody is the job-record of the person Job that asks owner how the
// task goes on after the re-plan Job replan of dead ended replanStatus while the
// Jobs in waiting still hold a done edge to dead (deadStatus is dead's status).
// It has the fields of ReplanJobBody with the person fields `grid job post
// --person` sets: role Person, intent.owner the owner. Kind ask is the kind of a
// person Job that a person answers. It carries no job: pointer, like it.
func ReplanOwnerJobBody(id, owner, replan, dead string, replanStatus, deadStatus contract.Status, waiting []string) map[string]any {
	return replanOwnerJobBody(id, owner, replan, dead, replanStatus, jobgraph.DoneReason(dead, deadStatus), waiting)
}

// replanOwnerJobBody is ReplanOwnerJobBody with reason the text of what each Job
// in waiting waits for.
func replanOwnerJobBody(id, owner, replan, dead string, replanStatus contract.Status, reason string, waiting []string) map[string]any {
	each := make([]string, 0, len(waiting))
	for _, w := range waiting {
		each = append(each, w+" ("+reason+")")
	}
	text := fmt.Sprintf("%s decides how the task goes on: re-plan Job %s of %s ended %s and %s still wait on %s: %s. The decision says for each of them whether it is cancelled with a reason or replaced by a new Job that does not wait on %s, and whether the work of %s is posted again.",
		owner, replan, dead, replanStatus, strings.Join(waiting, ", "), dead, strings.Join(each, ", "), dead, dead)
	body := ReplanJobBody(id, dead, "", waiting)
	body["title"] = fmt.Sprintf("Decide how %s continue: re-plan Job %s of %s ended %s", strings.Join(waiting, ", "), replan, dead, replanStatus)
	body["role"] = "Person"
	body["kind"] = "ask"
	body["skills"] = []string{}
	body["intent"] = map[string]any{"owner": owner, "item": "intent.md#request", "goal": "intent.md#request"}
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "owner"}}
	body["discovered_from"] = replan
	body["posts_back"] = []string{}
	return body
}
