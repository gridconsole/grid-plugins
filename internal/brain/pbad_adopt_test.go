package brain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pbAdVault is a small vault with a machine folder of its own. Every test edit
// changes the file's length, so the size-and-time shortcut of adoption never
// hides an edit made within one clock tick.
func pbAdVault(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	pbVrWrite(t, vault, "Notes/Plan.md", "---\ntitle: Plan\nowner: sam\n---\n# Plan\n\nFirst section.\n\n## Later\n\nSecond section.\n")
	pbVrWrite(t, vault, "Notes/Other.md", "Other note.\n")
	return Store{GridHome: filepath.Join(dir, "grid"), BrainDir: vault, Author: "Sam",
		MachineDir: filepath.Join(dir, "machine"), MachineID: "m-1"}
}

func pbAdRecords(t *testing.T, s Store) []recFile {
	t.Helper()
	top, err := s.openTop()
	if err != nil {
		t.Fatal(err)
	}
	defer top.Close()
	recs, bad, err := readRecords(top)
	if err != nil || len(bad) > 0 {
		t.Fatalf("records: %v, bad %v", err, bad)
	}
	return recs
}

func pbAdOf(recs []recFile, status, id string) []recFile {
	var out []recFile
	for _, r := range recs {
		if r.Status == status && r.ID == id {
			out = append(out, r)
		}
	}
	return out
}

func pbAdEdit(t *testing.T, s Store, rel, data string) {
	t.Helper()
	pbVrWrite(t, s.BrainDir, rel, data)
}

// F7 (row 1): a rename is recorded and the old id resolves.
func TestPbAdRenameIsRecordedAndOldIDResolves(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(s.BrainDir, "Notes/Plan.md"), filepath.Join(s.BrainDir, "Notes/Roadmap.md")); err != nil {
		t.Fatal(err)
	}
	n, err := s.Adopt()
	if err != nil || n != 1 {
		t.Fatalf("Adopt() = %d, %v; want 1, nil", n, err)
	}
	recs := pbAdRecords(t, s)
	got := pbAdOf(recs, "renamed", "Notes/Roadmap")
	if len(got) != 1 || got[0].OldID != "Notes/Plan" || got[0].OriginKind != "person" || got[0].Author != "Sam" {
		t.Fatalf("renamed records = %+v", got)
	}
	if len(pbAdOf(recs, "deleted", "Notes/Plan")) != 0 {
		t.Errorf("a rename also produced a deleted record")
	}
	e, err := s.Show("brain:home/Notes/Plan")
	if err != nil || e.ID != "brain:home/Notes/Roadmap" {
		t.Fatalf("Show(old id) = %q, %v; want brain:home/Notes/Roadmap", e.ID, err)
	}
	if _, err := s.Show("brain:home/Notes/Roadmap"); err != nil {
		t.Errorf("Show(new id): %v", err)
	}
}

// F8 (row 2): a delete is recorded.
func TestPbAdDeleteIsRecorded(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	want := NoteRev([]byte("Other note.\n"))
	if err := os.Remove(filepath.Join(s.BrainDir, "Notes/Other.md")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Adopt(); err != nil || n != 1 {
		t.Fatalf("Adopt() = %d, %v; want 1, nil", n, err)
	}
	got := pbAdOf(pbAdRecords(t, s), "deleted", "Notes/Other")
	if len(got) != 1 || got[0].OriginKind != "person" || got[0].Hash != want {
		t.Fatalf("deleted records = %+v, want one for rev %s", got, want)
	}
	// The deleted bytes are still under .grid/versions.
	top, _ := s.openTop()
	defer top.Close()
	if b, err := s.readVersion(top, "home", "Notes/Other", want); err != nil || string(b) != "Other note.\n" {
		t.Errorf("version of the deleted note: %q, %v", b, err)
	}
}

// Row 7: check adopts a person edit and reports no drift finding, with a count.
func TestPbAdCheckAdoptsWithoutDriftFinding(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	pbAdEdit(t, s, "Notes/Other.md", "Other note, edited by hand.\n")
	res, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 || res.Adopted != 1 {
		t.Fatalf("Check() = adopted %d, findings %+v; want 1 and none", res.Adopted, res.Findings)
	}
	recs := pbAdRecords(t, s)
	if got := pbAdOf(recs, "edited", "Notes/Other"); len(got) != 1 || got[0].OriginKind != "person" {
		t.Fatalf("edited records = %+v", got)
	}
	again, err := s.Check()
	if err != nil || again.Adopted != 0 || len(again.Findings) != 0 {
		t.Fatalf("second Check() = %+v, %v; want nothing adopted", again, err)
	}
}

// Row 8: two adoptions of the same edit are one logical change, also when a second
// machine saw the edit and wrote its own record.
func TestPbAdTwoAdoptionsAreOneChange(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	pbAdEdit(t, s, "Notes/Other.md", "Other note, edited by hand.\n")
	if n, err := s.Adopt(); err != nil || n != 1 {
		t.Fatalf("first Adopt() = %d, %v", n, err)
	}
	// The same machine again, and a second machine with an empty cache, adopt
	// nothing: the records already say what is on disk.
	if n, err := s.Adopt(); err != nil || n != 0 {
		t.Fatalf("repeat Adopt() = %d, %v; want 0", n, err)
	}
	other := s
	other.MachineDir, other.MachineID = filepath.Join(filepath.Dir(s.MachineDir), "machine-2"), "m-2"
	if n, err := other.Adopt(); err != nil || n != 0 {
		t.Fatalf("second machine Adopt() = %d, %v; want 0", n, err)
	}
	// Both computers adopted before either synced: the other's record of the same
	// edit arrives later and folds into the one change.
	top, _ := s.openTop()
	defer top.Close()
	dup := pbAdOf(pbAdRecords(t, s), "edited", "Notes/Other")[0].ChangeRecord
	if err := other.writeRecord(top, dup); err != nil {
		t.Fatal(err)
	}
	recs := pbAdRecords(t, s)
	if got := len(pbAdOf(recs, "edited", "Notes/Other")); got != 2 {
		t.Fatalf("%d raw records, want 2", got)
	}
	logical := 0
	for _, r := range logicalChanges(recs) {
		if r.Status == "edited" && r.ID == "Notes/Other" {
			logical++
		}
	}
	if logical != 1 {
		t.Errorf("%d logical changes, want 1", logical)
	}
	if live := s.foldRecords(recs).live[recKey{"home", "Notes/Other"}]; live != NoteRev([]byte("Other note, edited by hand.\n")) {
		t.Errorf("live rev = %s", live)
	}
}

// Row 10 and 11: a secret-shaped line gives one finding with the line number and
// no text; the note is not changed, no proposal is written, and the JSON holds no
// part of the matched value.
func TestPbAdSecretShapedLineIsOneFindingWithoutText(t *testing.T) {
	s := pbAdVault(t)
	const secret = "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	note := "# Keys\n\nSome words.\napi_key = " + secret + "\nMore words.\n"
	pbAdEdit(t, s, "Notes/Keys.md", note)
	before := pbVrTree(t, s.BrainDir)
	res, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	var found []Finding
	for _, f := range res.Findings {
		if f.Code == CheckSecretShaped {
			found = append(found, f)
		}
	}
	if len(found) != 1 || found[0].File != "Notes/Keys.md" || found[0].Line != 4 || found[0].Text != "" {
		t.Fatalf("secret-shaped findings = %+v", found)
	}
	if len(res.Findings) != 1 {
		t.Errorf("findings = %+v, want exactly the one", res.Findings)
	}
	if b, _ := os.ReadFile(filepath.Join(s.BrainDir, "Notes/Keys.md")); string(b) != note {
		t.Errorf("the note's bytes changed: %q", b)
	}
	for _, p := range pbVrTree(t, s.BrainDir) {
		if strings.HasPrefix(p, "Proposals/") {
			t.Errorf("a proposal was written: %s", p)
		}
	}
	_ = before
	js, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{secret, secret[4:20], "abcdefghijklmnop"} {
		if bytes.Contains(js, []byte(part)) {
			t.Errorf("the check JSON holds the matched value (%q): %s", part, js)
		}
	}
}
