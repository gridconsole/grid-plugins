package feed

import (
	"sort"

	"github.com/gridconsole/grid-plugins/internal/contract"
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
// authority. Times are display data; task controls retain tenure order.
func foldQuestions(read ReadResult, state *contract.State) map[string]QuestionDecision {
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
		for _, p := range controls {
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
				if str(p.Body["text"]) != str(object(post.Body["gate"])["default"]) {
					continue
				}
				d.State, d.Answer = "defaulted", str(p.Body["text"])
			case "withdrawn":
				d.State = "withdrawn"
			default:
				continue
			}
			d.Events = append(d.Events, p)
			for i := range state.Verdicts {
				if state.Verdicts[i].ID == p.ID {
					state.Verdicts[i].Kind, state.Verdicts[i].Reason = "counted", ""
				}
			}
		}
		selected := ""
		if q.Applied != nil {
			selected = *q.Applied
		} else if d.State != "withdrawn" {
			answer, conflict := "", false
			for _, aid := range q.Answers {
				p, ok := records[aid]
				if !ok {
					continue
				}
				// The question-file contract records these caller roles.
				switch p.Author.Role {
				case "owner", "member", "guest", "lead", "agent", "cli":
				default:
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
		if p, ok := records[selected]; ok {
			d.State, d.Answer = "answered", str(p.Body["answer"])
			d.Events = append(d.Events, p)
			q.Applied = &p.ID
		}
		q.Open = q.Open && d.State == "open"
		decisions[id] = d
	}
	state.NeedsYou = filterOpen(state.NeedsYou, state.Questions)
	for _, job := range state.Jobs {
		job.OpenQuestions = filterOpen(job.OpenQuestions, state.Questions)
	}
	return decisions
}

func filterOpen(ids []string, questions map[string]*contract.Question) []string {
	out := []string{}
	for _, id := range ids {
		if q := questions[id]; q != nil && q.Open {
			out = append(out, id)
		}
	}
	return out
}

// FoldRead uses the same admitted snapshot for state and question projections.
func FoldRead(read ReadResult) (contract.State, map[string]QuestionDecision) {
	state := contract.Fold(posts(read))
	questions := foldQuestions(read, &state)
	return state, questions
}
