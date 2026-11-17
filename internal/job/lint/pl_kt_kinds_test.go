package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// plKtBrief is the golden brief with role runner, the given kind and the given
// extra frontmatter lines (judge-land.md Q6 lane 1, rows 4 and 5).
func plKtBrief(t *testing.T, kind string, extra ...string) string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(briefWith(t, extra...), "\n") {
		switch {
		case strings.HasPrefix(l, "role:"):
			l = `role: "runner"`
		case strings.HasPrefix(l, "kind:"):
			l = `kind: "` + kind + `"`
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func plKtCheck(t *testing.T, text string) []lint.Finding {
	t.Helper()
	return lint.Check(text, lint.Options{Resolver: testFiles(t)})
}

// Row 4 (V27): a land Job whose branch is the task head's full ref lints clean.
// Lint checks only that branch is not empty (judge-land.md:363-367), so this
// holds on main before this lane too; the test pins it.
func TestPlKtLandFullRefBranchLintsClean(t *testing.T) {
	text := landBrief(t, "main", "refs/grid/T-1/head", "0123456789abcdef0123456789abcdef01234567")
	if fs := plKtCheck(t, text); len(fs) != 0 {
		t.Fatalf("findings: %+v", fs)
	}
}

// Row 5 (V26): a command Job with an argv and a wait Job with every and timeout
// lint clean.
func TestPlKtCommandAndWaitLintClean(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		extra      []string
	}{
		{"command", "command", []string{`command: ["land", "T-1"]`}},
		{"runner", "command", []string{`command: ["land", "test", "T-1", "J-3"]`}},
		{"wait", "wait", []string{`every: "1m"`, `timeout: "30m"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if fs := plKtCheck(t, plKtBrief(t, tc.kind, tc.extra...)); len(fs) != 0 {
				t.Fatalf("findings: %+v", fs)
			}
		})
	}
}

// Row 5 (V26): lint refuses a command Job with no argv, an empty argv or an
// argv with an empty word, with one job.command-argv error; another kind never
// needs the argv.
func TestPlKtCommandWithoutArgvRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"no command", nil},
		{"empty command", []string{`command: []`}},
		{"empty word", []string{`command: ["land", " "]`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []lint.Finding
			for _, f := range plKtCheck(t, plKtBrief(t, "command", tc.extra...)) {
				if f.Severity == "error" {
					got = append(got, f)
				}
			}
			if len(got) != 1 || got[0].Rule != "job.command-argv" {
				t.Fatalf("errors = %+v, want one job.command-argv", got)
			}
		})
	}
	for _, kind := range []string{"build", "wait", "land"} {
		for _, f := range plKtCheck(t, plKtBrief(t, kind)) {
			if f.Rule == "job.command-argv" {
				t.Fatalf("kind %s: %+v", kind, f)
			}
		}
	}
}
