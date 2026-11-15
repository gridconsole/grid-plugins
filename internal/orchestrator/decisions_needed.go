package orchestrator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// abDnA1Lead begins A1 of every decisions Job, and the result record id follows
// it. As with kbSfA1Lead, A1 is the one place of the job-record the id can be
// held: the record allows no free field and discovered_from belongs to the
// re-plan of a Job (gkReplanned).
const abDnA1Lead = "Decisions needed in result record "

// abDecisionsNeeded posts one orchestrator Job for each result record of a Job
// of the fold whose body holds a non-empty decisions_needed. Without it the fold
// only sets NeedsLead and the proposals pass skips the Job: the decision stays a
// flag and the task waits for a chat. The Job it posts asks the person who
// decides each one, so the task carries on once they answer.
//
// A result is asked about once. The Job already on the Feed that holds the
// result id in A1 (role orchestrator, abDnPosted) is the record that says so,
// read from the snapshot under the task lock, so a later tick and a restarted
// process post nothing more. A result of a Job this pass posted (its A1 begins
// with abDnA1Lead) posts nothing: its question is the path to the person. A
// result whose body is a ref (not read yet) holds no decisions and posts nothing.
// It posts a Job and nothing more: it refuses, cancels and counts nothing.
func (s Store) abDecisionsNeeded(snap Snapshot) error {
	for _, id := range ids(snap) {
		j := snap.State.Jobs[id]
		if j == nil || abDnAsks(snap, id) {
			continue
		}
		for _, result := range j.Results {
			r, ok := snap.Records[result]
			if !ok {
				continue
			}
			decisions := r9OaOpen(snap, abDnDecisions(r.Body))
			if len(decisions) == 0 || abDnPosted(snap, result) {
				continue
			}
			next := gkNextJobID(snap)
			if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: AbDnJobBody(next, result, id, decisions)}, s.Writer); err != nil {
				return err
			}
			// The next result counts this Job when it picks its id.
			var err error
			if snap, err = Load(s.Feed); err != nil {
				return err
			}
		}
	}
	return nil
}

// abDnDecisions reads decisions_needed of a result body, each decision as one
// line; a decision that holds no word is left out.
func abDnDecisions(body feed.Body) []string {
	raw, err := json.Marshal(body["decisions_needed"])
	if err != nil {
		return nil
	}
	var all []string
	if json.Unmarshal(raw, &all) != nil {
		return nil
	}
	var out []string
	for _, d := range all {
		if d = kbSfOneLine(d); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// r9OaJobID is a Job id as a word of a decision line.
var r9OaJobID = regexp.MustCompile(`\bJ-[0-9]+(?:\.[0-9]+)*\b`)

// r9OaOpen is the decisions no person Job on the Feed asks yet. One decision
// has one ask (round 9, task T-02): a decision line that names a Job whose role
// is Person was asked by that Job, so the pass asks it no second time. A line
// that names no Job, or only Jobs of another role, stays open.
func r9OaOpen(snap Snapshot, decisions []string) []string {
	person := map[string]bool{}
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), "person") {
			person[it.ID] = true
		}
	}
	var open []string
	for _, d := range decisions {
		asked := false
		for _, id := range r9OaJobID.FindAllString(d, -1) {
			asked = asked || person[id]
		}
		if !asked {
			open = append(open, d)
		}
	}
	return open
}

// abDnAsks reports whether the Job id is a decisions Job: its A1 begins with
// abDnA1Lead.
func abDnAsks(snap Snapshot, id string) bool {
	for _, it := range snap.Items {
		if it.ID != id {
			continue
		}
		for _, a := range it.Acceptance {
			if a.ID == "A1" && strings.HasPrefix(a.Text, abDnA1Lead) {
				return true
			}
		}
	}
	return false
}

// abDnPosted reports whether the Feed holds the decisions Job of a result
// record: an orchestrator Job whose A1 begins with abDnA1Lead and the record id.
func abDnPosted(snap Snapshot, resultID string) bool {
	lead := abDnA1Lead + resultID + " ("
	for _, it := range snap.Items {
		if !strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) {
			continue
		}
		for _, a := range it.Acceptance {
			if a.ID == "A1" && strings.HasPrefix(a.Text, lead) {
				return true
			}
		}
	}
	return false
}

// AbDnJobBody is the job-record of the orchestrator Job that asks the person
// the decisions a result record of a Job needs. It is the body ReplanJobBody
// builds, with the title and the one acceptance row of the decisions, and with
// discovered_from null, as SecurityFoundJobBody: a pointer would read as the
// re-plan of job. The result record id is in A1 (abDnA1Lead). Like the re-plan
// Job it carries no job: pointer.
func AbDnJobBody(id, result, job string, decisions []string) map[string]any {
	body := ReplanJobBody(id, job, "", nil)
	each := make([]string, 0, len(decisions))
	for _, d := range decisions {
		if d = gqCause(kbSfOneLine(d)); d != "" {
			each = append(each, d)
		}
	}
	first := ""
	for _, d := range decisions {
		if d = kbSfOneLine(d); d != "" {
			first = kbSfCut(d, kbSfTitleMax)
			break
		}
	}
	body["title"] = "Decisions needed on " + job + ": " + first
	body["discovered_from"] = nil
	a1 := fmt.Sprintf("%s%s (on %s): %s. One decision has one ask: ask nothing a person Job or question on this task already asks (`grid job tree`, `grid question list`), and only what the person's words leave open and the work cannot go on without; a new option the person did not ask for is left out and named in the result. Ask the person who decides each one with `grid question ask` (a default and a time) or a person Job, then post the result with `grid job result`.",
		abDnA1Lead, result, job, strings.Join(each, "; "))
	body["acceptance"] = []map[string]any{{"id": "A1", "text": a1, "traces": "owner"}}
	return body
}
