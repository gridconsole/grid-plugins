package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// lwClose closes Job id with status by a control.
func lwClose(t *testing.T, f *fixture, id string, status contract.Status) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.control(snap.State.Jobs[id], status, string(status), ""); err != nil {
		t.Fatal(err)
	}
}

// lwLandWait posts J-2, and J-3 and J-4 that wait for it to land.
func lwLandWait(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-2", nil)
	for _, id := range []string{"J-3", "J-4"} {
		f.job(t, id, []any{map[string]any{"job": "J-2", "kind": "landed", "why": "Check the live page after the land"}})
	}
	f.tick(t)
	if jobs := jhOrchestratorJobs(t, f); len(jobs) != 0 {
		t.Fatalf("J-2 is open, yet the tick posted %v", jobs)
	}
	return f
}

// GridV2 T-12, 19 Nov: J-64 waited, Posted, for the accepted J-63 to land; no Job
// posted the landed record, so J-64, J-65 behind it and the deliver Job waited for
// ever. The tick posts one orchestrator re-plan for J-63, naming every Job that
// waits for it to land, and when the re-plan ends without a fix no person is asked.
func TestLwReplanPostedJobsWaitingForAnAcceptedJobToLand(t *testing.T) {
	f := lwLandWait(t)
	lwClose(t, f, "J-2", contract.Accepted)
	f.tick(t)
	jobs := jhOrchestratorJobs(t, f)
	want := "Re-plan after J-2 was accepted: Jobs wait for it to land"
	if len(jobs) != 1 || jobs["J-5"]["title"] != want || jobs["J-5"]["discovered_from"] != nil {
		t.Fatalf("orchestrator Jobs %v, want J-5 titled %q with no discovered_from", jobs, want)
	}
	if text := jhA1(jobs["J-5"]); !strings.Contains(text, "Jobs wait for it to land (J-3, J-4)") || !strings.Contains(text, "grid feed post --type landed --job J-2 --lease task") {
		t.Fatalf("J-5 reads %q, want the waits and the landed record named", text)
	}
	lwClose(t, f, "J-5", contract.Cancelled)
	f.tick(t)
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Items) != 4 {
		t.Fatalf("Jobs %v after the re-plan was cancelled, want J-2 to J-5 only: the title is the record and no person is asked", ids(snap))
	}
}

// A landed record for J-2, posted as an agent does it (a worker's task write, as
// the re-plan names it), meets the waits: no re-plan, and the waiters run.
func TestLwNoReplanOnceTheJobLanded(t *testing.T) {
	f := lwLandWait(t)
	lwClose(t, f, "J-2", contract.Accepted)
	agent := f.s.Writer
	agent.Author.Role = "worker"
	// The agent's own GRID_ATTEMPT rides on the post (feedDefaults); a landed post has no attempt rule.
	f.post(t, feed.Input{Type: "landed", Job: "J-2", Attempt: 3, Body: map[string]any{"main_sha": "3f2c9e1"}}, agent)
	if out := f.tick(t); out.Assigned != 2 {
		t.Fatalf("tick assigned %d, want J-3 and J-4", out.Assigned)
	}
	if jobs := jhOrchestratorJobs(t, f); len(jobs) != 0 {
		t.Fatalf("J-2 landed, yet the tick posted %v", jobs)
	}
}
