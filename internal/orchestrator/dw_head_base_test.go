package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func dwSnap(jobs map[string]*contract.Job) Snapshot {
	for id, j := range jobs {
		j.ID = id
	}
	return Snapshot{State: contract.State{Jobs: jobs}}
}

func dwRef(s string) *string { return &s }

// The thv T-16 chain: J-21 and its rework J-29 are cancelled, J-30 continues J-29 and
// is accepted on attempt 1. A base of head:J-21 is the J-30 a1 branch.
func TestDwHeadBaseReplaysT16Chain(t *testing.T) {
	s := dwSnap(map[string]*contract.Job{
		"J-21": {Status: contract.Cancelled, Attempt: 1},
		"J-29": {Status: contract.Cancelled, Attempt: 1, Continues: dwRef("J-21")},
		"J-30": {Status: contract.Accepted, Attempt: 1, Continues: dwRef("J-29")},
	})
	got, err := ResolveBase(s, "01TASK", "head:J-21")
	if err != nil {
		t.Fatal(err)
	}
	if want := "grid-01TASK-J-30-a1"; got != want {
		t.Fatalf("base = %q, want %q", got, want)
	}
	if got, _ := ResolveBase(s, "01TASK", "main"); got != "main" {
		t.Fatalf("a plain base changed to %q", got)
	}
	if got, _ := ResolveBase(s, "01TASK", ""); got != "" {
		t.Fatalf("an empty base changed to %q", got)
	}
}

func TestDwHeadBaseNoAcceptedHeadNamesTheJob(t *testing.T) {
	s := dwSnap(map[string]*contract.Job{
		"J-3": {Status: contract.Running, Attempt: 1},
		"J-4": {Status: contract.Rejected, Attempt: 1},
	})
	for _, id := range []string{"J-3", "J-4", "J-9"} {
		got, err := ResolveBase(s, "01TASK", "head:"+id)
		if err == nil || got != "" || !strings.Contains(err.Error(), id) {
			t.Fatalf("head:%s = %q, %v; want an error naming %s and no ref", id, got, err, id)
		}
	}
}

// A Job whose base has no accepted head is not assigned and says why; once the
// head is accepted it starts from that head's branch.
func TestDwHeadBaseTickLeavesJobUnspawned(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	body := jobBody(t, "J-2")
	body["base"] = "head:J-1"
	f.post(t, feed.Input{Type: "job", Body: body, Job: "J-2"}, f.s.Writer)
	out := f.tick(t)
	for _, sp := range out.Spawns {
		if sp.Job == "J-2" {
			t.Fatalf("J-2 spawned with base %q before J-1 is accepted", sp.Base)
		}
	}
	st, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs["J-2"]
	if j.Status == contract.Assigned || j.Status == contract.Running || !strings.Contains(j.Reason, "J-1") {
		t.Fatalf("J-2 is %s with reason %q; want it unassigned and the reason to name J-1", j.Status, j.Reason)
	}
}
