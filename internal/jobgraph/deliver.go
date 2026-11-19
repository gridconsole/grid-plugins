package jobgraph

import (
	"slices"
	"sort"
	"strconv"
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
			if ok, reason := g.reworkDone(s, id, map[string]bool{id: true}); !ok && !g.superseded(s, id) {
				return Wait{Job: id, Target: id, Reason: reason}, false
			}
			continue
		}
		return Wait{Job: id, Target: id, Reason: DoneReason(id, status)}, false
	}
	return Wait{}, true
}

// superseded reports whether the rejected deliver Job id and every Job that reworks
// it, in turn, ended rejected or cancelled, and a deliver Job posted after the last
// of them, by Job number, is accepted: a decision Job that re-planned the rework
// posts that delivery with no continues link (atos T-01), and it is the delivery
// the review accepted. A rework still open is not superseded.
func (g *Graph) superseded(s Snapshot, id string) bool {
	chain, last := map[string]bool{id: true}, id
	for next := []string{id}; len(next) > 0; {
		r := next[0]
		next = next[1:]
		if st := s.Jobs[r].Status; st != contract.Rejected && st != contract.Cancelled {
			return false
		}
		if slices.Compare(jobNumber(r), jobNumber(last)) > 0 {
			last = r
		}
		for _, c := range g.reworksOf(r, chain) {
			chain[c] = true
			next = append(next, c)
		}
	}
	for _, d := range g.ids {
		if !chain[d] && IsDeliver(g.items[d]) && s.Jobs[d].Status == contract.Accepted && slices.Compare(jobNumber(d), jobNumber(last)) > 0 {
			return true
		}
	}
	return false
}

// jobNumber is the numbers of a J-n.m id, in the order the Jobs were numbered.
func jobNumber(id string) []int {
	var out []int
	for _, p := range strings.Split(strings.TrimPrefix(id, "J-"), ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
