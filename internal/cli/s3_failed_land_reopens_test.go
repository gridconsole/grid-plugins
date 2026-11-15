//go:build !windows

package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A land that fails after the ship answer puts the ship Job back with its person,
// with the exit code and the first stderr lines as its reason; nothing retries on
// its own; the person's second approve runs land again, which lands and closes.
func TestS3FailedLandReopensShipJob(t *testing.T) {
	// The deploy command fails once (exit 7, one stderr line), then merges.
	env, dir, laneSHA, store, stderr := shipLandFixture(t, "test -f once || { touch once && echo boom remote refused >&2 && exit 7 ; } && git merge -q --ff-only lane")

	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve press 1: %d %s", code, stderr)
	}
	jobs, snap := approveJobs(t, store)
	if len(jobs) != 1 {
		t.Fatalf("%d ship person Jobs after a failed land, want the one ship Job", len(jobs))
	}
	j := snap.State.Jobs[jobs[0].ID]
	if j == nil || j.Status != contract.Posted {
		t.Fatalf("ship Job %s is %+v after a failed land, want posted again (waiting for its person); stderr %s; history:\n%s", jobs[0].ID, j, stderr, shipHistory(t, dir))
	}
	for _, want := range []string{"land failed", "exited 7", "boom remote refused"} {
		if !strings.Contains(j.Reason, want) {
			t.Fatalf("ship Job reason %q does not hold %q", j.Reason, want)
		}
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got == laneSHA {
		t.Fatal("main moved on a failed land")
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a failed land, want waiting", rec.State, err)
	}

	// Nothing retries on its own: another pass of the step after the answer runs no land.
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	if n := strings.Count(shipHistory(t, dir), " · land started · "); n != 1 {
		t.Fatalf("%d land started lines before the second approve, want 1:\n%s", n, shipHistory(t, dir))
	}

	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve press 2: %d %s", code, stderr)
	}
	history := shipHistory(t, dir)
	if n := strings.Count(history, " · land started · "); n != 2 {
		t.Fatalf("%d land started lines after the second approve, want 2; stderr %s:\n%s", n, stderr, history)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the second approve, want the delivered %s; stderr %s; history:\n%s", got, laneSHA, stderr, history)
	}
	if !strings.Contains(history, " · land landed · ") {
		t.Fatalf("history has no land landed line:\n%s", history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the second approve, want closed", rec.State, err)
	}
	if jobs, _ := approveJobs(t, store); len(jobs) != 1 {
		t.Fatalf("%d ship person Jobs after the retry, want the one ship Job", len(jobs))
	}
}
