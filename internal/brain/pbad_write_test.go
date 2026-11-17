package brain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	pbAdBase = "---\ntitle: Plan\nowner: sam\n---\n# Plan\n\nFirst section.\n\n## Later\n\nSecond section.\n"
)

func pbAdProposals(t *testing.T, s Store) []string {
	t.Helper()
	var out []string
	for _, p := range pbVrTree(t, s.BrainDir) {
		if strings.HasPrefix(p, "Proposals/") {
			out = append(out, p)
		}
	}
	return out
}

func pbAdAgent() Origin { return Origin{Kind: "agent", ID: "session-1"} }

// pbAdRead is what an agent does before it changes a note: read it through Grid, so
// the bytes it saw are recorded and keep a version, and take the rev.
func pbAdRead(t *testing.T, s Store, id string) string {
	t.Helper()
	if _, err := s.Show("brain:home/" + id); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.BrainDir, filepath.FromSlash(id)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return NoteRev(b)
}

// F9 other lines (row 3): a person's edit to one section lands between the agent's
// read and its write; the agent's patch changes another section. Both are kept and
// no proposal is written.
func TestPbAdOtherSectionIsRedoneOnTop(t *testing.T) {
	s := pbAdVault(t)
	rev := pbAdRead(t, s, "Notes/Plan")
	person := strings.Replace(pbAdBase, "First section.", "First section, and a line a person added.", 1)
	pbAdEdit(t, s, "Notes/Plan.md", person)
	agent := strings.Replace(pbAdBase, "Second section.", "Second section, as the agent changed it.", 1)
	res, err := s.Write("home", "Notes", "Plan", rev, []byte(agent), pbAdAgent(), "why", false)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(person, "Second section.", "Second section, as the agent changed it.", 1)
	got, _ := os.ReadFile(filepath.Join(s.BrainDir, "Notes/Plan.md"))
	if string(got) != want {
		t.Fatalf("note =\n%q\nwant\n%q", got, want)
	}
	if res.Status != "edited" || res.Path != filepath.Join(s.BrainDir, "Notes/Plan.md") || res.Proposed != "" {
		t.Errorf("result = %+v", res)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}
	top, _ := s.openTop()
	defer top.Close()
	if b, err := s.readVersion(top, "home", "Notes/Plan", NoteRev([]byte(person))); err != nil || string(b) != person {
		t.Errorf("the person's bytes are not kept under .grid/versions: %v", err)
	}
}

// F9 same lines (row 4): the person's edit and the agent's patch change the same
// front-matter key. The agent's change goes to Proposals/ and the note stays the
// person's bytes.
func TestPbAdSameKeyMakesAProposal(t *testing.T) {
	s := pbAdVault(t)
	rev := pbAdRead(t, s, "Notes/Plan")
	person := strings.Replace(pbAdBase, "owner: sam", "owner: robin", 1)
	pbAdEdit(t, s, "Notes/Plan.md", person)
	agent := strings.Replace(pbAdBase, "owner: sam", "owner: kim", 1)
	res, err := s.Write("home", "Notes", "Plan", rev, []byte(agent), pbAdAgent(), "why", false)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(s.BrainDir, "Notes/Plan.md"))
	if string(got) != person {
		t.Fatalf("the note is not the person's bytes: %q", got)
	}
	p := pbAdProposals(t, s)
	if len(p) != 1 || res.Proposed != filepath.Join(s.BrainDir, p[0]) || res.Status != "proposed" {
		t.Fatalf("proposals %v, result %+v", p, res)
	}
	body, _ := os.ReadFile(res.Proposed)
	if !strings.Contains(string(body), "owner: kim") || !strings.Contains(string(body), "target: \"Notes/Plan\"") {
		t.Errorf("the proposal does not hold the agent's change and its target:\n%s", body)
	}
	// The listing shows the proposal and marks the note.
	rows := pbVrRows(t, s)
	if c := rows["brain:home/Notes/Plan"].Conflict; len(c) != 1 || !strings.HasPrefix(c[0], "brain:home/Proposals/") {
		t.Errorf("conflict = %v", c)
	}
}

// Row 5: an agent write to a person's note, based on the rev it has now, is written
// on top: the person's lines stay and the person's previous bytes are kept.
func TestPbAdWriteOnPersonNoteKeepsPersonBytes(t *testing.T) {
	s := pbAdVault(t)
	// No Grid record exists for this note: a person made it.
	pbAdEdit(t, s, "Notes/Hand.md", "A line a person wrote.\n")
	rev := NoteRev([]byte("A line a person wrote.\n"))
	agent := "A line a person wrote.\n\n## Added\n\nBy the agent.\n"
	res, err := s.Write("home", "Notes", "Hand", rev, []byte(agent), pbAdAgent(), "why", false)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(s.BrainDir, "Notes/Hand.md"))
	if string(got) != agent || res.Status != "edited" {
		t.Fatalf("note %q, result %+v", got, res)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}
	top, _ := s.openTop()
	defer top.Close()
	if b, err := s.readVersion(top, "home", "Notes/Hand", rev); err != nil || string(b) != "A line a person wrote.\n" {
		t.Errorf("the person's previous bytes are not under .grid/versions: %v", err)
	}
	// A rev nobody recorded cannot be redone on top: the old stale answer.
	_, err = s.Write("home", "Notes", "Hand", "0123456789abcdef", []byte("x\n"), pbAdAgent(), "why", false)
	var c *ConflictError
	if !errors.As(err, &c) || c.Rev != NoteRev(got) {
		t.Errorf("stale unknown rev: %v", err)
	}
}

// Row 9: a draft in Proposals/ changed on disk with no Grid record is adopted as
// edited, origin person, with its version under .grid/versions/<level>/Proposals/.
func TestPbAdProposalEditIsAdopted(t *testing.T) {
	s := pbAdVault(t)
	draft := "---\nstatus: proposed\ntarget: \"Notes/Plan\"\n---\n# Draft\n\nText.\n"
	pbAdEdit(t, s, "Proposals/draft.md", draft)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	edited := draft + "A person added this.\n"
	pbAdEdit(t, s, "Proposals/draft.md", edited)
	res, err := s.Check()
	if err != nil || res.Adopted != 1 || len(res.Findings) != 0 {
		t.Fatalf("Check() = %+v, %v", res, err)
	}
	recs := pbAdOf(pbAdRecords(t, s), "edited", "Proposals/draft")
	if len(recs) != 1 || recs[0].OriginKind != "person" || recs[0].Hash != NoteRev([]byte(edited)) {
		t.Fatalf("records = %+v", recs)
	}
	vp := filepath.Join(s.BrainDir, ".grid/versions/home/Proposals/draft", NoteRev([]byte(edited))+".version")
	if b, err := os.ReadFile(vp); err != nil || string(b) != edited {
		t.Errorf("version file: %v", err)
	}
	row, ok := pbVrRows(t, s)["brain:home/Proposals/draft"]
	if !ok || row.Origin != "person" || row.Kind != "proposal" {
		t.Errorf("listing row = %+v (found %v)", row, ok)
	}
}
