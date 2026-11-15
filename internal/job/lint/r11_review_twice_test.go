package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// r11Rule flags a review Job the orchestrator plans beside the review the worker
// posts by itself after a Job's result (eval rounds 10 and 11, P5: E2 and E3 got
// a planned reviewer J-3 beside the worker's J-2.1, so one change was reviewed twice).
const r11Rule = "job.review-twice"

// r11Review is the golden brief turned into a review Job: role, after,
// discovered_from and an optional review_reason replaced or added.
func r11Review(t *testing.T, role, after, from, reason string) string {
	t.Helper()
	text := brief(t)
	for _, old := range []string{"\nrole: \"writer\"\n", "\nafter: []\n", "\ndiscovered_from: null\n"} {
		if !strings.Contains(text, old) {
			t.Fatalf("the golden brief no longer has %q", strings.TrimSpace(old))
		}
	}
	text = strings.Replace(text, "\nrole: \"writer\"\n", "\nrole: \""+role+"\"\n", 1)
	text = strings.Replace(text, "\nafter: []\n", "\nafter: "+after+"\n", 1)
	text = strings.Replace(text, "\ndiscovered_from: null\n", "\ndiscovered_from: "+from+"\n", 1)
	if reason != "" {
		text = strings.Replace(text, "\nrisk: []\n", "\nrisk: []\nreview_reason: \""+reason+"\"\n", 1)
	}
	return text
}

func TestR11PlannedReviewOfABuildIsFlagged(t *testing.T) {
	// The E2 J-3 shape (tally T-02 jobs/J-3/job.yaml:5,17,58,65): role reviewer,
	// after names the build J-2, discovered_from null, no review_reason.
	for _, role := range []string{"reviewer", "grid-reviewer", "checker"} {
		for _, after := range []string{`["J-2"]`, `[{"job":"J-2","kind":"done","why":"The review checks the Total change that J-2 builds."}]`} {
			fs := lint.Check(r11Review(t, role, after, "null", ""), lint.Options{Resolver: testFiles(t)})
			got := seFindings(fs, r11Rule)
			if len(got) != 1 {
				t.Fatalf("role %s after %s: want one %s finding, got %+v", role, after, r11Rule, fs)
			}
			f := got[0]
			if f.Severity != "warning" || f.Line != 6 {
				t.Errorf("finding %+v; want severity warning on line 6 (the role line)", f)
			}
			if !strings.Contains(f.Message, "J-2") || !strings.Contains(f.Fix, "review_reason") {
				t.Errorf("finding %+v; want the message to name J-2 and the fix to name review_reason", f)
			}
			if lint.HasErrors(got) {
				t.Errorf("the finding is an error, so it would refuse the Job: %+v", f)
			}
		}
	}
}

func TestR11ReviewNotFlagged(t *testing.T) {
	for _, c := range []struct{ name, role, after, from, reason string }{
		// The worker's own review, the E2 J-2.1 shape (jobs/J-2.1/job.yaml:5,43): after [], discovered_from J-2.
		{"worker review", "reviewer", `[]`, `"J-2"`, ""},
		// A review the person asked for on top says so in review_reason.
		{"review on top", "reviewer", `["J-2"]`, "null", "The person asked for a second review of the money handling."},
		// A builder that waits on a build is no review.
		{"builder after a build", "builder", `["J-2"]`, "null", ""},
		// A reviewer that waits only on a person's answer names no Job.
		{"reviewer on a gate", "reviewer", `["gate:Q-7"]`, "null", ""},
	} {
		fs := lint.Check(r11Review(t, c.role, c.after, c.from, c.reason), lint.Options{Resolver: testFiles(t)})
		if got := seFindings(fs, r11Rule); len(got) != 0 {
			t.Errorf("%s: %+v", c.name, got)
		}
		// The Job was read whole: a brief that failed to decode would also show no finding.
		if lint.HasErrors(fs) {
			t.Errorf("%s: the brief does not lint clean, so the case proves nothing: %+v", c.name, fs)
		}
	}
}
