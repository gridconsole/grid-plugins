package brain

import "testing"

// pbFmBareOriginNote is a person note with an empty `origin:` property, which is what Obsidian leaves when a person
// adds the property and fills nothing in.
const pbFmBareOriginNote = "---\nname: cafe-plan\ntopic: conventions\ntype: note\ndescription: Cafe plan\nsummary: Cafe plan\n" +
	"updated: 2026-11-13\nsource: Brain page\nlevel: project\nstatus: accepted\norigin:\n---\n# Cafe plan\n\nPick a table by the window.\n"

// TestPbFmBareOriginIsNotAPersonNote: fix1 had this test assert that a bare `origin:` reads as a person note. Fix2 reverses
// that on purpose: the app's TestCaPnRealPluginResave (app main, not changeable from this repo) pins that a bare origin is
// refused as a bad note, so a filed note may leave the origin key out but not leave it empty.
func TestPbFmBareOriginIsNotAPersonNote(t *testing.T) {
	n, err := Parse([]byte(pbFmBareOriginNote))
	if err == nil {
		err = n.Validate()
	}
	if err == nil {
		t.Fatal("a filed note with a bare origin is accepted")
	}
}

// TestPbFmBadOriginStillRefused: an origin the note does name must still be valid, so the refusal the page's re-save test
// looks for still has a case that holds.
func TestPbFmBadOriginStillRefused(t *testing.T) {
	bad := "---\nname: cafe-plan\ntopic: conventions\ntype: note\ndescription: Cafe plan\nsummary: Cafe plan\n" +
		"updated: 2026-11-13\nsource: Brain page\nlevel: project\nstatus: accepted\norigin:\n  kind: bogus\n  id: brain-page\n---\n# Cafe plan\n"
	n, err := Parse([]byte(bad))
	if err == nil {
		err = n.Validate()
	}
	if err == nil {
		t.Fatal("a note whose origin kind is not one of session, job, server, import is accepted")
	}
}
