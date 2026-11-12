package orchestrator

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// pfMsgTitleRunes is the most runes of a message the Job title quotes.
const pfMsgTitleRunes = 80

// pfMsgA1Lead begins A1 of every Job that reads a person's message, and the post
// id follows it. discovered_from takes only a Job id (job-record), so the post id
// is held in A1, as kbSfA1Lead holds a found record id.
const pfMsgA1Lead = "Person message post "

// pfMsgReplan posts one orchestrator Job for each message a person typed to the
// task with no Job named (`grid feed post <task> --type post`: kind message, ref
// task, no job field). Nothing else reads such a post, so without it the message
// stays unread. The Job is posted once: the orchestrator Job already on the Feed
// that holds the post id in A1 (pfMsgPosted) is the record that says so, read
// from the snapshot under the task lock. It posts a Job and nothing more: it
// refuses, cancels and counts nothing.
func (s Store) pfMsgReplan(snap Snapshot) error {
	for _, id := range pfMsgRequests(snap) {
		if pfMsgPosted(snap, id) {
			continue
		}
		next := gkNextJobID(snap)
		text := strings.TrimSpace(fmt.Sprint(snap.Records[id].Body["text"]))
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: PfMsgJobBody(next, id, text)}, s.Writer); err != nil {
			return err
		}
		// The next message counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// pfMsgRequests lists, oldest first, the ids of the posts that are a person's
// request to the task: kind message, ref task, no Job, not addressed `To J-`,
// written by owner, member or lead (a guest's request goes by mail), not the
// tick's own note (Lease "task"), and not an answer notice (a control with Re
// that post follows it).
func pfMsgRequests(snap Snapshot) []string {
	answered := map[string]bool{}
	for _, r := range snap.Records {
		if r.Type == "control" && r.Re != nil {
			answered[*r.Re] = true
		}
	}
	var out []feed.Record
	for _, r := range snap.Records {
		if !pfMsgIsRequest(r) || answered[r.ID] {
			continue
		}
		out = append(out, r)
	}
	// Record ids do not sort by time; the feed sequence does.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Seq < out[j-1].Seq; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	ids := make([]string, 0, len(out))
	for _, r := range out {
		ids = append(ids, r.ID)
	}
	return ids
}

func pfMsgIsRequest(r feed.Record) bool {
	if r.Type != "post" || r.Job != nil || r.Body["kind"] != "message" || r.Body["ref"] != "task" {
		return false
	}
	if r.Lease != nil && *r.Lease == "task" {
		return false
	}
	if contract.TierFor(r.Author.Role) != contract.PersonTeam {
		return false
	}
	text, ok := r.Body["text"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		return false
	}
	return !strings.HasPrefix(text, "To J-")
}

// pfMsgTitle is the title of the Job: the first pfMsgTitleRunes runes of the
// message on one line.
func pfMsgTitle(text string) string {
	line := []rune(strings.Join(strings.Fields(text), " "))
	if len(line) > pfMsgTitleRunes {
		line = line[:pfMsgTitleRunes]
	}
	return "Read the person's message: " + string(line)
}

// pfMsgPosted reports whether the Feed holds the Job of a message: an
// orchestrator Job whose A1 begins with pfMsgA1Lead and the post id. It does not
// read discovered_from, which belongs to the re-plan of a Job (gkReplanned).
func pfMsgPosted(snap Snapshot, post string) bool {
	lead := pfMsgA1Lead + post + " ("
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

// PfMsgJobBody is the job-record of the orchestrator Job that reads a person's
// message to the task. It has the field set of ReplanJobBody, with
// discovered_from null (job-record takes only a Job id there, and gkReplanned
// reads an orchestrator Job's discovered_from as the Job being re-planned). The
// post id is the lead of A1 (pfMsgA1Lead).
func PfMsgJobBody(id, post, text string) map[string]any {
	body := ReplanJobBody(id, post, "", nil)
	body["title"] = pfMsgTitle(text)
	body["discovered_from"] = nil
	body["acceptance"] = []map[string]any{{"id": "A1", "text": fmt.Sprintf("%s%s (a message to the task) reads: %q. The next Jobs it needs are posted as new Jobs that `grid job lint` passes, or a question is asked of the person with `grid question`; a result is posted with `grid job result`.", pfMsgA1Lead, post, text), "traces": "owner"}}
	return body
}
