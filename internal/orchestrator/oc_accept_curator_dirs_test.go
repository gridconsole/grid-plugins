package orchestrator

import (
	"path/filepath"
	"testing"
)

// A proposal that appears in a folder ProposalDirs names, outside the task folder, asks the hook again.
func TestOcProposalInAWatchedFolderAsksAgain(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "builder")
	home := t.TempDir()
	hook := newUsClosedHook(t)
	hook.filed()
	s := h.stream(4242, nil)
	s.ClosedCurator = hook.ask
	s.ProposalDirs = func(dir string) []string {
		return []string{filepath.Join(home, "brain-proposals"), filepath.Join(home, "projects", "p", "brain-proposals")}
	}
	h.pass(s)
	ocAccept(t, h, s, 0, "J-1")
	for i := 0; i < 3; i++ {
		h.pass(s)
	}
	if got := len(hook.asked()); got != 1 {
		t.Fatalf("nothing waits and nothing changed: want the hook asked once, got %d", got)
	}
	ocPropose(t, filepath.Join(home, "projects", "p", "brain-proposals"), "p-1.md")
	hook.ocProposed()
	h.pass(s)
	if got := len(hook.asked()); got != 2 {
		t.Fatalf("a project proposal appeared: want the hook asked again, got %d calls", got)
	}
	if got := len(usClosedCurators(t, h, 0)); got != 1 {
		t.Fatalf("want one curator Job, got %d", got)
	}
}
