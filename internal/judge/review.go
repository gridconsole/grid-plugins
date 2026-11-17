package judge

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// copyBody is a shallow copy of a job record body: the verbs replace keys, never edit values.
func copyBody(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// jobOf is the Job id and its folded Job and job record body, or an error naming what is missing.
func (s State) jobOf(id string) (*contract.Job, map[string]any, error) {
	j := s.Jobs[id]
	if j == nil {
		return nil, nil, fmt.Errorf("no Job %s on the task's Feed", id)
	}
	body := s.bodyOf(id)
	if body == nil {
		return nil, nil, fmt.Errorf("Job %s has no job record on the task's Feed", id)
	}
	return j, body, nil
}

// submitted is the SHA of the newest submit of the Job's current attempt, "" when it has none.
func (s State) submitted(j *contract.Job) string {
	r, ok := s.newest(j.Submits, func(r feedRecord) bool { return attemptOf(r) == j.Attempt })
	if !ok {
		return ""
	}
	return str(r.Body["sha"])
}

// Review builds the review Job of a submitted builder Job (the app's hand): a reviewer, or a
// checker when the agent is the checker, numbered under the first Job of the builder's chain and
// discovered from the builder, with one acceptance row that holds the builder's rows. Its base is
// the SHA the builder submitted for its current attempt, so the review reads what was submitted.
// The wake is the event the call answers; the Feed takes a wake once.
func Review(st State, builder, wake, agent string) (Plan, error) {
	b, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	child := st.childID(st.rootOf(builder))
	reviewer := "reviewer"
	if a := strings.ToLower(strings.TrimSpace(agent)); a == "checker" || a == "grid-checker" {
		reviewer = "checker"
	}
	body := copyBody(src)
	for _, k := range []string{"review", "status", "created_at", "order"} {
		delete(body, k)
	}
	body["id"] = child
	body["role"] = reviewer
	body["title"] = "Review " + builder + ": " + str(src["title"])
	body["discovered_from"] = builder
	body["continues"] = nil
	body["after"] = []any{}
	body["write"] = []any{}
	body["verify"] = map[string]any{"kind": "reviewer-check", "job": child}
	body["out_of_scope"] = []any{}
	body["acceptance"] = []any{reviewAcceptance(builder, src)}
	body["attempt"] = 1
	body["review_rounds"] = 0
	body["checkpoint"] = nil
	body["close_reason"] = nil
	body["worktree"] = nil
	if sha := st.submitted(b); sha != "" {
		body["base"] = sha
	}
	return Plan{Posts: []Post{{Task: st.Task, Type: "job", Job: child, Body: body, Wake: wake}}}, nil
}

// reviewAcceptance is the one row of a review Job: the builder's result meets its own rows.
func reviewAcceptance(builder string, src map[string]any) map[string]any {
	var rows []string
	list, _ := src["acceptance"].([]any)
	for _, e := range list {
		if row, ok := e.(map[string]any); ok {
			rows = append(rows, strings.TrimSpace(str(row["id"])+" "+str(row["text"])))
		}
	}
	text := "the result of " + builder + " meets its acceptance rows"
	if len(rows) > 0 {
		text += ": " + strings.Join(rows, "; ")
	}
	return map[string]any{"id": "A1", "text": text, "traces": "item"}
}

// whyNotRun is why a cancelled review never ran: the code its worker exited with, else the close
// reason of its cancel, else the reason of the newest control that cancelled it.
func (s State) whyNotRun(r *contract.Job) string {
	if len(r.ExitCode) > 0 && string(r.ExitCode) != "null" {
		var text string
		if json.Unmarshal(r.ExitCode, &text) == nil {
			if text != "" {
				return text
			}
		} else {
			return string(r.ExitCode)
		}
	}
	if r.CloseReason != nil && strings.TrimSpace(*r.CloseReason) != "" {
		return *r.CloseReason
	}
	var newest feedRecord
	for _, rec := range s.Records {
		if rec.Type == "control" && str(rec.Body["target"]) == r.ID && str(rec.Body["status"]) == string(contract.Cancelled) && rec.Seq >= newest.Seq {
			newest = rec
		}
	}
	if why := str(newest.Body["reason"]); why != "" {
		return why
	}
	return "the review was cancelled and no reason was recorded"
}

// ran is whether a review gave a result: a review cancelled with no result never ran.
func (s State) ran(r *contract.Job) bool {
	_, ok := s.newest(r.Results, isResult)
	return ok
}
