package orchestrator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jiPlanFaultTitle is the title of the orchestrator Job the tick posts when
// plan.md cannot be published.
const jiPlanFaultTitle = "Re-plan: plan.md cannot be published"

// jiPlanIDs finds every J-<n> in the plan text. The digits are cut by hand
// below: a quantifier here would take the first nine digits of a longer id.
var jiPlanIDs = regexp.MustCompile(`J-([0-9]+)`)

// ReplanPlanFaultJobBody is the job-record of the orchestrator Job that re-plans
// when plan.md cannot be published. It is the body ReplanJobBody builds, with no
// Job it was discovered from (the fault is in the plan, not in a Job) and a
// title and acceptance text that say so. Like it, it carries no job: pointer.
//
// cause is the error planJobInputs returned: text about plan.md and nothing
// else. It ends the acceptance text, cut to gqCauseMax bytes.
func ReplanPlanFaultJobBody(id, cause string) map[string]any {
	body := ReplanJobBody(id, "", "", nil)
	text := fmt.Sprintf("plan.md is fixed so that the next tick publishes its Jobs; none of them uses the id %s, which is this Job's own; a result is posted with `grid job result`.", id)
	body["title"] = jiPlanFaultTitle
	body["discovered_from"] = nil
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text + " Cause: " + gqCause(cause), "traces": "owner"}}
	return body
}

// jiPlanFaultOpen reports whether the Feed holds a re-plan Job for a plan fault
// that is still open: role orchestrator, the plan-fault title, a folded status
// other than Accepted, Rejected or Cancelled, and a first job record that
// carries the task lease. A Job a person posted with that title has no lease
// and does not count.
func jiPlanFaultOpen(snap Snapshot) bool {
	for _, it := range snap.Items {
		if !strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) || it.Title != jiPlanFaultTitle {
			continue
		}
		st := snap.State.Jobs[it.ID]
		if st == nil || st.Status == contract.Accepted || st.Status == contract.Rejected || st.Status == contract.Cancelled {
			continue
		}
		var first *feed.Record
		for _, r := range snap.Records {
			if r.Type != "job" || r.Job == nil || *r.Job != it.ID {
				continue
			}
			r := r
			if first == nil || r.At < first.At || (r.At == first.At && r.ID < first.ID) {
				first = &r
			}
		}
		if first != nil && first.Lease != nil && *first.Lease == "task" {
			return true
		}
	}
	return false
}

// jiPlanFaultJobID is one above the highest J-<n> of the fold (gkNextJobID) and
// of every J-<n> in the plan text that failed, so the fixed plan keeps its ids.
// A plan id whose number Atoi refuses, or that has more than nine digits, is not
// counted.
func jiPlanFaultJobID(snap Snapshot, planText string) string {
	next, _ := strconv.Atoi(strings.TrimPrefix(gkNextJobID(snap), "J-"))
	for _, m := range jiPlanIDs.FindAllStringSubmatch(planText, -1) {
		if len(m[1]) > 9 {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n+1 > next {
			next = n + 1
		}
	}
	return "J-" + strconv.Itoa(next)
}

// postPlanFaultJob posts one re-plan Job to the orchestrator role with the
// cause, unless one for a plan fault is still open. It posts a Job and nothing
// more: it refuses, holds and counts nothing.
func (s Store) postPlanFaultJob(snap Snapshot, planText, cause string) error {
	if jiPlanFaultOpen(snap) {
		return nil
	}
	id := jiPlanFaultJobID(snap, planText)
	_, err := s.Feed.Post(feed.Input{Type: "job", Job: id, Body: ReplanPlanFaultJobBody(id, cause)}, s.Writer)
	return err
}
