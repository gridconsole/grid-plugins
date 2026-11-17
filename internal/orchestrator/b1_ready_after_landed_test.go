package orchestrator

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// b1LandedFixture posts J-1 and J-2, J-2 waiting on the landing of J-1, and
// returns the fixture with J-1 accepted.
func b1LandedFixture(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{map[string]any{"job": "J-1", "kind": "landed", "why": "Start after the build Job lands"}})
	f.tick(t)
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	return f
}

// Row 2b: an accepted target is not a landed one.
func TestB1NotReadyBeforeLanded(t *testing.T) {
	f := b1LandedFixture(t)
	if got := f.state(t).Status; got != "accepted" {
		t.Fatalf("J-1 is %s, want accepted", got)
	}
	for _, id := range readyIDs(t, f) {
		if id == "J-2" {
			t.Fatalf("J-2 is offered before J-1 landed")
		}
	}
}

// Row 2a: the landed post of J-1, from a leaseless writer, readies J-2.
func TestB1ReadyAfterLanded(t *testing.T) {
	f := b1LandedFixture(t)
	cli := feed.Writer{Author: feed.Author{ID: "cli", Role: "cli"}, Worker: "cli@" + machine}
	f.post(t, feed.Input{Type: "landed", Job: "J-1", Body: map[string]any{"main_sha": "9ab01cd"}}, cli)
	if got := readyIDs(t, f); !reflect.DeepEqual(got, []string{"J-2"}) {
		t.Fatalf("ready after the landed post = %v, want [J-2]", got)
	}
}
