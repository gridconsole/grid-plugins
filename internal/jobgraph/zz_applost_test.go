package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func zzReport(t *testing.T, name string, ready bool, waits []Wait, wantWait bool) {
	t.Helper()
	if ready == wantWait {
		t.Errorf("%s: ready=%v waits=%+v (want wait=%v)", name, ready, waits, wantWait)
	} else {
		t.Logf("%s: ok ready=%v waits=%+v", name, ready, waits)
	}
}

// The app Judge reviewLost card (gridconsole-app judge.go:865-869): a person Job numbered under B, discovered from B.
func zzAppLostCard(id, b string) job.Job {
	return job.Job{ID: id, Role: job.RolePerson, Title: "Decide how " + b + " continues without its review", DiscoveredFrom: m1Ptr(b)}
}

func TestZzAppLostCardAccepted(t *testing.T) {
	r, w := m1Deliver(t, m1Plan(zzAppLostCard("J-2.2", "J-2")), m1Status("J-2", "accepted", "J-2.1", "cancelled", "J-2.2", "accepted"))
	zzReport(t, "B accepted, review cancelled, app lost card accepted", r, w, false)
	items := m1Plan(zzAppLostCard("J-2.2", "J-2"), m1Rework("J-4", "J-2", "J-2.2"), m1Review("J-2.3", "J-4"))
	r, w = m1Deliver(t, items, m1Status("J-2", "rejected", "J-2.1", "cancelled", "J-2.2", "accepted", "J-4", "accepted", "J-2.3", "accepted"))
	zzReport(t, "B reworked after app lost card accepted, rework and review accepted", r, w, false)
}

// A rework's card is numbered under the rework (childID(b)) while its lost review is numbered under the first build.
func TestZzAppLostCardOfReworkAccepted(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), zzAppLostCard("J-4.1", "J-4"))
	r, w := m1Deliver(t, items, m1Status("J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "cancelled", "J-4.1", "accepted"))
	zzReport(t, "rework accepted, its review cancelled, app lost card accepted", r, w, false)
}

// Only an accepted card settles the lost review: an open, cancelled or rejected one leaves it holding.
func TestZzAppLostCardNotAcceptedHolds(t *testing.T) {
	items := m1Plan(zzAppLostCard("J-2.2", "J-2"))
	for _, card := range []string{"posted", "running", "cancelled", "rejected"} {
		status := m1Status("J-2", "accepted", "J-2.1", "cancelled", "J-2.2", card)
		jobs := map[string]contract.Job{}
		for id, st := range status {
			jobs[id] = contract.Job{ID: id, Status: st}
		}
		if mustGraph(t, items).lostDecided(Snapshot{Jobs: jobs}, "J-2.1") {
			t.Errorf("card %s: the cancelled review J-2.1 is settled", card)
		}
		r, w := m1Deliver(t, items, status)
		zzReport(t, "B accepted, review cancelled, app lost card "+card, r, w, true)
	}
	other := m1Plan(zzAppLostCard("J-2.2", "J-5"))
	jobs := map[string]contract.Job{"J-2.1": {ID: "J-2.1", Status: contract.Cancelled}, "J-2.2": {ID: "J-2.2", Status: contract.Accepted}}
	if mustGraph(t, other).lostDecided(Snapshot{Jobs: jobs}, "J-2.1") {
		t.Error("an accepted card for another Job settles the cancelled review J-2.1")
	}
}
