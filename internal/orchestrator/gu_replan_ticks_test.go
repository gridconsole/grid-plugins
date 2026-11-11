package orchestrator

import (
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guCtTicks runs n Ticks at once, each on its own copy of the fixture Store,
// and returns the error of each. Every goroutine reports that it is waiting
// and then blocks on the start channel; the channel is closed once all n have
// reported, so the Ticks are released together. Nothing sleeps and nothing
// times out: the pass lock (Store.locked) is what the Ticks meet.
func guCtTicks(f *fixture, n int) []error {
	start := make(chan struct{})
	var waiting, done sync.WaitGroup
	errs := make([]error, n)
	waiting.Add(n)
	for i := range n {
		s := f.s
		done.Go(func() {
			waiting.Done()
			<-start
			_, errs[i] = s.Tick()
		})
	}
	waiting.Wait()
	close(start)
	done.Wait()
	return errs
}

// guCtNoErrors fails the test for every Tick that did not return nil.
func guCtNoErrors(t *testing.T, errs []error) {
	t.Helper()
	for i, err := range errs {
		if err != nil {
			t.Errorf("Tick %d of %d returned %v, want nil", i+1, len(errs), err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

// guCtJob reads one Job from the folded state; it must exist and be assigned.
func guCtJob(t *testing.T, f *fixture, id string) *contract.Job {
	t.Helper()
	st, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[id]
	if j == nil || j.Assign == nil {
		t.Fatalf("%s has no assignment: %+v", id, j)
	}
	return j
}

// guCtAck is f.ack for the Job id: the worker writes with a Lease that is the
// Job id, as the worker of that Job does.
func guCtAck(t *testing.T, f *fixture, id string) {
	t.Helper()
	j := guCtJob(t, f, id)
	w := f.worker
	w.Lease, w.Fence, w.Gen = id, j.Assign.Fence, j.Generation
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "ack", Job: id, Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
}

// guCtExit is f.exit for the Job id, written after guCtAck with the fence and
// generation the Job holds then.
func guCtExit(t *testing.T, f *fixture, id, reason string) {
	t.Helper()
	j := guCtJob(t, f, id)
	w := f.worker
	w.Lease, w.Fence, w.Gen = id, j.Assign.Fence, j.Generation
	body := map[string]any{}
	if reason != "" {
		body["reason"] = reason
	}
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "worker-exit", Job: id, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// guCtDiscovered returns, sorted, the Job each re-plan Job on the Feed was
// discovered from; a Job id that two re-plan Jobs share shows up twice.
func guCtDiscovered(replans map[string]map[string]any) []string {
	var from []string
	for _, body := range replans {
		s, _ := body["discovered_from"].(string)
		from = append(from, s)
	}
	sort.Strings(from)
	return from
}

func TestGuCtTwoTicksOneReplan(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	guCtNoErrors(t, guCtTicks(f, 2))
	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-3"] == nil || replans["J-3"]["discovered_from"] != "J-1" {
		t.Fatalf("want exactly one re-plan Job J-3 from J-1, got %v (Jobs %v)", replans, all)
	}
}

func TestGuCtEightTicksOneReplan(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	guCtNoErrors(t, guCtTicks(f, 8))
	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-3"] == nil || replans["J-3"]["discovered_from"] != "J-1" {
		t.Fatalf("want exactly one re-plan Job J-3 from J-1, got %v (Jobs %v)", replans, all)
	}
	if want := []string{"J-1", "J-2", "J-3"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
}

// J-3 waits on J-1 and J-4 on J-2. One tick assigns J-1 and J-2, both workers
// ack and exit cancelled, and four Ticks at once re-plan each dead Job once,
// under two different ids.
func TestGuCtTwoDeadJobs(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", nil)
	f.job(t, "J-3", []any{"J-1"})
	f.job(t, "J-4", []any{"J-2"})
	if out := f.tick(t); out.Assigned != 2 {
		t.Fatalf("the first tick assigned %d Jobs, want J-1 and J-2: %+v", out.Assigned, out)
	}
	for _, id := range []string{"J-1", "J-2"} {
		guCtAck(t, f, id)
		guCtExit(t, f, id, "cancelled")
	}
	guCtNoErrors(t, guCtTicks(f, 4))
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4", "J-5", "J-6"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if want := []string{"J-1", "J-2"}; len(replans) != 2 || !reflect.DeepEqual(guCtDiscovered(replans), want) {
		t.Fatalf("want two re-plan Jobs discovered from %v, got %v from %v", want, replans, guCtDiscovered(replans))
	}
}
