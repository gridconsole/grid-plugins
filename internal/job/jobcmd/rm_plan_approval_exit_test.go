package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// A render that fails lint exits 1 with its findings and writes no jobs/ folder.
// Jobs render without an approved plan (TestJobsRenderWithoutApprovedPlan), so an
// approval finding has no exit code of its own.
func TestRenderLintErrorExitsOne(t *testing.T) {
	e, out, errs := fixture(t)
	plan := filepath.Join(e.Cwd, "T-01", "plan.md")
	original, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(original), "on_blocked: ask-and-continue", "on_blocked: wait", 1)
	if bad == string(original) {
		t.Fatal("fixture plan has no on_blocked: ask-and-continue to replace")
	}
	if err = os.WriteFile(plan, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	h := jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}
	h.Approve(t)
	r := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if len(r.Findings) == 0 {
		t.Fatal("missing findings")
	}
	for _, finding := range r.Findings {
		if finding.Rule != "job.on-blocked" {
			t.Fatalf("finding rule %q want job.on-blocked: %+v", finding.Rule, finding)
		}
	}
	if _, err = os.Stat(filepath.Join(e.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
		t.Fatal("invalid render created artifacts")
	}
}
