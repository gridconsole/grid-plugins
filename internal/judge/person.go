package judge

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Person builds the Job that decides what happens to a builder Job (the app's person, personPoint
// and reviewLost). With point and no from, it is the workflow's person point: a person Job, the
// builder is ready for the owner's judgement. With from, it reads that review: one cancelled before
// it ran leaves the builder with no verdict, a REJECTED one is the second rejection after a rework,
// and a STUCK one (or one with no verdict line) cannot judge the builder; each posts one agent
// decision Job (role orchestrator, the planner agent on Opus), not a person Job: nothing there is
// a question only a person can answer. With neither, it looks for a review of the builder cancelled
// before it ran. Each Job is raised once: a second call finds it on the Feed and posts nothing,
// saying so in the plan's note. A person Job of the old kind still open for the same review is
// cancelled by the call that posts its decision Job.
func Person(st State, builder, from, owner, wake, point string) (Plan, error) {
	b, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	raise := func(child, discovered, title, goal string) Plan {
		if p := st.personFrom(discovered); p != nil {
			return Plan{Note: fmt.Sprintf("the person Job %s from %s is already on the Feed", p.ID, discovered)}
		}
		body := personBody(child, builder, discovered, title, goal, owner, src)
		return Plan{Posts: []Post{{Task: st.Task, Type: "job", Job: child, Body: body, Wake: wake, Person: owner}}}
	}
	// decide posts the agent decision Job for the review (or builder) discovered, and cancels an
	// old-kind person Job still open for it, so an upgraded task does not wait on the owner.
	decide := func(child, discovered, title, goal string) Plan {
		if d := st.decisionFrom(discovered); d != nil {
			return Plan{Note: fmt.Sprintf("the decision Job %s from %s is already on the Feed", d.ID, discovered)}
		}
		old := st.oldPersonFrom(discovered)
		if p := st.personFrom(discovered); old == nil && p != nil {
			return Plan{Note: fmt.Sprintf("the person Job %s from %s is already on the Feed", p.ID, discovered)}
		}
		goal += st.decisionGuide(builder, discovered, from)
		plan := Plan{Posts: []Post{{Task: st.Task, Type: "job", Job: child, Body: decisionBody(child, builder, discovered, title, goal, src), Wake: wake}}}
		if old != nil {
			plan.Posts = append(plan.Posts, Control(st.Task, old.ID, string(contract.Cancelled), "replaced by the decision Job "+child+": an agent decides, and asks only a real question", "", nil))
		}
		return plan
	}
	if from == "" && point != "" {
		if owner == "" {
			return Plan{}, errors.New("a person Job needs its owner: name one with --owner, or give the task an owner")
		}
		goal := fmt.Sprintf("%s is ready for your judgement: the workflow's next step is %s", builder, point)
		return raise(st.childID(st.rootOf(builder)), builder, "Judge "+builder+" before "+point, goal), nil
	}
	if from == "" {
		for _, r := range st.reviewsOf(b.ID) {
			if r.Status == contract.Cancelled && !st.ran(r) {
				return st.lost(builder, r, decide), nil
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
			return st.lost(builder, r, decide), nil
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
		return decide(root, from, "Decide how "+builder+" continues", goal), nil
	case Stuck:
		goal := fmt.Sprintf("The review %s of %s is stuck and cannot judge it. The reviewer's reason:\n\n%s", from, builder, text)
		return decide(root, from, "Decide how "+builder+" continues", goal), nil
	}
	goal := fmt.Sprintf("The review %s of %s gave no verdict, so %s is not accepted.", from, builder, builder)
	if text != "" {
		goal += " The reviewer wrote:\n\n" + text
	}
	return decide(root, from, "Decide how "+builder+" continues", goal), nil
}

// Decide is the agent decision Job raised from builder itself for a reason no review gave, titled
// title ("Decide how <builder> continues: ..."): the land's, when a rebase Job it accepted still
// cannot land on the same target (internal/cli ship_rebase.go). goal says why, and the guide of
// every decision Job follows it. It is raised once per title: a decision Job from builder with that
// title already on the Feed posts nothing, and one of another title (a lost review's) does not count.
func Decide(st State, builder, title, goal string) (Plan, error) {
	_, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	for _, k := range sortedIDs(st.Jobs) {
		if j := st.Jobs[k]; role(j) == "orchestrator" && j.DiscoveredFrom != nil && *j.DiscoveredFrom == builder && str(st.bodyOf(k)["title"]) == title {
			return Plan{Note: fmt.Sprintf("the decision Job %s from %s is already on the Feed", k, builder)}, nil
		}
	}
	child := st.childID(builder)
	goal += st.decisionGuide(builder, builder, "")
	return Plan{Posts: []Post{{Task: st.Task, Type: "job", Job: child, Body: decisionBody(child, builder, builder, title, goal, src)}}}, nil
}

// lost is the decision Job for a review cancelled before it ran (the app's reviewLost).
func (s State) lost(builder string, r *contract.Job, raise func(child, discovered, title, goal string) Plan) Plan {
	goal := fmt.Sprintf("The review %s of %s was cancelled before it ran, so %s has no verdict and is not accepted. "+
		"Why the review could not start: %s.",
		r.ID, builder, builder, s.whyNotRun(r))
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

// Migrate is the plan that hands one open person Job of the old kind to an agent (the Judge used to
// ask the owner how a builder continues), so no one has to run grid judge person by hand. It is
// Person's own plan: the decision Job is raised once and the person Job is cancelled by the same
// plan. When Person posts nothing the person Job is still cancelled, naming why. It reports false when no such Job is left, and a caller reads the state again for the next.
func Migrate(st State) (Plan, bool) {
	for _, id := range sortedIDs(st.Jobs) {
		p := st.Jobs[id]
		if !st.oldPersonOpen(p) || p.DiscoveredFrom == nil {
			continue
		}
		builder, review := *p.DiscoveredFrom, *p.DiscoveredFrom
		if r := st.Jobs[review]; r != nil && isReviewer(r) {
			b := st.reviewedBy(r)
			if b == nil {
				return Plan{Posts: []Post{Control(st.Task, p.ID, string(contract.Cancelled), "the review "+review+" has no builder to decide on", "", nil)}}, true
			}
			builder = b.ID
		} else {
			review = ""
		}
		plan, err := Person(st, builder, review, "", "", "")
		if err != nil {
			continue
		}
		if len(plan.Posts) == 0 {
			// A verdict or an existing decision Job already settles it: close the old card with Person's reason.
			plan = Plan{Posts: []Post{Control(st.Task, p.ID, string(contract.Cancelled), "no decision needed from the owner: "+plan.Note, "", nil)}}
		}
		return plan, true
	}
	return Plan{}, false
}

// oldPersonFrom is the open person Job of the old kind discovered from id.
func (s State) oldPersonFrom(id string) *contract.Job {
	for _, k := range sortedIDs(s.Jobs) {
		if j := s.Jobs[k]; j.DiscoveredFrom != nil && *j.DiscoveredFrom == id && s.oldPersonOpen(j) {
			return j
		}
	}
	return nil
}

// oldPersonOpen is whether p is a person Job of the old kind, still open: an ask titled "Decide how
// ..." whose one row says the owner "decides how <builder> continues", raised before the Judge handed
// these to an agent. The workflow's point gate ("Judge ... before ...") is not of this kind.
func (s State) oldPersonOpen(p *contract.Job) bool {
	if closed(p) || role(p) != "person" || !strings.HasPrefix(str(s.bodyOf(p.ID)["title"]), "Decide how ") {
		return false
	}
	rows, _ := s.bodyOf(p.ID)["acceptance"].([]any)
	for _, e := range rows {
		if row, ok := e.(map[string]any); ok && strings.Contains(str(row["text"]), " decides how ") && strings.HasSuffix(str(row["text"]), " continues") {
			return true
		}
	}
	return false
}

// decisionFrom is the decision Job already raised from id.
func (s State) decisionFrom(id string) *contract.Job {
	for _, k := range sortedIDs(s.Jobs) {
		j := s.Jobs[k]
		if role(j) == "orchestrator" && j.DiscoveredFrom != nil && *j.DiscoveredFrom == id && strings.HasPrefix(str(s.bodyOf(k)["title"]), "Decide how ") {
			return j
		}
	}
	return nil
}

// decisionGuide is the rest of a decision Job's brief: the facts that tell the agent whether
// another rework could bring anything new (the reviews and earlier decisions on the chain) and the
// four choices with the command of each.
func (s State) decisionGuide(builder, discovered, review string) string {
	earlier := 0
	for _, c := range s.chainOf(builder) {
		for _, r := range s.reviewsOf(c) {
			if s.decisionFrom(r.ID) != nil {
				earlier++
			}
		}
	}
	rework := "grid judge rework " + builder + " --from <review>"
	if review != "" {
		rework = "grid judge rework " + builder + " --from " + review
	}
	return fmt.Sprintf("\n\nReview rounds used on this chain: %d. Earlier decision Jobs on it: %d.\n\n"+
		"You decide how %s continues; the owner is not asked. Choose one and say why in your result:\n"+
		"1. A fresh rework: `%s` posts a new builder Job that continues %s (not a resume of it). Choose it only when the reviewer's text holds something the earlier rounds lacked; a rework that repeats the same work with no new information only repeats the rejection.\n"+
		"2. Accept %s with the reason, when what is left does not matter.\n"+
		"3. Re-plan or split: post the Jobs the work needs instead, and cancel %s.\n"+
		"4. Cancel %s, when the work is no longer needed.\n"+
		"Only when the decision truly needs a person (money, access, taste, an outside party) post one real question with `grid question ask`: 2 to 4 options and a recommended default, as the questions skill says. Never ask whether to continue.",
		s.roundsOf(builder), earlier, builder, rework, builder, builder, builder, builder)
}

// decisionBody is the job record of an agent decision Job: role orchestrator, which runs on the
// planner agent (settings.AgentForRole) and so on its model, Opus, with no model of its own here.
func decisionBody(id, builder, from, title, goal string, src map[string]any) map[string]any {
	body := personBody(id, builder, from, title, goal, "", src)
	body["role"] = "orchestrator"
	delete(body, "kind")
	if intent, ok := body["intent"].(map[string]any); ok {
		intent["owner"] = intent["item"]
		// A precise pointer, so the Job lints clean; the prose of goal is an acceptance row below.
		intent["goal"] = "job:" + from + "#verdict"
	}
	body["skills"] = []any{"questions"}
	body["read"] = []any{"intent.md#request", "task.md"}
	body["acceptance"] = []any{
		map[string]any{"id": "A1", "text": "How " + builder + " continues is decided and carried out: a fresh rework, accept with the reason, re-plan or split, or cancel", "traces": "item"},
		map[string]any{"id": "A2", "text": "A real question with 2-4 options and a recommended default is posted only when the decision needs a person; otherwise none is", "traces": "item"},
		map[string]any{"id": "A3", "text": goal, "traces": "item"},
	}
	body["budget"] = map[string]any{"turns_estimate": 6, "reads_before_first_edit": 2}
	body["on_blocked"] = "ask-and-continue"
	body["posts_back"] = []any{"result", "question"}
	return body
}
