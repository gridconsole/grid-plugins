package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// invalidJobFixture is the fixture with a plan whose Job 1 fails lint, as in
// TestRenderRefusesInvalidAndChangedJobs.
func invalidJobFixture(t *testing.T) (Env, func(args ...string) (int, string, string)) {
	t.Helper()
	e, out, errs := fixture(t)
	plan := filepath.Join(e.Cwd, "T-01", "plan.md")
	original, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(original), "on_blocked: ask-and-continue", "on_blocked: wait", 1)
	if err = os.WriteFile(plan, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	(jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}).Approve(t)
	return e, func(args ...string) (int, string, string) {
		out.Reset()
		errs.Reset()
		code := Run(args, e)
		return code, out.String(), errs.String()
	}
}

// The job-lint level decides what a lint error does at render: block (the default)
// refuses, warn says the finding and renders, off does not lint.
func TestRenderFollowsTheJobLintLevel(t *testing.T) {
	for _, tt := range []struct {
		level     string // "" stores nothing: the catalog default
		code      int
		finding   bool
		artifacts bool
	}{
		{"", 1, false, false},
		{"block", 1, false, false},
		{"warn", 0, true, true},
		{"off", 0, false, true},
	} {
		t.Run("level="+tt.level, func(t *testing.T) {
			e, run := invalidJobFixture(t)
			if tt.level != "" {
				body := `{"hooks":{"job-lint":"` + tt.level + `"}}`
				if err := os.WriteFile(filepath.Join(e.GridHome, "settings.json"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			code, out, errs := run("render", "T-01", "1", "--json")
			if code != tt.code {
				t.Fatalf("render at job-lint %q: exit %d, want %d; stdout %s stderr %s", tt.level, code, tt.code, out, errs)
			}
			if got := strings.Contains(errs, "job-lint is warn"); got != tt.finding {
				t.Fatalf("finding on stderr = %v, want %v: %s", got, tt.finding, errs)
			}
			_, err := os.Stat(filepath.Join(e.Cwd, "T-01", "jobs"))
			if got := err == nil; got != tt.artifacts {
				t.Fatalf("jobs folder exists = %v, want %v", got, tt.artifacts)
			}
		})
	}
}

// Settings the command cannot read keep the refusal, and say why.
func TestRenderKeepsTheRefusalWhenHookSettingsAreUnreadable(t *testing.T) {
	e, run := invalidJobFixture(t)
	if err := os.WriteFile(filepath.Join(e.GridHome, "settings.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs := run("render", "T-01", "1", "--json")
	if code != 1 || !strings.Contains(errs, "hook settings cannot be read") {
		t.Fatalf("exit %d, want 1 with the settings error; stdout %s stderr %s", code, out, errs)
	}
}
