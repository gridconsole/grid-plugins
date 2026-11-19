package orchestrator

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// ldClose accepts Job id of task i as its owner does, so the task has no open Job.
func ldClose(t *testing.T, h *streamHome, i int, id string) {
	t.Helper()
	st := h.store(i)
	ts := lease.TaskStore{Feed: st}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = g.Fence
	if _, err := st.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"target": id, "status": string(contract.Accepted), "reason": "done"}
	if _, err := st.Post(feed.Input{Type: "control", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
}

// A task whose Jobs are all closed is left alone, as it was; one whose land is still to
// run (LandPending) keeps its pass, so the step after the answer runs again on every pass
// until the land has run.
func TestLandPendingKeepsATaskWithNoOpenJob(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pending func(string) bool
		want    int
	}{
		{"no hook", nil, 0},
		{"nothing pending", func(string) bool { return false }, 0},
		{"a land pending", func(string) bool { return true }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newStreamHome(t, 1)
			h.post(0, "J-1")
			ldClose(t, h, 0, "J-1")
			ticked := 0
			s := h.stream(4242, nil)
			s.LandPending = tc.pending
			s.Ticked = func(string, feed.Writer) { ticked++ }
			h.pass(s)
			h.pass(s)
			if ticked != tc.want {
				t.Fatalf("Ticked ran %d times over two passes, want %d", ticked, tc.want)
			}
		})
	}
}

// A task dropped as quiet is looked at again once its land is pending, though the Feed
// did not change: arming a land writes only the task's history.
func TestLandPendingReachesATaskThePassHadDroppedAsQuiet(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	ldClose(t, h, 0, "J-1")
	pending := false
	ticked := 0
	s := h.stream(4242, nil)
	s.LandPending = func(string) bool { return pending }
	s.Ticked = func(string, feed.Writer) { ticked++ }
	h.pass(s)
	h.pass(s)
	if ticked != 0 {
		t.Fatalf("Ticked ran %d times for a task with nothing pending", ticked)
	}
	pending = true
	h.pass(s)
	if ticked != 1 {
		t.Fatalf("Ticked ran %d times after the land was armed, want 1", ticked)
	}
}

// The pass visits the tasks whose land is pending first, each group in the order given,
// so a land that waited for its rebase Job runs before another task's land.
func TestLandPendingTasksAreVisitedFirst(t *testing.T) {
	h := newStreamHome(t, 3)
	for i := range h.dirs {
		h.post(i, "J-1")
	}
	var order []string
	s := h.stream(4242, nil)
	s.LandPending = func(dir string) bool { return dir == h.dirs[2] }
	s.Ticked = func(dir string, _ feed.Writer) { order = append(order, dir) }
	h.pass(s)
	if want := []string{h.dirs[2], h.dirs[0], h.dirs[1]}; !reflect.DeepEqual(order, want) {
		t.Fatalf("tasks ticked in order %v, want the pending one first: %v", order, want)
	}
}
