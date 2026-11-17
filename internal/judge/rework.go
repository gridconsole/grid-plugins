package judge

import (
	"fmt"
	"strconv"
	"strings"
)

// Rework builds the rework of a builder Job its review rejected (the app's rework): a new Job
// under the next free top-level id that continues the builder and is discovered from the review,
// with the builder's body and one more acceptance row holding the review's must-fix, and then the
// control that rejects the builder as reworked by it. A rework already on the Feed is not posted
// again; only the control is. The rework's base is the SHA of the task's head (refs/grid/<T>/head),
// else the target, else the builder's own target or base. The wake goes on the control, the last post.
func Rework(st State, builder, review, wake, target string) (Plan, error) {
	_, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	r := st.Jobs[review]
	if r == nil {
		return Plan{}, fmt.Errorf("no review Job %s on the task's Feed", review)
	}
	round := st.roundsOf(builder)
	var posts []Post
	id := st.reworkOf(builder, review)
	if id == "" {
		id = st.nextTopID()
		_, text, mustFix := st.verdictOf(r)
		if len(mustFix) == 0 {
			if text == "" {
				text = "the review " + review + " gave no must-fix"
			}
			mustFix = []string{text}
		}
		note := fmt.Sprintf("Must-fix from review round %d:\n- %s", round, strings.Join(mustFix, "\n- "))
		body := copyBody(src)
		rows, _ := src["acceptance"].([]any)
		rows = append(append([]any{}, rows...), map[string]any{"id": nextRowID(rows), "text": note, "traces": "owner"})
		for _, k := range []string{"status", "created_at", "order"} {
			delete(body, k)
		}
		body["id"] = id
		body["acceptance"] = rows
		body["continues"] = builder
		body["discovered_from"] = review
		body["attempt"] = 1
		body["review_rounds"] = round
		body["checkpoint"] = nil
		body["close_reason"] = nil
		if base := st.reworkBase(src, target); base != "" {
			body["base"] = base
		}
		posts = append(posts, Post{Task: st.Task, Type: "job", Job: id, Body: body})
	}
	posts = append(posts, Control(st.Task, builder, "rejected", fmt.Sprintf("rework after review round %d", round), wake,
		map[string]any{"close_reason": "reworked as " + id}))
	return Plan{Posts: posts}, nil
}

// reworkBase is the base of a rework Job: the task head's SHA, else target, else the builder's
// own target or base.
func (s State) reworkBase(src map[string]any, target string) string {
	if head, ok := s.Records[s.Head]; ok && s.Head != "" {
		if sha := str(head.Body["sha"]); sha != "" {
			return sha
		}
	}
	for _, v := range []string{target, str(src["target"]), str(src["base"])} {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// reworkOf is the rework of builder after review already on the Feed: the Job that continues the
// builder and is discovered from that review, "" when there is none.
func (s State) reworkOf(builder, review string) string {
	for _, id := range sortedIDs(s.Jobs) {
		j := s.Jobs[id]
		if j.Continues != nil && *j.Continues == builder && j.DiscoveredFrom != nil && *j.DiscoveredFrom == review {
			return id
		}
	}
	return ""
}

// nextRowID is the next free acceptance row id A<n> after rows.
func nextRowID(rows []any) string {
	highest := 0
	for _, e := range rows {
		if row, ok := e.(map[string]any); ok {
			if n, err := strconv.Atoi(strings.TrimPrefix(str(row["id"]), "A")); err == nil && n > highest {
				highest = n
			}
		}
	}
	return "A" + strconv.Itoa(highest+1)
}
