package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// b1LandedCheck lints the golden brief with its after edges replaced.
func b1LandedCheck(t *testing.T, after []job.Dependency) []lint.Finding {
	t.Helper()
	base := brief(t)
	j, _, err := job.DecodeBrief(base)
	if err != nil {
		t.Fatal(err)
	}
	j.After = after
	m, err := job.Metadata(j)
	if err != nil {
		t.Fatal(err)
	}
	return lint.Check(m+base[strings.Index(base, "\n## Intent"):], lint.Options{})
}

// A landed after edge is a kind the graph and the fold already know: lint
// accepts it, and an unknown kind is still refused with a fix naming landed.
func TestB1LandedAfterLintsClean(t *testing.T) {
	fs := b1LandedCheck(t, []job.Dependency{{Job: "J-2", Kind: "landed", Why: "Start after the build Job lands"}})
	if len(fs) != 0 {
		t.Fatalf("a landed after edge has findings: %+v", fs)
	}
}

func TestB1LandedAfterUnknownKindRefused(t *testing.T) {
	fs := b1LandedCheck(t, []job.Dependency{{Job: "J-2", Kind: "unknown", Why: "Needs the other result"}})
	var found *lint.Finding
	for i := range fs {
		if fs[i].Rule == "job.after-kind" {
			found = &fs[i]
		}
	}
	if found == nil {
		t.Fatalf("an unknown after kind is not refused: %+v", fs)
	}
	if !strings.Contains(found.Message+" "+found.Fix, "landed") {
		t.Fatalf("job.after-kind does not name landed: %+v", found)
	}
}
