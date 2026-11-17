package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The version check on every write path of a vault (docs/research/brain-obsidian.md
// 5.4 step 3 and 6.1): a person's edit between a verb's read and its write is
// redone on top when both edits hold, and a proposal is written only when they
// cannot. vaultBeforeWrite makes the person's edit land in that window.

const pbAfCfg = "---\ntitle: Daily\ncron: 0 9 * * *\n---\nRun it.\n"

// pbAfHook runs fn once, between the read and the write of the verb that changes
// the note id.
func pbAfHook(t *testing.T, id string, fn func()) {
	t.Helper()
	done := false
	vaultBeforeWrite = func(_, got string) {
		if got == id && !done {
			done = true
			fn()
		}
	}
	t.Cleanup(func() { vaultBeforeWrite = nil })
}

func pbAfRead(t *testing.T, s Store, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.BrainDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func pbAfExists(s Store, rel string) bool {
	_, err := os.Stat(filepath.Join(s.BrainDir, filepath.FromSlash(rel)))
	return err == nil
}

// EditConfigBy (row 1): a person edits one key of a config entry between the read
// and the write; the agent changes another. Both stay. Both change the same key:
// the agent's change is a proposal and the entry keeps the person's bytes.
func TestPbAfEditConfigByRedoneOnTop(t *testing.T) {
	s := pbAdVault(t)
	ag := pbAdAgent()
	create := func([]byte) ([]byte, error) { return []byte(pbAfCfg), nil }
	if err := s.EditConfigBy("home", "routines", "daily", ag, create); err != nil {
		t.Fatal(err)
	}
	if got := pbAfRead(t, s, "grid/routines/daily.md"); got != pbAfCfg {
		t.Fatalf("entry = %q", got)
	}
	person := strings.Replace(pbAfCfg, "title: Daily", "title: Daily report", 1)
	pbAfHook(t, "grid/routines/daily", func() { pbAdEdit(t, s, "grid/routines/daily.md", person) })
	err := s.EditConfigBy("home", "routines", "daily", ag, func(old []byte) ([]byte, error) {
		return []byte(strings.Replace(string(old), "cron: 0 9 * * *", "cron: 0 10 * * *", 1)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(person, "cron: 0 9 * * *", "cron: 0 10 * * *", 1)
	if got := pbAfRead(t, s, "grid/routines/daily.md"); got != want {
		t.Fatalf("entry =\n%q\nwant\n%q", got, want)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}

	// The same key on both sides cannot hold.
	person2 := strings.Replace(want, "cron: 0 10 * * *", "cron: 0 11 * * *", 1)
	pbAfHook(t, "grid/routines/daily", func() { pbAdEdit(t, s, "grid/routines/daily.md", person2) })
	err = s.EditConfigBy("home", "routines", "daily", ag, func(old []byte) ([]byte, error) {
		return []byte(strings.Replace(string(old), "cron: 0 10 * * *", "cron: 0 12 * * *", 1)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pbAfRead(t, s, "grid/routines/daily.md"); got != person2 {
		t.Fatalf("the entry is not the person's bytes: %q", got)
	}
	p := pbAdProposals(t, s)
	if len(p) != 1 || !strings.Contains(pbAfRead(t, s, p[0]), "cron: 0 12 * * *") {
		t.Fatalf("proposals %v", p)
	}
}

// Supersede (row 1): a person adds a line to the new note between the read and the
// write of the link. The link goes on top of the line.
func TestPbAfSupersedeRedoneOnTop(t *testing.T) {
	s := pbAdVault(t)
	pbAfHook(t, "Notes/Other", func() { pbAdEdit(t, s, "Notes/Other.md", "Other note.\nA line a person added.\n") })
	res, err := s.Supersede("brain:home/Notes/Plan", "brain:home/Notes/Other", "the other note replaces the plan", pbAdAgent())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Wrote || len(res.Proposed) != 0 {
		t.Errorf("result = %+v", res)
	}
	other := pbAfRead(t, s, "Notes/Other.md")
	if !strings.Contains(other, "A line a person added.") || !strings.Contains(other, "- [[Notes/Plan]] - supersedes: the other note replaces the plan") {
		t.Errorf("the new note holds the person's line and the link:\n%s", other)
	}
	plan, err := ParseVault([]byte(pbAfRead(t, s, "Notes/Plan.md")))
	if err != nil || frontString(plan.Front["superseded_by"]) != "brain:home/Notes/Other" {
		t.Errorf("the old note is not marked: %+v, %v", plan.Front, err)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}
	// Doing it again writes nothing.
	again, err := s.Supersede("brain:home/Notes/Plan", "brain:home/Notes/Other", "the other note replaces the plan", pbAdAgent())
	if err != nil || again.Wrote {
		t.Errorf("repeat = %+v, %v", again, err)
	}
}

// Revert (row 1): the agent puts version 1 back while a person edits another
// section of the note. The revert is written on top of the person's lines.
func TestPbAfRevertRedoneOnTop(t *testing.T) {
	s := pbAdVault(t)
	ag := pbAdAgent()
	rev := pbAdRead(t, s, "Notes/Plan")
	v2 := strings.Replace(pbAdBase, "Second section.", "Second section, as the agent changed it.", 1)
	if _, err := s.Write("home", "Notes", "Plan", rev, []byte(v2), ag, "why", false); err != nil {
		t.Fatal(err)
	}
	person := strings.Replace(v2, "First section.", "First section, and a line a person added.", 1)
	pbAfHook(t, "Notes/Plan", func() { pbAdEdit(t, s, "Notes/Plan.md", person) })
	res, err := s.RevertTo("home", "Notes", "Plan", 1, ag)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(person, "Second section, as the agent changed it.", "Second section.", 1)
	if got := pbAfRead(t, s, "Notes/Plan.md"); got != want {
		t.Fatalf("note =\n%q\nwant\n%q", got, want)
	}
	if !res.Wrote || res.Proposed != "" || res.Version != 1 {
		t.Errorf("result = %+v", res)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}
	if _, err := s.RevertTo("home", "Notes", "Plan", 9, ag); err == nil || !strings.Contains(err.Error(), "no version 9") {
		t.Errorf("a version that was never recorded: %v", err)
	}
}

// Drop (row 1): a drop cannot keep a person's edit made since the agent read the
// note, so it is a proposal and the note stays. Without an edit it drops, the
// bytes stay under .grid/versions, and a restore brings them back.
func TestPbAfDropWithPersonEditIsAProposal(t *testing.T) {
	s := pbAdVault(t)
	ag := pbAdAgent()
	rev := pbAdRead(t, s, "Notes/Plan")
	person := pbAdBase + "A person's line.\n"
	pbAdEdit(t, s, "Notes/Plan.md", person)
	res, err := s.Drop("home", "Notes", "Plan", OpDrop, rev, ag, "why")
	if err != nil {
		t.Fatal(err)
	}
	p := pbAdProposals(t, s)
	if res.Status != "proposed" || len(p) != 1 || res.Proposed != filepath.Join(s.BrainDir, p[0]) {
		t.Fatalf("result %+v, proposals %v", res, p)
	}
	if got := pbAfRead(t, s, "Notes/Plan.md"); got != person {
		t.Fatalf("the note is not the person's bytes: %q", got)
	}
	if body := pbAfRead(t, s, p[0]); !strings.Contains(body, "delete the note") {
		t.Errorf("the proposal does not say what was asked:\n%s", body)
	}

	res, err = s.Drop("home", "Notes", "Plan", OpDrop, NoteRev([]byte(person)), ag, "why")
	if err != nil || res.Status != "dropped" || pbAfExists(s, "Notes/Plan.md") {
		t.Fatalf("drop = %+v, %v", res, err)
	}
	top, _ := s.openTop()
	defer top.Close()
	if b, err := s.readVersion(top, "home", "Notes/Plan", NoteRev([]byte(person))); err != nil || string(b) != person {
		t.Errorf("the dropped bytes are not under .grid/versions: %v", err)
	}
	res, err = s.Drop("home", "Notes", "Plan", OpRestore, "none", ag, "why")
	if err != nil || res.Status != "restored" || pbAfRead(t, s, "Notes/Plan.md") != person {
		t.Fatalf("restore = %+v, %v", res, err)
	}
}

// Rename (row 1): the person's edit made since the agent read the note comes along
// with the file, so both edits hold; when the new name is taken a rename cannot
// hold and is a proposal.
func TestPbAfRenameCarriesPersonEdit(t *testing.T) {
	s := pbAdVault(t)
	ag := pbAdAgent()
	rev := pbAdRead(t, s, "Notes/Plan")
	person := strings.Replace(pbAdBase, "First section.", "First section, edited by a person.", 1)
	pbAdEdit(t, s, "Notes/Plan.md", person)
	res, err := s.Rename("home", "Notes", "Plan", "Roadmap", rev, ag, "why")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "renamed" || res.To != "Notes/Roadmap" || res.Proposed != "" {
		t.Errorf("result = %+v", res)
	}
	if pbAfExists(s, "Notes/Plan.md") || pbAfRead(t, s, "Notes/Roadmap.md") != person {
		t.Fatalf("the file did not move with the person's edit")
	}
	if e, err := s.Show("brain:home/Notes/Plan"); err != nil || e.ID != "brain:home/Notes/Roadmap" {
		t.Errorf("the old id does not resolve to the new note: %+v, %v", e.ID, err)
	}
	if p := pbAdProposals(t, s); len(p) != 0 {
		t.Errorf("proposals written: %v", p)
	}

	res, err = s.Rename("home", "Notes", "Roadmap", "Other", NoteRev([]byte(person)), ag, "why")
	if err != nil {
		t.Fatal(err)
	}
	p := pbAdProposals(t, s)
	if res.Status != "proposed" || len(p) != 1 || pbAfRead(t, s, "Notes/Roadmap.md") != person || pbAfRead(t, s, "Notes/Other.md") != "Other note.\n" {
		t.Fatalf("result %+v, proposals %v", res, p)
	}
}
