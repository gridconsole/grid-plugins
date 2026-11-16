package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// pastedRule flags a Job that copies the text of a standing-rule note it names
// instead of pointing at it (docs/decisions/standing-rules-in-the-brain.md).
const pastedRule = "job.pasted-rule"

// pastedAcceptance is the golden brief with its A1 text replaced. The golden
// brief keeps brain:project/export-format, whose body says "Keep the id column
// before the title column." (testdata/golden/job-brief.golden:58).
func pastedAcceptance(t *testing.T, text string) string {
	t.Helper()
	const old = `"text":"Document the export format and column order."`
	b := brief(t)
	if !strings.Contains(b, old) {
		t.Fatalf("the golden brief no longer has %s", old)
	}
	return strings.Replace(b, old, `"text":"`+text+`"`, 1)
}

func TestPastedRuleTextIsFlagged(t *testing.T) {
	text := pastedAcceptance(t, "Document the export format and column order. Keep the id column before the title column.")
	fs := lint.Check(text, lint.Options{Resolver: testFiles(t)})
	got := seFindings(fs, pastedRule)
	if len(got) != 1 {
		t.Fatalf("want one %s finding, got %+v", pastedRule, fs)
	}
	f := got[0]
	if f.Severity != "warning" || lint.HasErrors(got) {
		t.Errorf("finding %+v; want a warning, never an error that refuses the Job", f)
	}
	if !strings.Contains(f.Message, "acceptance row A1") || !strings.Contains(f.Message, "brain:project/export-format") || !strings.Contains(f.Fix, "grid brain show") {
		t.Errorf("finding %+v; want the row, the note id and the read step", f)
	}
}

func TestPastedRuleNotFlagged(t *testing.T) {
	for _, c := range []struct{ name, text string }{
		// The check in the planner's own words names the note's subject, not its sentence.
		{"own words", "Document the export format; the id column stays first as the kept note fixes."},
		// Part of the note's sentence is a shared phrase, not a pasted rule.
		{"short phrase", "Document the export format and column order, with the id column before the title."},
		// Case and punctuation do not hide a copy, but a different sentence is no copy.
		{"other sentence", "Document the export format and its column order for readers."},
	} {
		fs := lint.Check(pastedAcceptance(t, c.text), lint.Options{Resolver: testFiles(t)})
		if got := seFindings(fs, pastedRule); len(got) != 0 {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}

func TestPastedRuleIgnoresCaseAndPunctuation(t *testing.T) {
	text := pastedAcceptance(t, "Document the export format and column order: KEEP the id column, before the title column")
	if got := seFindings(lint.Check(text, lint.Options{Resolver: testFiles(t)}), pastedRule); len(got) != 1 {
		t.Fatalf("want one %s finding for a re-cased copy, got %+v", pastedRule, got)
	}
}
