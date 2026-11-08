package feed

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// QuestionDecision is the person-gate projection of admitted Feed records.
// Explicit task controls still select between conflicting answer candidates.
// A single distinct human answer closes the gate without acquiring a lease.
type QuestionDecision struct {
	Question      Record
	State, Answer string
	Aging         int
	Events        []Record
}

// foldQuestions extends the shared fold for person gates without changing Job
// authority or contract verdicts. Task controls retain tenure order; only
// gate deadlines use the read time.
func foldQuestions(read ReadResult, state *contract.State, now time.Time, allowed func(string) bool) map[string]QuestionDecision {
	records := map[string]Record{}
	for _, e := range read.Records {
		if e.Verdict == "counted" {
			records[e.ID] = e.Record
		}
	}
	decisions := map[string]QuestionDecision{}
	for id, q := range state.Questions {
		post, ok := records[id]
		if !ok || str(object(post.Body["gate"])["kind"]) != "person" {
			continue
		}
		d := QuestionDecision{Question: post, State: "open", Events: []Record{}}
		gate := object(post.Body["gate"])
		deadline, deadlineErr := time.Parse(time.RFC3339Nano, str(gate["at"]))
		due := deadlineErr == nil && !now.Before(deadline) && str(gate["default"]) != ""
		controls := []Record{}
		for _, p := range records {
			if p.Type == "control" && p.Lease != nil && *p.Lease == "task" && p.Fence > 0 && str(p.Body["target"]) == id && value(p.Re) == id && str(p.Body["question_event"]) != "" {
				controls = append(controls, p)
			}
		}
		sort.Slice(controls, func(i, j int) bool {
			a, b := controls[i], controls[j]
			if a.Gen != b.Gen {
				return a.Gen < b.Gen
			}
			if a.Fence != b.Fence {
				return a.Fence < b.Fence
			}
			if a.Worker != b.Worker {
				return a.Worker < b.Worker
			}
			return a.Seq < b.Seq
		})
		selected := ""
		if q.Applied != nil && PersonAnswer(records[*q.Applied], q.Who) {
			selected = *q.Applied
		} else {
			answer, conflict := "", false
			for _, aid := range q.Answers {
				p, ok := records[aid]
				if !ok {
					continue
				}
				if !PersonAnswer(p, q.Who) {
					continue
				}
				text := str(p.Body["answer"])
				if selected != "" && text != answer {
					conflict = true
				}
				if selected == "" {
					selected, answer = aid, text
				}
			}
			if conflict {
				selected = ""
			}
		}

		// Keep person events in Feed order and substitute tenure-sorted task
		// controls into their existing slots. Never sort all events by fences:
		// leaseless person records have fence zero regardless of arrival.
		events := []Record{}
		nextControl := 0
		for _, entry := range read.Records {
			p := entry.Record
			if entry.Verdict != "counted" {
				continue
			}
			if p.Type == "control" && p.Lease != nil && *p.Lease == "task" && p.Fence > 0 && str(p.Body["target"]) == id && value(p.Re) == id && str(p.Body["question_event"]) != "" {
				events = append(events, controls[nextControl])
				nextControl++
			} else if p.ID == selected || (personWithdraw(p, allowed) && value(p.Re) == id && str(p.Body["target"]) == id) {
				events = append(events, p)
			}
		}
		q.Applied = nil
		for _, p := range questionEventOrder(events) {
			if p.Type == "answer" {
				// A person may still override a default, as before.
				if d.State != "withdrawn" {
					d.State, d.Answer = "answered", str(p.Body["answer"])
					d.Events = append(d.Events, p)
					answerID := p.ID
					q.Applied = &answerID
				}
				continue
			}
			if personWithdraw(p, allowed) {
				at, err := time.Parse(time.RFC3339Nano, p.At)
				if d.State == "open" && deadlineErr == nil && err == nil && !at.Before(deadline) && str(gate["default"]) != "" {
					d.State, d.Answer = "defaulted", str(gate["default"])
				}
			}
			if d.State != "open" {
				continue
			}
			switch str(p.Body["question_event"]) {
			case "aged":
				level, _ := p.Body["level"].(float64)
				if post.Body["blocking"] != true || int(level) != d.Aging+1 {
					continue
				}
				d.Aging++
			case "defaulted":
				at, err := time.Parse(time.RFC3339Nano, p.At)
				if deadlineErr != nil || err != nil || at.Before(deadline) || str(p.Body["text"]) != str(gate["default"]) {
					continue
				}
				d.State, d.Answer = "defaulted", str(p.Body["text"])
			case "withdrawn":
				d.State = "withdrawn"
			default:
				continue
			}
			d.Events = append(d.Events, p)
		}
		if d.State == "open" && due {
			d.State, d.Answer = "defaulted", str(gate["default"])
		}
		q.Open = d.State == "open"
		if q.Job != nil {
			if job := state.Jobs[*q.Job]; job != nil && job.Status == contract.Cancelled {
				q.Open = false
			}
		}
		decisions[id] = d
	}
	// Rebuild indexes because the person layer can also reject a contract-selected
	// non-person answer. Contract verdicts remain untouched.
	state.NeedsYou = []string{}
	for _, job := range state.Jobs {
		job.OpenQuestions = []string{}
	}
	for id, q := range state.Questions {
		if !q.Open {
			continue
		}
		state.NeedsYou = append(state.NeedsYou, id)
		if q.Job != nil {
			if job := state.Jobs[*q.Job]; job != nil {
				job.OpenQuestions = append(job.OpenQuestions, id)
			}
		}
	}
	sort.Strings(state.NeedsYou)
	for _, job := range state.Jobs {
		sort.Strings(job.OpenQuestions)
	}
	return decisions
}

// PersonAnswer reports whether p is an answer a person gate addressed to who admits.
func PersonAnswer(p Record, who string) bool {
	if p.Type != "answer" {
		return false
	}
	switch p.Author.Role {
	case "owner", "member", "lead":
		return true
	case "guest":
		return p.Author.ID == who
	default:
		return false
	}
}

// FoldReadQuestions uses the same admitted snapshot for state and question projections.
func FoldReadQuestions(read ReadResult) (contract.State, map[string]QuestionDecision) {
	return FoldReadAt(read, time.Now())
}

// FoldReadAt projects person gates at a specified read time without writing events.
// An optional callback replaces the default withdrawal role policy.
func FoldReadAt(read ReadResult, now time.Time, withdrawAllowed ...func(string) bool) (contract.State, map[string]QuestionDecision) {
	state := contract.Fold(posts(read))
	allowed := DefaultWithdrawAllowed
	if len(withdrawAllowed) > 0 && withdrawAllowed[0] != nil {
		allowed = withdrawAllowed[0]
	}
	questions := foldQuestions(read, &state, now, allowed)
	return state, questions
}

// DefaultWithdrawAllowed is the shared default for person withdrawal policy.
func DefaultWithdrawAllowed(role string) bool {
	return slices.Contains(workflow.DefaultQuestionWithdrawRoles(), role)
}

func personWithdraw(p Record, allowed func(string) bool) bool {
	return p.Type == "control" && p.Lease == nil && p.Fence == 0 && p.Gen == 0 &&
		str(p.Body["question_event"]) == "withdrawn" &&
		(p.Author.Role == "owner" || p.Author.Role == "member" || p.Author.Role == "lead" || p.Author.Role == "guest") && allowed(p.Author.Role)
}

// QuestionWithdrawPolicy reads the task's workflow policy. Hosts can instead
// supply a callback to FoldReadAt and question.Store.WithdrawAllowed. An unreadable
// policy falls back to defaults with one warning per read.
func QuestionWithdrawPolicy(taskDir string) func(string) bool {
	roles, err := workflow.GatePostRoles(taskDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot read task withdrawal policy; using defaults: %v\n", err)
		return DefaultWithdrawAllowed
	}
	return func(role string) bool { return slices.Contains(roles["withdraw"], role) }
}

// questionEventOrder preserves Feed order except for explicit causal links.
// Tenure order has already been established for the task-control slots.
func questionEventOrder(events []Record) []Record {
	pending := map[string]bool{}
	for _, p := range events {
		pending[p.ID] = true
	}
	ordered := make([]Record, 0, len(events))
	for len(ordered) < len(events) {
		advanced := false
		for i, p := range events {
			if !pending[p.ID] {
				continue
			}
			ready := true
			if p.Lease != nil && *p.Lease == "task" {
				for _, prior := range events[:i] {
					if prior.Lease != nil && *prior.Lease == "task" && pending[prior.ID] {
						ready = false
						break
					}
				}
			}
			for _, id := range p.After {
				if pending[id] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			ordered = append(ordered, p)
			delete(pending, p.ID)
			advanced = true
			break
		}
		if !advanced {
			break
		} // Cyclic dependencies do not close a gate.
	}
	return ordered
}
