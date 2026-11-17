package brain

import (
	"strings"
	"testing"
)

// pbFmOriginNote is a filed note whose front matter ends with the given origin lines ("" for no origin key).
func pbFmOriginNote(origin string) string {
	return "---\nname: cafe-plan\ntopic: conventions\ntype: note\ndescription: Cafe plan\nsummary: Cafe plan\n" +
		"updated: 2026-11-13\nsource: Brain page\nlevel: project\nstatus: accepted\n" + origin + "---\n# Cafe plan\n\nPick a table by the window.\n"
}

// TestPbFmBareOriginRefused: an `origin:` key that is there but holds nothing is refused as a bad note, as it was before
// the filed form, because the page's re-save test (app TestCaPnRealPluginResave) pins that refusal; a note with no origin
// key at all is still a filed note and reads.
func TestPbFmBareOriginRefused(t *testing.T) {
	n, err := Parse([]byte(pbFmOriginNote("origin:\n")))
	if err == nil {
		err = n.Validate()
	}
	if err == nil {
		t.Fatal("a filed note with a bare origin key is accepted")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("the refusal does not name origin: %v", err)
	}
	for _, empty := range []string{"origin: null\n", "origin: ~\n", "origin: {}\n"} {
		n, err := Parse([]byte(pbFmOriginNote(empty)))
		if err == nil {
			err = n.Validate()
		}
		if err == nil {
			t.Errorf("a filed note with %q is accepted", empty)
		}
	}
}

// TestPbFmNoOriginKeyReads: a filed note with no origin key reads and validates (4.5).
func TestPbFmNoOriginKeyReads(t *testing.T) {
	n, err := Parse([]byte(pbFmOriginNote("")))
	if err != nil {
		t.Fatalf("a filed note with no origin key does not parse: %v", err)
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("a filed note with no origin key is refused: %v", err)
	}
}
