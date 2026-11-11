package question

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// JobKind marks the person Job that `grid question ask` posts for a question.
const JobKind = "ask"

// JobPort is how the store reaches the person Job of a question. The question
// package cannot import the orchestrator (which imports this package), so the
// CLI wires the port to it. A nil port keeps the question a plain Feed record.
type JobPort struct {
	// Post writes the person Job and returns its id. It returns "" and no error
	// when neither writer may post a Job, so an agent's ask is never refused: the
	// question is still posted, only without a Job behind it.
	Post func(f feed.Store, lease, person feed.Writer, body map[string]any) (string, error)
	// Close gives the person Job its result so the Jobs that list it in their
	// after list carry on. A Job that is not a question Job, or is already
	// closed, is left alone.
	Close func(f feed.Store, lease, person feed.Writer, id, result string) error
}

// jobBody is the person Job record of a question. A Job record has no field for
// choices, so the question (with its options) stays on the question post that
// names this Job in its job field; the Job holds who is asked and what.
func jobBody(qid string, in Input) map[string]any {
	title := qid + ": " + strings.Join(strings.Fields(in.Question), " ")
	if r := []rune(title); len(r) > 120 {
		title = string(r[:119]) + "…"
	}
	item := in.Item
	if item < 1 {
		item = 1
	}
	return map[string]any{
		"item": item, "title": title, "role": "Person", "kind": JobKind,
		"skills": []any{}, "review": "L0", "risk": []any{}, "max_rounds": 1,
		"intent": map[string]any{"owner": in.Who, "item": "questions/" + qid + ".md", "goal": strings.TrimSpace(in.Question)},
		"links":  []any{}, "keep": []any{}, "decisions": []any{}, "read": []any{}, "write": []any{}, "out_of_scope": []any{},
		"acceptance": []any{map[string]any{"id": "A1", "text": fmt.Sprintf("%s answers %s", in.Who, qid), "traces": "item"}},
		// The file is the read-only view of this Job; its existence is the check.
		"verify":     map[string]any{"kind": "file-exists", "path": "questions/" + qid + ".md"},
		"budget":     map[string]any{"turns_estimate": 1, "reads_before_first_edit": 0},
		"on_blocked": "ask-and-continue", "after": []any{},
		"attempt": 1, "continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": "none", "posts_back": []any{},
	}
}

// postJob posts the person Job of a new question and returns its id, or "" when
// there is no port or no writer that may post one.
func (s Store) postJob(f feed.Store, qid string, in Input) (string, error) {
	if s.Jobs.Post == nil {
		return "", nil
	}
	// The Feed refuses an agent a person gate; post no Job the question post
	// would then leave without its question.
	if role := contract.NormalizeRole(s.Writer.Author.Role); role == "agent" || role == "cli" {
		return "", nil
	}
	return s.Jobs.Post(f, s.ControlWriter, s.Writer, jobBody(qid, in))
}

// settleJob closes the question's person Job from the last answer, default or
// withdrawal in the history, so the Feed holds one decision and the Job follows
// it. It runs again on a repeated command, so a crash between the two posts is
// repaired by the retry.
func (s Store) settleJob(f feed.Store, r Record) error {
	if s.Jobs.Close == nil || r.Job == "" {
		return nil
	}
	for i := len(r.History) - 1; i >= 0; i-- {
		e := r.History[i]
		switch e.Kind {
		case "answered", "defaulted":
			return s.Jobs.Close(f, s.ControlWriter, s.Writer, r.Job, e.Text)
		case "withdrawn":
			return s.Jobs.Close(f, s.ControlWriter, s.Writer, r.Job, "withdrawn: "+e.Text)
		}
	}
	return nil
}
