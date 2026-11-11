package guard

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestRecallBash(t *testing.T) {
	cases := []struct {
		command string
		deny    bool
	}{
		{`node plan-hash.mjs plan.md`, true},
		{`node plan-hash.mjs 'plan.md'`, true},
		{`node plan-hash.mjs "plan.md"`, true},
		{`node plan-hash.mjs plan.md --print`, false},
		{`node plan-hash.mjs other.md`, false},
		{`echo '· plan ready ·' >> history.md`, true},
		{`echo '· plan ready ·' | tee history.md`, true},
		{`echo '· plan ready ·' | tee hist*.md`, true},
		{`grep 'plan ready' history.md`, false},
		{`grep 'plan ready' history.md | tee notes.md`, false},
		{"grep 'plan ready' history.md | tee \"$OUTPUT\"", true},
		{`sed -i 's/old/plan ready/' history.md`, true},
		{`perl -pi -e 's/old/plan ready/' history.md`, true},
		{`python3 -c "open('history.md', 'a').write('plan ready')"`, true},
		{`echo ordinary >> history.md`, false},
	}
	for _, tt := range cases {
		for _, recall := range []bool{false, true} {
			t.Run(tt.command+map[bool]string{false: "/missing", true: "/recall"}[recall], func(t *testing.T) {
				f := setup(t)
				planBlock(f) // plan-guard warns by default; the recall deny is covered at block
				if recall {
					f.write(filepath.Join(f.task, "plan.md"), "- **Used:** knowledge/note.md\n")
				}
				i := f.input("Bash", tt.command)
				want := ""
				if tt.deny && !recall {
					want = "deny"
				}
				d := EvaluatePlan(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, PlanGuard, i, f.env, d)
			})
		}
	}
}
func TestRecallLineForms(t *testing.T) {
	for _, tt := range []struct {
		text string
		want bool
	}{
		{"Recall: nothing found", true}, {" Used: note", true}, {"- **Used:** note", true}, {"__Recall__: nothing found", true}, {"**Recall:** nothing found", true}, {"recall: Nothing Found", true},
		{"Recall: none", false}, {"    Used: note", false}, {"```\nUsed: note\n```", false}, {"<!-- Used: note -->", false},
		{"Recall:", false}, {"Recall: **", false}, {"prefix Recall: none", false}, {"Used: ___", false},
	} {
		if got := plan.HasRecall(tt.text); got != tt.want {
			t.Errorf("%q: %v", tt.text, got)
		}
	}
}
