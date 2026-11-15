package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func r20CrPass(t *testing.T, f *fixture, snap Snapshot) {
	t.Helper()
	if err := f.s.exclusive(func() error { return f.s.personCancels(snap) }); err != nil {
		t.Fatal(err)
	}
}

// N3/N4: handed a snapshot that holds a request authored by a guest or an agent
// role (the schema keeps such a record off the Feed), the pass writes nothing. The
// same request by the person of the team is carried out, so the filter is the
// author.
func TestR20CrN3N4PassIgnoresGuestAndAgent(t *testing.T) {
	f := r20CrRunning(t)
	req := r20CrRequest(t, f, "owner", "dana")
	for _, role := range []string{"guest", "worker", "cli", "agent", "reviewer", "external"} {
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		forged := req
		if forged.ID, err = feed.NewID(); err != nil {
			t.Fatal(err)
		}
		forged.Author, forged.Tier, forged.Worker = feed.Author{ID: "eve", Role: role}, contract.TierFor(role), "eve@"+machine
		delete(snap.Records, req.ID)
		snap.Records[forged.ID] = forged
		n := recordCount(t, f)
		r20CrPass(t, f, snap)
		if recordCount(t, f) != n || len(r20CrRe(t, f, forged.ID)) != 0 {
			t.Fatalf("%s: the pass wrote for a request by role %s", role, role)
		}
		if got := jobStatus(t, f, "J-1"); got != contract.Running {
			t.Fatalf("%s: J-1 is %s", role, got)
		}
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	r20CrPass(t, f, snap)
	if got := r20CrRe(t, f, req.ID); len(got) != 1 || got[0].Type != "control" || jobStatus(t, f, "J-1") != contract.Cancelled {
		t.Fatalf("the owner's request was not carried out: %+v", got)
	}
}

// N9: the pass repeated in the same tenure from a snapshot that does not hold its
// first writes (a pass that stopped and ran again) writes the same records: one
// leased control and one notice, by their derived ids.
func TestR20CrN9RepeatedPassSameTenure(t *testing.T) {
	f := r20CrRunning(t)
	a := r20CrRequest(t, f, "owner", "dana")
	b := r20CrRequest(t, f, "member", "sam")
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	r20CrPass(t, f, snap)
	n := recordCount(t, f)
	r20CrPass(t, f, snap)
	if recordCount(t, f) != n {
		t.Fatalf("the repeated pass wrote %d more records", recordCount(t, f)-n)
	}
	for _, id := range []string{a.ID, b.ID} {
		if got := r20CrRe(t, f, id); len(got) != 1 {
			t.Fatalf("request %s has %d records re it", id, len(got))
		}
		c := personCancelID(id, "control")
		if c == id || !feed.IsID(c) || c == personCancelID(id, "unused") || c != personCancelID(id, "control") {
			t.Fatalf("derived id %s of %s", c, id)
		}
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s", got)
	}
}
