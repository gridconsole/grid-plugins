package orchestrator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// gkReplanRole is the role of the Job that re-plans after a cancel.
const gkReplanRole = "orchestrator"

// gkReplan posts one orchestrator Job for each Job its worker cancelled while
// other Jobs still wait on it with a done edge. Without it those Jobs wait for
// ever: a done edge is released only by an accepted Job (jobgraph.Released) and
// nothing else reacts to the cancel. The Job is posted once: the re-plan Job
// already on the Feed (role orchestrator, discovered_from the cancelled Job) is
// the record that says so, read from the snapshot under the task lock. A Job
// cancelled by a control alone, with no worker-exit, was decided by someone and
// gets nothing. It posts a Job and nothing more: it refuses, cancels and counts
// nothing.
func (s Store) gkReplan(snap Snapshot) error {
	for _, id := range ids(snap) {
		j := snap.State.Jobs[id]
		if j.Status != contract.Cancelled || j.ExitReason != "cancelled" {
			continue
		}
		waiting := gkWaiting(snap, id)
		if len(waiting) == 0 || gkReplanned(snap, id) {
			continue
		}
		next := gkNextJobID(snap)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: ReplanJobBody(next, id, gqCauseText(j.ExitCode), waiting)}, s.Writer); err != nil {
			return err
		}
		// The next cancelled Job counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// gkWaiting lists the Jobs that are still Posted or Ready and hold a done edge
// (a bare id or kind done, as jobgraph reads it) to target.
func gkWaiting(snap Snapshot, target string) []string {
	var out []string
	for _, it := range snap.Items {
		if st := snap.State.Jobs[it.ID]; st == nil || st.Status != contract.Posted && st.Status != contract.Ready {
			continue
		}
		for _, e := range it.After {
			if e.Job == target && (e.Kind == "" || e.Kind == jobgraph.Done) {
				out = append(out, it.ID)
				break
			}
		}
	}
	return out
}

// gkReplanned reports whether the Feed holds a re-plan Job for the cancelled Job.
func gkReplanned(snap Snapshot, cancelled string) bool {
	for _, it := range snap.Items {
		if strings.EqualFold(strings.TrimSpace(it.Role), gkReplanRole) && it.DiscoveredFrom != nil && *it.DiscoveredFrom == cancelled {
			return true
		}
	}
	return false
}

// gkNextJobID is the next free top-level J-<n>, counting the Jobs of the fold
// and the id of every job record the Feed holds, even one the fold holds out of
// State (snap.JobIDs). The rule is that of nextJobID in internal/cli, which this
// package cannot import.
func gkNextJobID(snap Snapshot) string {
	highest := 0
	count := func(id string) {
		if n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-")); err == nil && n > highest {
			highest = n
		}
	}
	for _, it := range snap.Items {
		count(it.ID)
	}
	for id := range snap.State.Jobs {
		count(id)
	}
	for _, id := range snap.JobIDs {
		count(id)
	}
	return "J-" + strconv.Itoa(highest+1)
}

// gqCauseMax is the most bytes of a cause the re-plan Job text holds.
const gqCauseMax = 500

// gqCauseText reads a folded exit code as text: a JSON string unquoted, a number
// as its digits, nothing for no code.
func gqCauseText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// gqCause trims a cause and cuts it to gqCauseMax bytes, never inside a rune.
func gqCause(cause string) string {
	cause = strings.TrimSpace(cause)
	if len(cause) <= gqCauseMax {
		return cause
	}
	cut := gqCauseMax
	for cut > 0 && !utf8.RuneStart(cause[cut]) {
		cut--
	}
	return cause[:cut]
}

// ReplanJobBody is the job-record of the orchestrator Job that re-plans after a
// Job was cancelled while the Jobs in waiting hold a done edge to it. It has the
// fields of the first Job of a task (internal/cli firstJob) and reads the task's
// request. It carries no job: pointer: a Job that never started, or was refused
// at render, has no jobs/<id>/brief.md to point at, and lint refuses such a
// pointer. The Jobs are named in the title and the acceptance text only.
//
// cause is the cancelled Job's worker-exit code as text, empty when it has none.
// It becomes one sentence of the acceptance text, cut to gqCauseMax bytes. It is
// not put in intent.goal: that is a pointer render resolves.
func ReplanJobBody(id, cancelled, cause string, waiting []string) map[string]any {
	request := "intent.md#request"
	list := strings.Join(waiting, ", ")
	reason := jobgraph.DoneReason(cancelled, contract.Cancelled)
	each := make([]string, 0, len(waiting))
	for _, w := range waiting {
		each = append(each, w+" ("+reason+")")
	}
	text := fmt.Sprintf("No Job waits on %s any more: each of %s is cancelled with a reason or replaced by a new Job that does not wait on %s; the work of %s is posted again as a new Job that `grid job lint` passes when the task still needs it; a result is posted with `grid job result`.",
		cancelled, strings.Join(each, ", "), cancelled, cancelled)
	if c := gqCause(cause); c != "" {
		text += " Stopped because: " + c
	}
	return map[string]any{
		"id": id, "item": 1,
		"title":  fmt.Sprintf("Re-plan after %s was cancelled: %s wait on it", cancelled, list),
		"role":   gkReplanRole,
		"skills": []string{"plan-jobs"},
		"risk":   []string{}, "max_rounds": 1,
		"intent": map[string]any{"owner": request, "item": request, "goal": request},
		"links":  []any{}, "keep": []string{}, "decisions": []string{},
		"read": []string{request}, "write": []string{}, "out_of_scope": []string{},
		"acceptance": []map[string]any{{"id": "A1", "text": text, "traces": "owner"}},
		"verify":     map[string]any{"kind": "file-exists", "path": "task:task.md"},
		"budget":     map[string]any{"turns_estimate": 10, "reads_before_first_edit": 3},
		"on_blocked": "ask-and-continue",
		"after":      []any{}, "attempt": 1, "continues": nil, "discovered_from": cancelled, "close_reason": nil,
		"checkpoint": nil, "review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []string{"result", "question"},
	}
}
