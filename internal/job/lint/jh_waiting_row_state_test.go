package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

func jhStateFindings(t *testing.T, state string) []lint.Finding {
	t.Helper()
	base := brief(t)
	j, _, e := job.DecodeBrief(base)
	if e != nil {
		t.Fatal(e)
	}
	j.Acceptance = []job.Acceptance{{ID: "A1", Text: "Export format", Traces: "item", State: state}}
	m, e := job.Metadata(j)
	if e != nil {
		t.Fatal(e)
	}
	return lint.Check(m+base[strings.Index(base, "\n## Intent"):], lint.Options{})
}

// A row in the waiting state of docs/decisions/D-job-hold.md passes the state
// check; an unknown word is still refused and the hint names waiting.
func TestJhWaitingRowStateLints(t *testing.T) {
	if fs := jhStateFindings(t, "waiting"); has(fs, "job.acceptance-state") {
		t.Fatalf("waiting refused: %+v", fs)
	}
	fs := jhStateFindings(t, "parked")
	if !has(fs, "job.acceptance-state") {
		t.Fatalf("unknown state accepted: %+v", fs)
	}
	for _, f := range fs {
		if f.Rule == "job.acceptance-state" && !strings.Contains(f.Fix, "waiting") {
			t.Fatalf("hint does not name waiting: %+v", f)
		}
	}
}
