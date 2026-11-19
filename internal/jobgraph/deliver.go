package jobgraph

import (
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// DeliverTitle is the title of a task's deliver Job, the last Job of a plan, which
// runs grid deliver (skills/plan-jobs/SKILL.md). The app's Judge gives a rework of
// it the same title, so a rework delivers again.
const DeliverTitle = "Deliver the task"

// QuestionKind is the kind of the person Job grid question ask posts for a
// question (question.JobKind; this package reads no question files).
const QuestionKind = "ask"

// IsDeliver reports whether it is a deliver Job: a Job for an agent titled
// DeliverTitle.
func IsDeliver(it job.Job) bool {
	return !it.IsPerson() && strings.TrimSpace(it.Title) == DeliverTitle
}

// closed is a Job that will not move again by itself.
func closed(status contract.Status) bool {
	return status == contract.Accepted || status == contract.Cancelled || status == contract.Rejected
}

// questionWaits is why the deliver Job id has not started: each person question
// of the task (a person Job of kind QuestionKind) that is still open. An answer
// can change what is delivered, so the delivery waits for it (round 14 F2: the
// deliver Job started while a question another Job asked was open). A Job that
// has started is never held by a question asked later; Ready skips it before this.
func (g *Graph) questionWaits(s Snapshot, id string) []Wait {
	out := []Wait{}
	for _, q := range g.ids {
		it := g.items[q]
		if !it.IsPerson() || it.Kind != QuestionKind || closed(s.Jobs[q].Status) {
			continue
		}
		out = append(out, Wait{Job: id, Target: q, Reason: "waiting for " + q + ", a question for " + it.PersonName() + ", to be answered"})
	}
	return out
}

// DeliveryWait is why the work a task delivered may not land yet, given the Jobs
// of its Feed: a deliver Job that has started is not yet accepted, or one was
// rejected and the Jobs that rework it are not yet accepted (reworkDone), so the
// delivery the person would approve is not the one the review accepted (round 14
// F1). A deliver Job that has not started delivered nothing, and a cancelled one
// was stopped; neither holds the land. ok is true when nothing waits. It reads
// items as they are: an after target that is not posted is not its concern.
func DeliveryWait(items []job.Job, s Snapshot) (Wait, bool) {
	g := &Graph{items: map[string]job.Job{}, children: map[string][]string{}}
	for _, it := range items {
		if _, dup := g.items[it.ID]; dup || !jobID.MatchString(it.ID) {
			continue
		}
		g.items[it.ID] = it
		g.ids = append(g.ids, it.ID)
	}
	sort.Strings(g.ids)
	g.link()
	for _, id := range g.ids {
		if !IsDeliver(g.items[id]) {
			continue
		}
		status := s.Jobs[id].Status
		switch status {
		case "", contract.Posted, contract.Ready, contract.Cancelled, contract.Accepted:
			continue
		case contract.Rejected:
			if ok, reason := g.reworkDone(s, id, map[string]bool{id: true}); !ok {
				return Wait{Job: id, Target: id, Reason: reason}, false
			}
			continue
		}
		return Wait{Job: id, Target: id, Reason: DoneReason(id, status)}, false
	}
	return Wait{}, true
}
