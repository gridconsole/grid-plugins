package orchestrator

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// iuRkJobRecords returns the entries of the Feed that are job records with the
// body id id, whatever their verdict.
func iuRkJobRecords(t *testing.T, f *fixture, id string) []feed.Entry {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Entry
	for _, e := range r.Records {
		if e.Type == "job" && stringValue(e.Body["id"]) == id {
			out = append(out, e)
		}
	}
	return out
}

// iuRkVerdict is the verdict the Feed reader gives the record id, "" when the
// Feed has no such record.
func iuRkVerdict(t *testing.T, f *fixture, id string) string {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.ID == id {
			return e.Verdict
		}
	}
	return ""
}

// A job record the fold holds (its after target is on no record) is in neither
// State nor Items, but its id is taken: the re-plan Job gets the next one.
func TestIuRkReplanSkipsHeldID(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	// J-3 is posted after a ULID that no record carries, so the fold holds it.
	missing, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	held := f.post(t, feed.Input{Type: "job", Job: "J-3", Body: jobBody(t, "J-3"), After: []string{missing}}, f.s.Writer)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.Jobs["J-3"] != nil {
		t.Fatal("the held J-3 is in State.Jobs")
	}
	if v := iuRkVerdict(t, f, held.ID); v != "held" {
		t.Fatalf("verdict of the J-3 record = %q, want held", v)
	}
	for _, it := range snap.Items {
		if it.ID == "J-3" {
			t.Fatal("the held J-3 is in Items")
		}
	}
	if _, in := snap.Records[held.ID]; in {
		t.Fatal("the held J-3 record is in Records")
	}
	if !reflect.DeepEqual(snap.JobIDs, []string{"J-1", "J-2", "J-3"}) {
		t.Fatalf("JobIDs = %v, want J-1 J-2 J-3", snap.JobIDs)
	}

	f.tick(t)

	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-4"] == nil || replans["J-4"]["discovered_from"] != "J-1" {
		t.Fatalf("want one re-plan Job J-4 from J-1, got %v (counted Jobs %v)", replans, all)
	}
	if records := iuRkJobRecords(t, f, "J-3"); len(records) != 1 || records[0].ID != held.ID {
		t.Fatalf("J-3 records = %d, want only the held one %s", len(records), held.ID)
	}
	if snap, err := Load(f.s.Feed); err != nil {
		t.Fatal(err)
	} else if snap.State.Jobs["J-3"] != nil || snap.State.Jobs["J-4"] == nil {
		t.Fatalf("after the tick State holds J-3 = %v, J-4 = %v", snap.State.Jobs["J-3"], snap.State.Jobs["J-4"])
	}
}
