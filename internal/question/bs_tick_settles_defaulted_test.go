package question

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// bsTsEvents counts the history events of one kind.
func bsTsEvents(r Record, kind string) int {
	n := 0
	for _, e := range r.History {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// TestBsTsSecondTickClosesDefaultedJob: when the Close after a default fails, the
// next Tick settles the person Job of the record it skips, and returns nothing new.
func TestBsTsSecondTickClosesDefaultedJob(t *testing.T) {
	s, now := setup(t)
	calls := &jobCalls{}
	s.Jobs = calls.port("J-7", nil)
	boom := errors.New("close failed")
	inner := s.Jobs.Close
	attempts := 0
	s.Jobs.Close = func(f feed.Store, lease, person feed.Writer, job, result string) error {
		attempts++
		if attempts == 1 {
			return boom
		}
		return inner(f, lease, person, job, result)
	}
	if _, err := s.Ask(input()); err != nil {
		t.Fatal(err)
	}
	*now = start.Add(24 * time.Hour)
	changed, err := s.Tick()
	if !errors.Is(err, boom) {
		t.Fatalf("first Tick error %v, want the Close error", err)
	}
	if len(changed) != 1 || bsTsEvents(changed[0], "defaulted") != 1 {
		t.Fatalf("first Tick changed %+v", changed)
	}
	if len(calls.closes) != 0 {
		t.Fatalf("closes after the failed Close: %v", calls.closes)
	}
	if rec := list(t, s)[0]; bsTsEvents(rec, "defaulted") != 1 {
		t.Fatalf("history after first Tick %+v", rec.History)
	}

	changed, err = s.Tick()
	if err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("second Tick changed %+v, want none", changed)
	}
	if want := [][2]string{{"J-7", "CSV"}}; !slices.Equal(calls.closes, want) {
		t.Fatalf("closes %v, want %v", calls.closes, want)
	}
	if rec := list(t, s)[0]; bsTsEvents(rec, "defaulted") != 1 {
		t.Fatalf("history after second Tick %+v", rec.History)
	}
}
