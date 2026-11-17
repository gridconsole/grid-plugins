package judge

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Person builds the person Job that hands a builder Job to its owner (the app's person, personPoint
// and reviewLost). With point and no from, it is the workflow's person point: the builder is ready
// for the owner's judgement. With from, it reads that review: one cancelled before it ran leaves
// the builder with no verdict, a REJECTED one is the second rejection after a rework, and a STUCK
// one (or one with no verdict line) cannot judge the builder; each raises one person Job. With
// neither, it looks for a review of the builder cancelled before it ran. Each person Job is raised
// once: a second call finds it on the Feed and posts nothing, saying so in the plan's note.
func Person(st State, builder, from, owner, wake, point string) (Plan, error) {
	b, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	if owner == "" {
		return Plan{}, errors.New("a person Job needs its owner: name one with --owner, or give the task an owner")
	}
	raise := func(child, discovered, title, goal string) Plan {
		if p := st.personFrom(discovered); p != nil {
			return Plan{Note: fmt.Sprintf("the person Job %s from %s is already on the Feed", p.ID, discovered)}
		}
		body := personBody(child, builder, discovered, title, goal, owner, src)
		return Plan{Posts: []Post{{Task: st.Task, Type: "job", Job: child, Body: body, Wake: wake, Person: owner}}}
	}
	if from == "" && point != "" {
		goal := fmt.Sprintf("%s is ready for your judgement: the workflow's next step is %s", builder, point)
		return raise(st.childID(st.rootOf(builder)), builder, "Judge "+builder+" before "+point, goal), nil
	}
	if from == "" {
		for _, r := range st.reviewsOf(b.ID) {
			if r.Status == contract.Cancelled && !st.ran(r) {
				return st.lost(builder, r, raise), nil
			}
		}
		return Plan{Note: "no review of " + builder + " was cancelled before it ran: name the review with --from, or the workflow step with --point"}, nil
	}
	r := st.Jobs[from]
	if r == nil {
		return Plan{}, fmt.Errorf("no review Job %s on the task's Feed", from)
	}
	if r.Status == contract.Cancelled {
		if !st.ran(r) {
			return st.lost(builder, r, raise), nil
		}
		return Plan{Note: fmt.Sprintf("the review %s gave its result before it was cancelled: read it with grid judge verdict %s", from, from)}, nil
	}
	verdict, text, _ := st.verdictOf(r)
	root := st.childID(st.rootOf(builder))
	switch verdict {
	case Accepted:
		return Plan{Note: fmt.Sprintf("the review %s accepted %s: there is no decision to hand to a person; name the workflow step with --point", from, builder)}, nil
	case Rejected:
		goal := fmt.Sprintf("The review of %s rejected it again after its rework. The reviewer's recommendation:\n\n%s", builder, text)
		return raise(root, from, "Decide how "+builder+" continues", goal), nil
	case Stuck:
		goal := fmt.Sprintf("The review %s of %s is stuck and cannot judge it. The reviewer's reason:\n\n%s", from, builder, text)
		return raise(root, from, "Decide how "+builder+" continues", goal), nil
	}
	goal := fmt.Sprintf("The review %s of %s gave no verdict, so %s is not accepted.", from, builder, builder)
	if text != "" {
		goal += " The reviewer wrote:\n\n" + text
	}
	return raise(root, from, "Decide how "+builder+" continues", goal), nil
}

// lost is the person Job for a review cancelled before it ran (the app's reviewLost).
func (s State) lost(builder string, r *contract.Job, raise func(child, discovered, title, goal string) Plan) Plan {
	goal := fmt.Sprintf("The review %s of %s was cancelled before it ran, so %s has no verdict and is not accepted. "+
		"Why the review could not start: %s. Choose: accept %s without a review, or ask for rework.",
		r.ID, builder, builder, s.whyNotRun(r), builder)
	return raise(s.childID(builder), builder, "Decide how "+builder+" continues without its review", goal)
}

// personBody is the job record of a person Job (the app's personBody): an ask for the owner, who
// decides how the builder continues, discovered from from.
func personBody(id, builder, from, title, goal, owner string, src map[string]any) map[string]any {
	item := 1
	switch v := src["item"].(type) {
	case float64:
		if v >= 1 {
			item = int(v)
		}
	case int:
		if v >= 1 {
			item = v
		}
	}
	intentItem := "intent.md#request"
	if intent, ok := src["intent"].(map[string]any); ok && str(intent["item"]) != "" {
		intentItem = str(intent["item"])
	}
	return map[string]any{
		"id": id, "kind": "ask", "item": item, "title": title, "role": "Person",
		"skills": []any{}, "risk": []any{}, "links": []any{}, "keep": []any{}, "decisions": []any{},
		"intent":       map[string]any{"owner": owner, "item": intentItem, "goal": goal},
		"read":         []any{"intent.md#request"},
		"write":        []any{},
		"out_of_scope": []any{},
		"acceptance":   []any{map[string]any{"id": "A1", "text": owner + " decides how " + builder + " continues", "traces": "owner"}},
		"verify":       map[string]any{"kind": "file-exists", "path": "intent.md"},
		"budget":       map[string]any{"turns_estimate": 1, "reads_before_first_edit": 1},
		"on_blocked":   "ask-and-continue",
		"after":        []any{},
		"attempt":      1,
		"continues":    nil, "discovered_from": from,
		"close_reason": nil, "checkpoint": nil, "review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []any{"result"},
	}
}

// sortedIDs are the keys of jobs in order.
func sortedIDs(jobs map[string]*contract.Job) []string {
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
