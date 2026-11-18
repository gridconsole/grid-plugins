package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// ocProposed says proposals wait again: later calls answer a curator body.
func (k *usClosedHook) ocProposed() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.waiting = true
}

// ocPropose writes a proposal file into the proposals folder at folder.
func ocPropose(t *testing.T, folder, name string) {
	t.Helper()
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, name), []byte("---\nstatus: proposed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// With nothing waiting, an accepted Job asks the hook once, not on every pass; a
// proposal file that appears asks it again and posts one curator Job.
func TestOcNothingWaitingAsksOnceUntilAProposalAppears(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder")
	hook := newUsClosedHook(t)
	hook.filed()
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	h.pass(s)
	ocAccept(t, h, s, 0, "J-1")
	for i := 0; i < 5; i++ {
		h.pass(s)
	}
	if got := len(hook.asked()); got != 1 {
		t.Fatalf("nothing waits and nothing changed: want the hook asked once over the passes, got %d", got)
	}
	ocPropose(t, filepath.Join(h.dirs[0], "brain-proposals"), "p-1.md")
	hook.ocProposed()
	h.pass(s)
	if got := len(hook.asked()); got != 2 {
		t.Fatalf("a proposal file appeared: want the hook asked again, got %d calls", got)
	}
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Fatalf("want one curator Job once a proposal waits, got %d", got)
	}
}

// A proposal that cannot be read prints one Diagnostics line, not one per pass; changing the
// proposal prints one more.
func TestOcUnreadableProposalDiagPrintsOncePerChange(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder")
	calls := 0
	s := h.stream(4242, nil)
	s.ClosedCurator = func(dir, taskID, id string) (map[string]any, error) {
		calls++
		return nil, errors.New("proposal p-1.md has an invalid or missing status")
	}
	h.pass(s)
	ocPropose(t, filepath.Join(h.dirs[0], "brain-proposals"), "p-1.md")
	ocAccept(t, h, s, 0, "J-1")
	for i := 0; i < 5; i++ {
		h.pass(s)
	}
	const line = "no curator Job for the waiting proposals"
	if got := strings.Count(h.diag.String(), line); got != 1 {
		t.Fatalf("want the unreadable proposal named once over the passes, got %d:\n%s", got, h.diag.String())
	}
	ocPropose(t, filepath.Join(h.dirs[0], "brain-proposals"), "p-2.md")
	h.pass(s)
	if got := strings.Count(h.diag.String(), line); got != 2 {
		t.Fatalf("want one more line after the proposals changed, got %d", got)
	}
	if calls != 2 {
		t.Fatalf("want the hook asked twice, got %d", calls)
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
