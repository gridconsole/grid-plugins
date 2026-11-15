package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// A whole-file read of one task brain proposal has no range and passes; any
// other unranged read is still a job.read-range error.
func TestR10WholeProposalReadPasses(t *testing.T) {
	f := testFiles(t)
	if err := os.MkdirAll(filepath.Join(f.Task, "brain-proposals", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"tally-money-handling.md", filepath.Join("sub", "x.md")} {
		if err := os.WriteFile(filepath.Join(f.Task, "brain-proposals", rel), []byte("---\nstatus: proposed\ntopic: testing\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := brief(t)
	for _, p := range []string{"brain-proposals/tally-money-handling.md", "task:brain-proposals/tally-money-handling.md"} {
		if fs := lint.Check(strings.Replace(base, "task:research.md#format", p, 1), lint.Options{Resolver: f}); has(fs, "job.read-range") {
			t.Errorf("%s: whole proposal read refused: %+v", p, fs)
		}
	}
	for _, p := range []string{"task:research.md", "brain-proposals/sub/x.md", "brain-proposals/missing.md"} {
		if fs := lint.Check(strings.Replace(base, "task:research.md#format", p, 1), lint.Options{Resolver: f}); !has(fs, "job.read-range") {
			t.Errorf("%s: unranged read passed", p)
		}
	}
}
