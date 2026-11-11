package migrate

import (
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// joPjHeldFeed is a Feed with the counted Job J-4 and a `job` record of body id
// J-5 that waits on a record this Feed does not have yet, so the fold holds it
// and Load does not see it. It returns the id that record waits for.
func joPjHeldFeed(t *testing.T) (feed.Store, feed.Writer, string) {
	t.Helper()
	dir, w := receiptFolder(t, "")
	store := feed.Store{TaskDir: dir, TaskID: receiptTaskID}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: "J-4", Body: PlanJobBody("J-4", "dana")}); err != nil {
		t.Fatal(err)
	}
	u, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	planted := PlanJobBody("J-5", "sam")
	planted["title"] = "Planted"
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-5", After: []string{u}, Body: planted}, personWriter()); err != nil {
		t.Fatalf("the held seed was refused: %v", err)
	}
	read, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	held := false
	for _, e := range read.Records {
		if e.Body["id"] == "J-5" && e.Verdict == "held" {
			held = true
		}
	}
	if !held {
		t.Fatalf("the seed is not held: %+v", read.Records)
	}
	return store, w, u
}

// joPjIDCount counts the `job` entries of the Feed, with any verdict, whose body
// id is id.
func joPjIDCount(t *testing.T, store feed.Store, id string) int {
	t.Helper()
	read, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range read.Records {
		if e.Body["id"] == id {
			n++
		}
	}
	return n
}

// A `job` record that waits on a record not yet on the Feed is held, kept out of
// the snapshot, and still owns its id: the plan Job takes the next number, so
// the record that resolves later does not push it out of the fold.
func TestJoPjHeldIDSkipped(t *testing.T) {
	store, w, u := joPjHeldFeed(t)
	id, err := PostPlanJob(store, personWriter(), "dana")
	if err != nil || id != "J-6" {
		t.Fatalf("want J-6 above the held J-5, got %q %v", id, err)
	}
	// The record the held one waits for arrives.
	if _, err := orchestrator.Write(store, w, feed.Input{ID: u, Type: "job", Job: "J-9", Body: PlanJobBody("J-9", "sam")}); err != nil {
		t.Fatal(err)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := st.Jobs["J-5"]; j == nil || j.Title != "Planted" {
		t.Fatalf("J-5 after the held record resolved: %+v", j)
	}
	if j := st.Jobs["J-6"]; j == nil || j.Title != "Approve the plan" {
		t.Fatalf("J-6 after the held record resolved: %+v", j)
	}
}

// Two writers press at once on a Feed with a held J-5: they take two ids, neither
// is J-5, and no id is carried twice.
func TestJoPjTwoWriters(t *testing.T) {
	store, w, _ := joPjHeldFeed(t)
	writers := []feed.Writer{w, personWriter()}
	ids := make([]string, len(writers))
	errs := make([]error, len(writers))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ids[i], errs[i] = PostPlanJob(store, writers[i], "dana")
		}()
	}
	close(start)
	wg.Wait()
	for i := range writers {
		if errs[i] != nil {
			t.Fatalf("writer %d: %q %v", i, ids[i], errs[i])
		}
	}
	if ids[0] == ids[1] || ids[0] == "J-5" || ids[1] == "J-5" {
		t.Fatalf("the two writers took %q and %q; want two ids, neither J-5", ids[0], ids[1])
	}
	if n := joPjIDCount(t, store, "J-5"); n != 1 {
		t.Fatalf("J-5 is carried by %d job records, want 1", n)
	}
	for _, id := range ids {
		if n := joPjIDCount(t, store, id); n != 1 {
			t.Fatalf("%s is carried by %d job records, want 1", id, n)
		}
	}
}
