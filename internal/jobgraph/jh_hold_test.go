package jobgraph

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// jhHeld is a Job J-1 held on the given waits, next to J-6 and its rework J-7.
func jhHeld(t *testing.T, status contract.Status, waits ...contract.HeldWait) (*Graph, Snapshot) {
	t.Helper()
	cont := "J-6"
	g := mustGraph(t, []job.Job{{ID: "J-1"}, {ID: "J-6"}, {ID: "J-7", Continues: &cont}})
	return g, Snapshot{Jobs: map[string]contract.Job{
		"J-1": {ID: "J-1", Status: status, WaitsOn: waits},
		"J-6": {ID: "J-6", Status: contract.Running},
		"J-7": {ID: "J-7", Status: contract.Posted},
	}, OpenGates: map[string]bool{}, Landed: map[string]bool{}}
}

// jhReleased says whether J-1 is in Ready, and returns the waits it still shows.
func jhReleased(t *testing.T, g *Graph, s Snapshot) (bool, []Wait) {
	t.Helper()
	r, err := g.Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	var waits []Wait
	for _, w := range r.Waiting {
		if w.Job == "J-1" {
			waits = append(waits, w)
		}
	}
	for _, id := range r.Ready {
		if id == "J-1" {
			return true, waits
		}
	}
	return false, waits
}

// A Job held on another Job is released when that Job is accepted, and not before:
// not while it runs, and not when it is submitted with its verify passing (the
// decision's wait is on the change being accepted).
func TestHoldReleasedWhenJobAccepted(t *testing.T) {
	g, s := jhHeld(t, contract.Held, contract.HeldWait{Row: "A3", Kind: contract.WaitJob, Ref: "J-6"})
	for _, status := range []contract.Status{contract.Running, contract.Rejected, contract.Cancelled} {
		s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: status}
		if released, waits := jhReleased(t, g, s); released || len(waits) != 1 || waits[0].Target != "J-6" || waits[0].Kind != contract.WaitJob {
			t.Fatalf("J-6 %s: released %v waits %+v, want held on J-6", status, released, waits)
		}
	}
	s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: contract.Submitted, Verified: []byte(`{"pass":true}`)}
	if released, _ := jhReleased(t, g, s); released {
		t.Fatal("released on a submitted Job that is not accepted")
	}
	s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: contract.Accepted}
	if released, waits := jhReleased(t, g, s); !released || len(waits) != 0 {
		t.Fatalf("J-6 accepted: released %v waits %+v, want released with no wait", released, waits)
	}
}

// The wait follows a rework: J-6 rejected and J-7, which continues it, accepted
// meets a wait on J-6; a rework that is not accepted does not.
func TestHoldFollowsReworkOfTheJobItWaitsOn(t *testing.T) {
	g, s := jhHeld(t, contract.Held, contract.HeldWait{Row: "A3", Kind: contract.WaitJob, Ref: "J-6"})
	s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: contract.Rejected}
	if released, _ := jhReleased(t, g, s); released {
		t.Fatal("released on a rejected Job whose rework is not accepted")
	}
	s.Jobs["J-7"] = contract.Job{ID: "J-7", Status: contract.Accepted}
	if released, _ := jhReleased(t, g, s); !released {
		t.Fatal("not released when the rework of J-6 is accepted")
	}
}

// A Job held on a landing is released by a counted landed post for the Job it
// names, not by that Job's accept.
func TestHoldReleasedWhenLanded(t *testing.T) {
	g, s := jhHeld(t, contract.Held, contract.HeldWait{Row: "A2", Kind: contract.WaitLanded, Ref: "J-6"})
	s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: contract.Accepted}
	if released, waits := jhReleased(t, g, s); released || len(waits) != 1 || waits[0].Reason != "waiting for J-6 to land (A2)" {
		t.Fatalf("accepted, not landed: released %v waits %+v", released, waits)
	}
	s.Landed = map[string]bool{"J-2": true}
	if released, _ := jhReleased(t, g, s); released {
		t.Fatal("released by another Job landing")
	}
	s.Landed = map[string]bool{"J-6": true}
	if released, waits := jhReleased(t, g, s); !released || len(waits) != 0 {
		t.Fatalf("J-6 landed: released %v waits %+v, want released", released, waits)
	}
}

// A Job held on a question is released when the question is answered: its gate,
// as the after edge gate:Q-n names it, reads open.
func TestHoldReleasedWhenQuestionAnswered(t *testing.T) {
	g, s := jhHeld(t, contract.Held, contract.HeldWait{Row: "A4", Kind: contract.WaitQuestion, Ref: "Q-2"})
	s.OpenGates = map[string]bool{"gate:Q-2": false, "gate:Q-3": true}
	if released, waits := jhReleased(t, g, s); released || len(waits) != 1 || waits[0].Target != "Q-2" {
		t.Fatalf("question open: released %v waits %+v", released, waits)
	}
	s.OpenGates["gate:Q-2"] = true
	if released, waits := jhReleased(t, g, s); !released || len(waits) != 0 {
		t.Fatalf("question answered: released %v waits %+v, want released", released, waits)
	}
}

// A Blocked Job that waits on a permission is released when a counted approval
// answers its blocked post. Until then it shows no wait and the tick posts nothing
// over it, as before; a Blocked Job that names no permission wait is never released.
func TestHoldReleasedWhenPermissionGranted(t *testing.T) {
	wait := contract.HeldWait{Row: "A5", Kind: contract.WaitPermission, Ref: "01HBLOCKEDPOST"}
	g, s := jhHeld(t, contract.Blocked, wait)
	if released, waits := jhReleased(t, g, s); released || len(waits) != 0 {
		t.Fatalf("no approval: released %v waits %+v, want blocked and silent", released, waits)
	}
	s.Approved = map[string]bool{"01HOTHERPOST": true}
	if released, _ := jhReleased(t, g, s); released {
		t.Fatal("released by an approval of another blocked post")
	}
	s.Approved = map[string]bool{wait.Ref: true}
	if released, _ := jhReleased(t, g, s); !released {
		t.Fatal("not released when its blocked post is approved")
	}
	for name, waits := range map[string][]contract.HeldWait{
		"no wait named":          nil,
		"only a Job it waits on": {{Row: "A1", Kind: contract.WaitJob, Ref: "J-6"}},
	} {
		g, s := jhHeld(t, contract.Blocked, waits...)
		s.Jobs["J-6"] = contract.Job{ID: "J-6", Status: contract.Accepted}
		s.Approved = map[string]bool{wait.Ref: true}
		if released, _ := jhReleased(t, g, s); released {
			t.Fatalf("%s: a Blocked Job with no permission wait was released", name)
		}
	}
}

// A hold waits for every wait it names, shows only the unmet ones, and reports
// each with its met state for the tree. No count or time is read: the same hold
// is released the moment its last wait lands, however long it took.
func TestHoldNeedsEveryWaitAndReportsMet(t *testing.T) {
	g, s := jhHeld(t, contract.Held,
		contract.HeldWait{Row: "A2", Kind: contract.WaitLanded, Ref: "J-6"},
		contract.HeldWait{Row: "A4", Kind: contract.WaitQuestion, Ref: "Q-2"},
	)
	s.Landed = map[string]bool{"J-6": true}
	r, err := g.Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(r.Ready, "J-1") {
		t.Fatalf("Ready = %v, want J-1 held while the question is open", r.Ready)
	}
	yes, no := true, false
	want := []contract.HeldWait{{Row: "A2", Kind: contract.WaitLanded, Ref: "J-6", Met: &yes}, {Row: "A4", Kind: contract.WaitQuestion, Ref: "Q-2", Met: &no}}
	if !reflect.DeepEqual(r.Holds["J-1"], want) {
		t.Fatalf("Holds = %+v, want %+v", r.Holds["J-1"], want)
	}
	s.OpenGates = map[string]bool{"gate:Q-2": true}
	if released, _ := jhReleased(t, g, s); !released {
		t.Fatal("not released when the last wait landed")
	}
}
