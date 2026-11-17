package brain

import "testing"

// pbFmBareOriginNote is a person note with an empty `origin:` property, which is what Obsidian leaves when a person
// adds the property and fills nothing in.
const pbFmBareOriginNote = "---\nname: cafe-plan\ntopic: conventions\ntype: note\ndescription: Cafe plan\nsummary: Cafe plan\n" +
	"updated: 2026-11-13\nsource: Brain page\nlevel: project\nstatus: accepted\norigin:\n---\n# Cafe plan\n\nPick a table by the window.\n"

// TestPbFmBareOriginIsAPersonNote: an empty `origin:` is no origin, and a filed note needs none (4.5), so the note reads
// and validates instead of being refused as it was when origin was a required front matter field.
func TestPbFmBareOriginIsAPersonNote(t *testing.T) {
	n, err := Parse([]byte(pbFmBareOriginNote))
	if err != nil {
		t.Fatalf("a note with a bare origin does not parse: %v", err)
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("a filed note with a bare origin is refused: %v", err)
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
