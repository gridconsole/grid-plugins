package orchestrator

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// kbSfA1Lead begins A1 of every security Job, and the found record id follows
// it. That text is the one place the id is held in the Job body: job.json allows
// no free field (job-record has additionalProperties false, a link takes only a
// brain, job or person target, and discovered_from is read by gkReplanned as the
// Job being re-planned), and A1 is a free string the contract already requires.
const kbSfA1Lead = "Security found record "

// kbSfTitleMax is the most runes of the finding text the title holds.
const kbSfTitleMax = 120

// kbSecurityFound posts one orchestrator Job for each security `found` record in
// the folded Wakes whose Job is a Job of the fold. Without it the only reader of
// such a record is a lead that is running and reads its events: a lead that is
// not running, or that ends its turn, leaves no Job behind and the task carries
// on as if nothing was found. It is guReplanRejected for a found record.
//
// A found record is posted once. The Job already on the Feed that holds the
// record id in A1 (role orchestrator, kbSfPosted) is the record that says so,
// read from the snapshot under the task lock, so a later tick and a restarted
// process post nothing more. A kind other than security, a found for a Job the
// fold does not hold and a found whose body is a ref (not read yet) post nothing.
// It posts a Job and nothing more: it refuses, cancels and counts nothing.
func (s Store) kbSecurityFound(snap Snapshot) error {
	for _, id := range snap.State.Wakes {
		r, ok := snap.Records[id]
		if !ok || r.Type != "found" {
			continue
		}
		kind, _ := r.Body["kind"].(string)
		job, _ := r.Body["job"].(string)
		if kind != "security" || snap.State.Jobs[job] == nil || kbSfPosted(snap, id) {
			continue
		}
		at, _ := r.Body["at"].(string)
		text, _ := r.Body["text"].(string)
		next := gkNextJobID(snap)
		if _, err := s.Feed.Post(feed.Input{Type: "job", Job: next, Body: SecurityFoundJobBody(next, id, job, at, text)}, s.Writer); err != nil {
			return err
		}
		// The next found record counts this Job when it picks its id.
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
	return nil
}

// kbSfPosted reports whether the Feed holds the security Job of a found record:
// an orchestrator Job whose A1 begins with kbSfA1Lead and the record id. It does
// not read discovered_from, which belongs to the re-plan of a Job (gkReplanned).
func kbSfPosted(snap Snapshot, found string) bool {
	lead := kbSfA1Lead + found + " ("
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

// SecurityFoundJobBody is the job-record of the orchestrator Job that decides
// what a security found record on a Job needs: a fix Job, a cancel with a reason
// or a Job for a person. It is the body ReplanJobBody builds, with the title and
// the one acceptance row of the finding, and with discovered_from null:
// gkReplanned reads any orchestrator Job with discovered_from <job> as the
// re-plan of <job>, so a pointer here would stop the re-plan after <job> is
// rejected or cancelled. The found record id is in A1 (kbSfA1Lead). Like the
// re-plan Job it carries no job: pointer.
func SecurityFoundJobBody(id, found, job, at, text string) map[string]any {
	body := ReplanJobBody(id, job, "", nil)
	finding := kbSfOneLine(text)
	body["title"] = "Security finding on " + job + ": " + kbSfCut(finding, kbSfTitleMax)
	body["discovered_from"] = nil
	a1 := fmt.Sprintf("%s%s (at %s) on %s is a security finding: %s. Decide what it needs and post the result: a fix Job for it, a cancel of %s with a reason, or a Job for a person; the decision is posted with `grid job result`.",
		kbSfA1Lead, found, strings.TrimSpace(at), job, gqCause(finding), job)
	body["acceptance"] = []map[string]any{{"id": "A1", "text": a1, "traces": "owner"}}
	return body
}

// kbSfOneLine reads text as one line: its words, each separated by one space.
func kbSfOneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// kbSfCut cuts s to at most max runes.
func kbSfCut(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
