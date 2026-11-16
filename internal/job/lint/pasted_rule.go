package lint

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// pastedRuleWords is the shortest run of a note's words that counts as pasted:
// a shorter one is a shared phrase, not a copied rule.
const pastedRuleWords = 6

// pastedRule warns when the Job's own words (title, acceptance rows, link and
// after reasons, review_reason) copy a sentence of a brain note the Job names in
// links, keep or decisions. A standing rule is pointed at, never pasted
// (docs/decisions/standing-rules-in-the-brain.md): the brief renders the note
// itself, so a pasted copy fills the worker's context twice and goes stale when
// the note changes. A warning only: the Job still renders and runs.
func (c *checker) pastedRule(j job.Job, o Options) {
	if o.Resolver == nil {
		return
	}
	type field struct{ key, label, text string }
	fields := []field{{"title", "title", j.Title}}
	for _, a := range j.Acceptance {
		fields = append(fields, field{"acceptance", "acceptance row " + a.ID, a.Text})
	}
	for _, l := range j.Links {
		fields = append(fields, field{"links", "the why of the link to " + l.To, l.Why})
	}
	for _, d := range j.After {
		fields = append(fields, field{"after", "the why of after " + d.Job, d.Why})
	}
	fields = append(fields, field{"review_reason", "review_reason", j.ReviewReason})
	ids, seen := []string{}, map[string]bool{}
	for _, l := range j.Links {
		if strings.HasPrefix(l.To, "brain:") && !seen[l.To] {
			seen[l.To] = true
			ids = append(ids, l.To)
		}
	}
	for _, id := range append(append([]string{}, j.Keep...), j.Decisions...) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		s, e := o.Resolver.Resolve(id)
		if e != nil || s.PointerOnly {
			continue // a broken pointer is job.pointer's finding; a personal note is never in the brief
		}
		sentences := noteSentences(s.Text)
		for _, f := range fields {
			text := " " + wordRun(f.text) + " "
			for _, sentence := range sentences {
				if strings.Contains(text, " "+sentence+" ") {
					c.addAt("job.pasted-rule", c.line(f.key), "warning",
						fmt.Sprintf("The %s copies text of %s: %q", f.label, id, sentence),
						"Keep the note id in keep or links with a why, and state the check in your own words; the brief shows the note and the worker reads it with grid brain show.")
					break
				}
			}
		}
	}
}

// noteSentences splits a note's text into lower-case word runs, one per line or
// sentence, keeping those of at least pastedRuleWords words.
func noteSentences(text string) []string {
	out := []string{}
	for _, line := range strings.Split(text, "\n") {
		for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == '.' || r == ';' || r == '!' || r == '?' }) {
			run := wordRun(part)
			if len(strings.Fields(run)) >= pastedRuleWords {
				out = append(out, run)
			}
		}
	}
	return out
}

// wordRun is the words of s, lower-cased and joined by single spaces, so a copy
// matches whatever its punctuation, quoting or line breaks.
func wordRun(s string) string {
	return strings.Join(words.FindAllString(strings.ToLower(s), -1), " ")
}
