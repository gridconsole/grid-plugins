package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// The curator Job deliver posts for a waiting proposal passes the lint grid job
// render uses and renders. In round 10 render refused it: its read pointer
// brain-proposals/<name>.md had no range (job.read-range), and the words of the
// task's Outcome, its intent.item, were absent from its acceptance
// (job.acceptance-coverage).
func TestR10DeliverCuratorJobPassesRenderLint(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "tally-money-handling")
	outcome := "Tally money handling rounds amounts to whole cents and the rounding rule is written down"
	if code := h.deliverRun(dir, "--outcome", outcome, "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	jobs, snap := curatorJobs(t, store)
	if len(jobs) != 1 {
		t.Fatalf("want one curator Job, got %d (stdout %q)", len(jobs), h.out.String())
	}
	jid := jobs[0].ID
	body := snap.Bodies[jid]
	if body == nil {
		t.Fatalf("no body for %s on the Feed", jid)
	}
	findings, err := postFindings(h.env, dir, body)
	if err != nil {
		t.Fatalf("the lint options could not be built: %v", err)
	}
	for _, f := range findings {
		if f.Rule == "job.read-range" || f.Rule == "job.acceptance-coverage" {
			t.Errorf("curator Job finding %s: %s", f.Rule, f.Message)
		}
	}
	if lint.HasErrors(findings) {
		t.Fatalf("render would refuse the curator Job: %+v", findings)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(dir, "job", "render", dir, jid, "--json"); code != 0 {
		t.Fatalf("job render exit %d: %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs", jid, "brief.md")); err != nil {
		t.Fatalf("job render saved no brief: %v (render printed %s)", err, h.out)
	}
}
