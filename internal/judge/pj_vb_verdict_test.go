package judge

import (
	"reflect"
	"testing"
)

// Ported from the app's internal/kernel/verdict/verdict_test.go:8, case for case.
func TestVerdictParse(t *testing.T) {
	cases := []struct {
		name    string
		body    map[string]any
		verdict string
		text    string
		mustFix []string
	}{
		{"nil body", nil, "", "", nil},
		{"accepted outcome", map[string]any{"outcome": "ACCEPTED\nAll rows hold."}, "ACCEPTED", "ACCEPTED\nAll rows hold.", nil},
		{"rejected with punctuation and case", map[string]any{"outcome": "  **rejected.**  "}, "REJECTED", "**rejected.**", nil},
		{"markdown heading", map[string]any{"outcome": "# Accepted\nfine"}, "ACCEPTED", "# Accepted\nfine", nil},
		{"older key verdict", map[string]any{"verdict": "REJECTED"}, "REJECTED", "REJECTED", nil},
		{"older key summary", map[string]any{"summary": "ACCEPTED"}, "ACCEPTED", "ACCEPTED", nil},
		{"outcome wins over text", map[string]any{"outcome": "ACCEPTED", "text": "REJECTED"}, "ACCEPTED", "ACCEPTED", nil},
		{"blank outcome falls to text", map[string]any{"outcome": "  ", "text": "REJECTED"}, "REJECTED", "REJECTED", nil},
		{"not a verdict", map[string]any{"outcome": "Looks fine to me"}, "", "Looks fine to me", nil},
		{"must_fix strings, blanks dropped", map[string]any{"outcome": "REJECTED", "must_fix": []any{" row 2 fails ", "", "row 3"}}, "REJECTED", "REJECTED", []string{"row 2 fails", "row 3"}},
		{"must_fix objects", map[string]any{"outcome": "REJECTED", "must_fix": []any{
			map[string]any{"row": "A2", "at": "x.go:4"},
			map[string]any{"row": "A3"},
			map[string]any{"at": "y.go:9"},
			map[string]any{"row": " ", "at": ""},
		}}, "REJECTED", "REJECTED", []string{"A2: x.go:4", "A3", "y.go:9"}},
		{"must_fix mixed shapes", map[string]any{"outcome": "REJECTED", "must_fix": []any{"one", map[string]any{"row": "A2", "at": "f:1"}, 7}}, "REJECTED", "REJECTED", []string{"one", "A2: f:1"}},
		{"must_fix list wins over the block", map[string]any{"outcome": "REJECTED\nMust-fix:\n- from block", "must_fix": []any{"from list"}}, "REJECTED", "REJECTED\nMust-fix:\n- from block", []string{"from list"}},
		{"empty must_fix falls to the block", map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 2 fails", "must_fix": []any{}}, "REJECTED", "REJECTED\nMust-fix:\n- row 2 fails", []string{"- row 2 fails"}},
		{"block lines until a blank line", map[string]any{"outcome": "REJECTED\nMust-fix:\n- a\n- b\n\nafter the blank"}, "REJECTED", "REJECTED\nMust-fix:\n- a\n- b\n\nafter the blank", []string{"- a", "- b"}},
		{"block on the heading line", map[string]any{"outcome": "REJECTED\nMust fix: row 1\n- row 2"}, "REJECTED", "REJECTED\nMust fix: row 1\n- row 2", []string{"row 1", "- row 2"}},
		{"block ends at Notes:", map[string]any{"outcome": "REJECTED\nMust-fix:\n- a\nNotes:\n- not this"}, "REJECTED", "REJECTED\nMust-fix:\n- a\nNotes:\n- not this", []string{"- a"}},
		{"block ends at Rows:", map[string]any{"outcome": "REJECTED\nmust-fix:\n- a\nRows:\n- not this"}, "REJECTED", "REJECTED\nmust-fix:\n- a\nRows:\n- not this", []string{"- a"}},
		{"no block, no list", map[string]any{"outcome": "ACCEPTED\nNotes:\n- fine"}, "ACCEPTED", "ACCEPTED\nNotes:\n- fine", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, text, mf := Parse(c.body)
			if v != c.verdict || text != c.text || !reflect.DeepEqual(mf, c.mustFix) {
				t.Fatalf("Parse = %q, %q, %q; want %q, %q, %q", v, text, mf, c.verdict, c.text, c.mustFix)
			}
		})
	}
}

// Row 1 adds STUCK: a reviewer that cannot judge says so on its first line, as it says
// ACCEPTED or REJECTED, and the same trims and the same must-fix reading apply.
func TestPjVbVerdictParseAddsStuck(t *testing.T) {
	cases := []struct {
		name    string
		body    map[string]any
		verdict string
		mustFix []string
	}{
		{"stuck outcome", map[string]any{"outcome": "STUCK\nno test runner on the box"}, "STUCK", nil},
		{"stuck with markdown and case", map[string]any{"outcome": "**Stuck.**"}, "STUCK", nil},
		{"stuck heading with a must-fix block", map[string]any{"outcome": "# STUCK\nMust-fix:\n- a person picks the runner"}, "STUCK", []string{"- a person picks the runner"}},
		{"stuck under an older key", map[string]any{"verdict": "stuck"}, "STUCK", nil},
		{"stuck is a word, not a prefix", map[string]any{"outcome": "STUCKY\n"}, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, _, mf := Parse(c.body)
			if v != c.verdict || !reflect.DeepEqual(mf, c.mustFix) {
				t.Fatalf("Parse = %q, %q; want %q, %q", v, mf, c.verdict, c.mustFix)
			}
		})
	}
}

// Row 6: verdict counts the review rounds over the continues chain. J-2 reworks J-1 after the
// review J-1.1 rejected it, and J-1.2 reviews J-2: the verdict of J-1.2 is round 2.
func TestPjVbVerdictReportsRoundsOverTheContinuesChain(t *testing.T) {
	st := pjVbChain()
	rep, err := Verdict(st, "J-1.2")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Reviewed != "J-2" || rep.RoundsUsed != 2 || rep.Verdict != Rejected {
		t.Fatalf("verdict of J-1.2 = %+v, want J-2 reviewed, REJECTED, rounds_used 2", rep)
	}
	rep, err = Verdict(st, "J-1.1")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Reviewed != "J-1" || rep.RoundsUsed != 1 {
		t.Fatalf("verdict of J-1.1 = %+v, want J-1 reviewed and rounds_used 1", rep)
	}
	if rep.Rounds != nil {
		t.Fatalf("rounds = %v with none given, want null", *rep.Rounds)
	}
}

// Row 7: a result from an older attempt of the review Job is reported stale; once the
// current attempt posts its own result, that one is read and is not stale.
func TestPjVbVerdictFromAnOlderAttemptIsStale(t *testing.T) {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-1.1", "reviewer", "running", 2, "J-1", "")
	pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "ACCEPTED"})
	rep, err := Verdict(st, "J-1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Stale || rep.ReviewAttempt != 1 || rep.Attempt != 2 || rep.Verdict != Accepted {
		t.Fatalf("verdict = %+v, want the attempt-1 ACCEPTED reported stale against attempt 2", rep)
	}
	pjVbRecord(&st, "R2", "result", "J-1.1", 2, map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 1"})
	rep, err = Verdict(st, "J-1.1")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stale || rep.ReviewAttempt != 2 || rep.Verdict != Rejected || len(rep.MustFix) != 1 {
		t.Fatalf("verdict = %+v, want the attempt-2 REJECTED, not stale, one must-fix", rep)
	}
}
