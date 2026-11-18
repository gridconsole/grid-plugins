package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// ocAccept accepts Job id of task i under the task lease the stream holds, as a verdict does.
func ocAccept(t *testing.T, h *streamHome, s *Stream, i int, id string) {
	t.Helper()
	tn := h.tenure(i)
	w := feed.Writer{Author: feed.Author{ID: s.Member, Role: s.Role}, Worker: s.Holder(), Lease: "task", Gen: tn.Gen, Fence: tn.Fence}
	snap, err := Load(h.store(i))
	if err != nil {
		t.Fatal(err)
	}
	st := Store{Feed: h.store(i), Writer: w}
	if err := st.control(snap.State.Jobs[id], contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
}

// An accepted Job on an open task with waiting proposals posts one curator Job.
func TestOcAcceptedJobPostsACuratorJob(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	h.pass(s)
	if got := len(usClosedCurators(t, h, 0)); got != 0 {
		t.Fatalf("a curator Job was posted before any Job was accepted: %d", got)
	}
	ocAccept(t, h, s, 0, "J-1")
	h.pass(s)
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Fatalf("want one curator Job after J-1 was accepted, got %d", got)
	}
}

// With a curator Job open, a second accepted Job posts none.
func TestOcSecondAcceptWhileCuratorOpenPostsNone(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder", "builder")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	h.pass(s)
	ocAccept(t, h, s, 0, "J-1")
	h.pass(s)
	ocAccept(t, h, s, 0, "J-2")
	h.pass(s)
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Fatalf("want one curator Job while it is open, got %d", got)
	}
}

// A curator Job that is accepted with proposals still waiting does not earn another one.
func TestOcAcceptedCuratorPostsNoMore(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder")
	hook := newUsClosedHook(t)
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	h.pass(s)
	ocAccept(t, h, s, 0, "J-1")
	h.pass(s)
	ocAccept(t, h, s, 0, "J-3")
	h.pass(s)
	h.pass(s)
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Fatalf("want one curator Job, got %d", got)
	}
}
