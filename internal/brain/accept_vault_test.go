package brain

import (
	"path/filepath"
	"testing"
)

// With a brain folder set, an accepted note is the one search finds.
func TestAcceptedNoteIsSearchableInVault(t *testing.T) {
	s := testStore(t)
	s.BrainDir = filepath.Join(t.TempDir(), "vault")
	n := sample()
	n.Name, n.Summary = "zebrafinch", "Zebrafinch migration rules."
	accepted(t, s, n)
	hits, err := s.Search("zebrafinch", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID == "" {
		t.Fatalf("accepted note not found by search: %+v", hits)
	}
}
