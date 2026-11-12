package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// A repo named "." (or a spelling that cleans to it) is the project folder, so lint takes
// it though the project lists only named repos.
func TestCiDotRepoPassesLint(t *testing.T) {
	for _, name := range []string{".", "./"} {
		t.Run(name, func(t *testing.T) {
			fs := lint.CheckJSON(vcRepoRecord(t, name), lint.Options{Repos: []string{"web"}})
			if got := append(vcRepoRule(fs, "job.repo"), vcRepoRule(fs, "job.record")...); len(got) != 0 {
				t.Fatalf("repo %q has findings: %+v", name, got)
			}
		})
	}
}

// A name that is neither "." nor listed is still one job.repo error, with the name as written.
func TestCiDotUnknownRepoStillRefused(t *testing.T) {
	fs := lint.CheckJSON(vcRepoRecord(t, "nope"), lint.Options{Repos: []string{"web"}})
	got := vcRepoRule(fs, "job.repo")
	if len(got) != 1 {
		t.Fatalf("want one job.repo finding, got %+v", fs)
	}
	if m := got[0].Message; !strings.Contains(m, "nope") || !strings.Contains(m, "web") {
		t.Fatalf("job.repo message: %q", m)
	}
}
