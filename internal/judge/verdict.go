package judge

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func isResult(r feed.Record) bool { return r.Type == "result" }

// The verdicts a reviewer gives on the first line of its result.
const (
	Accepted = "ACCEPTED"
	Rejected = "REJECTED"
	Stuck    = "STUCK"
)

// Parse reads the verdict of one result body (the app's verdict.Parse, with STUCK). The first line
// of its outcome (else verdict, text, result or summary) is ACCEPTED, REJECTED or STUCK; verdict
// is "" when it is none of them. text is that whole field, trimmed. mustFix is the body's must_fix
// list (strings, or {row, at} objects), else the lines under a "Must-fix:" heading of the text. A
// nil body gives three empty values.
func Parse(body map[string]any) (verdict, text string, mustFix []string) {
	// outcome is the result body key (contract/schemas/result.json); the rest are older keys.
	for _, k := range []string{"outcome", "verdict", "text", "result", "summary"} {
		if v, _ := body[k].(string); strings.TrimSpace(v) != "" {
			text = strings.TrimSpace(v)
			break
		}
	}
	first, _, _ := strings.Cut(text, "\n")
	switch word := strings.ToUpper(strings.Trim(strings.TrimSpace(first), ".:*# ")); word {
	case Accepted, Rejected, Stuck:
		verdict = word
	}
	if list, ok := body["must_fix"].([]any); ok {
		for _, e := range list {
			switch item := e.(type) {
			case string:
				if line := strings.TrimSpace(item); line != "" {
					mustFix = append(mustFix, line)
				}
			case map[string]any:
				// The result schema (contract/schemas/result.json) makes each item {"row", "at"}.
				row, _ := item["row"].(string)
				at, _ := item["at"].(string)
				row, at = strings.TrimSpace(row), strings.TrimSpace(at)
				switch {
				case row != "" && at != "":
					mustFix = append(mustFix, row+": "+at)
				case row != "":
					mustFix = append(mustFix, row)
				case at != "":
					mustFix = append(mustFix, at)
				}
			}
		}
	}
	if len(mustFix) == 0 {
		in := false
		for _, line := range strings.Split(text, "\n") {
			head := strings.ToLower(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(head, "must-fix:"), strings.HasPrefix(head, "must fix:"):
				in = true
				if _, rest, _ := strings.Cut(line, ":"); strings.TrimSpace(rest) != "" {
					mustFix = append(mustFix, strings.TrimSpace(rest))
				}
			case strings.HasPrefix(head, "notes:"), strings.HasPrefix(head, "rows:"), head == "":
				in = false
			case in:
				mustFix = append(mustFix, strings.TrimSpace(line))
			}
		}
	}
	return
}

// Report is what `grid judge verdict` says of one review Job. It posts nothing.
type Report struct {
	// Verdict is ACCEPTED, REJECTED or STUCK, or "" when the review has no result or its result
	// has no verdict line.
	Verdict string   `json:"verdict"`
	Text    string   `json:"text"`
	MustFix []string `json:"must_fix"`
	Review  string   `json:"review"`
	// Result is the record id of the result read: the review Job's newest counted result.
	Result string `json:"result"`
	// ReviewAttempt is the attempt that result was posted under; Attempt is the review Job's
	// attempt now. Stale is true when the result is from an older attempt than the Job's own.
	ReviewAttempt int    `json:"review_attempt"`
	Attempt       int    `json:"attempt"`
	Stale         bool   `json:"stale"`
	Status        string `json:"status"`
	// Reviewed is the Job the review judges; RoundsUsed counts the reviews over its continues
	// chain (a rework continues the Job it reworks). Rounds is the workflow's review rounds when
	// the caller names them, else null: the verb holds no default.
	Reviewed   string `json:"reviewed"`
	RoundsUsed int    `json:"rounds_used"`
	Rounds     *int   `json:"rounds"`
}

// Verdict reads the verdict of the review Job review.
func Verdict(st State, review string) (Report, error) {
	r := st.Jobs[review]
	if r == nil {
		return Report{}, fmt.Errorf("no Job %s on the task's Feed", review)
	}
	rep := Report{Review: review, MustFix: []string{}, Attempt: r.Attempt, Status: string(r.Status)}
	if res, ok := st.newest(r.Results, isResult); ok {
		rep.Result, rep.ReviewAttempt = res.ID, attemptOf(res)
		rep.Stale = rep.ReviewAttempt < r.Attempt
		var mf []string
		rep.Verdict, rep.Text, mf = Parse(res.Body)
		if mf != nil {
			rep.MustFix = mf
		}
	}
	if b := st.reviewedBy(r); b != nil {
		rep.Reviewed, rep.RoundsUsed = b.ID, st.roundsOf(b.ID)
	}
	return rep, nil
}

// verdictOf is the verdict of a review Job's newest result, "" when it has none.
func (s State) verdictOf(r *contract.Job) (verdict, text string, mustFix []string) {
	res, ok := s.newest(r.Results, isResult)
	if !ok {
		return "", "", nil
	}
	return Parse(res.Body)
}
