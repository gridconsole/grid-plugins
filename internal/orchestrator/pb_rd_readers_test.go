package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// TestPbRdProgressSkipsProposals walks the orchestrator row of the reader table
// (brain-obsidian.md 2.2, progress.go): a file in a task folder's Proposals/ is a
// brain proposal, not work product, so it never moves the progress hash, as one
// in brain-proposals/ (the old folder) does not; a file anywhere else does.
func TestPbRdProgressSkipsProposals(t *testing.T) {
	dir := t.TempDir()
	put := func(rel string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := Store{Feed: feed.Store{TaskDir: dir}}
	progress := func() string {
		t.Helper()
		h, err := s.fileProgress(&memory{})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	put("result.md")
	base := progress()
	for _, rel := range []string{"Proposals/idea.md", "brain-proposals/old.md"} {
		put(rel)
		if got := progress(); got != base {
			t.Errorf("%s moved the progress hash", rel)
		}
	}
	put("notes/result2.md")
	if progress() == base {
		t.Error("a work product did not move the progress hash")
	}
}
