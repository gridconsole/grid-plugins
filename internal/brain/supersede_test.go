package brain

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Tests for accepting a note that links another with rel "supersedes": the old
// note is marked superseded_by the new one, and nothing is ever deleted.

func ssPath(s Store, topic, name string) string {
	return filepath.Join(s.GridHome, "projects", "example", "knowledge", topic, name+".md")
}

func ssLink(to string) Link {
	return Link{To: to, Rel: "supersedes", Summary: "The older note this one replaces.", Why: "The older note is out of date now."}
}

// ssNew is a proposal that supersedes the given targets.
func ssNew(name string, targets ...string) Note {
	n := extra(name)
	for _, to := range targets {
		n.Links = append(n.Links, ssLink(to))
	}
	return n
}

func ssPropose(t *testing.T, s Store, n Note) {
	t.Helper()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
}

func ssRead(t *testing.T, s Store, name string) []byte {
	t.Helper()
	return published(t, s, "conventions", name)
}

func ssParse(t *testing.T, b []byte) Note {
	t.Helper()
	n, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func ssSuperseded(t *testing.T, s Store, name string) string {
	t.Helper()
	e, err := s.Show("brain:project/" + name)
	if err != nil {
		t.Fatalf("show %s: %v", name, err)
	}
	return e.Note.SupersededBy
}

func ssLogCount(t *testing.T, s Store) int {
	t.Helper()
	return strings.Count(readLog(t, s), "| superseded |")
}

func TestAcceptMarksTheSupersededNote(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	before := ssRead(t, s, "deploy")
	wantChain(t, s, "conventions", "deploy", 1, before)

	n := ssNew("deploy-v2", "brain:project/deploy")
	ssPropose(t, s, n)
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}

	// Show still finds the old note and shows the field.
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/deploy-v2" {
		t.Fatalf("superseded_by of the old note is %q, want brain:project/deploy-v2", got)
	}

	// The file is still there; only that field changed.
	after := ssRead(t, s, "deploy")
	want := ssParse(t, before)
	want.SupersededBy = "brain:project/deploy-v2"
	if got := ssParse(t, after); !reflect.DeepEqual(got, want) {
		t.Fatalf("old note changed beyond superseded_by:\n got %#v\nwant %#v", got, want)
	}

	// A new version holds the new bytes, and the log has one line for it.
	v := wantChain(t, s, "conventions", "deploy", 2, after)
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "superseded", "deploy", "job:T-01/J-01",
		"superseded by brain:project/deploy-v2", "conventions", "v2", v[1].Hash+"\n")
	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range changes {
		if c.Status == "superseded" && c.Name == "deploy" && c.Topic == "conventions" && c.Version == 2 && c.Hash == v[1].Hash {
			found = true
		}
	}
	if !found {
		t.Fatalf("Changes has no superseded line for deploy: %+v", changes)
	}

	// The new note is untouched and live; Search drops only the old one.
	if got := ssSuperseded(t, s, "deploy-v2"); got != "" {
		t.Fatalf("the new note is marked superseded by %q", got)
	}
	hits, err := s.Search("deploy", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	if !reflect.DeepEqual(ids, []string{"brain:project/deploy-v2"}) {
		t.Fatalf("Search returned %v, want only brain:project/deploy-v2", ids)
	}
}

func TestIndexLeavesTheSupersededNoteOut(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	row := func() TopicRow {
		t.Helper()
		idx, err := s.Index()
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range idx.Topics {
			if r.Topic == "conventions" {
				if !strings.Contains(idx.Text, "\nconventions | ") {
					t.Fatalf("session text has no conventions row:\n%s", idx.Text)
				}
				return r
			}
		}
		t.Fatal("no conventions row")
		return TopicRow{}
	}
	if got := row().Entries; got != 1 {
		t.Fatalf("one live note counts %d entries", got)
	}
	ssPropose(t, s, ssNew("deploy-v2", "brain:project/deploy"))
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	if got := row().Entries; got != 1 {
		t.Fatalf("two notes, one superseded: %d entries, want 1", got)
	}
	idx, err := s.RebuildIndex()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(idx.Text, "\n") {
		if strings.HasPrefix(line, "conventions | ") && !strings.Contains(line, " | 1 | ") {
			t.Fatalf("session text counts the superseded note: %q", line)
		}
	}
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "index.md"))
	if err != nil || !strings.Contains(string(b), "conventions | ") || strings.Contains(string(b), "| 2 |") {
		t.Fatalf("index.md counts the superseded note: %v\n%s", err, b)
	}
}

// A target that is missing, at the other level or not a note is a plain link:
// the accept succeeds, nothing is marked.
func TestSupersedesLinkWithoutATargetIsAPlainLink(t *testing.T) {
	cases := map[string]string{
		"missing note":               "brain:project/ghost",
		"other level":                "brain:home/deploy",
		"job target":                 "job:T-01/J-01",
		"person target":              "person:ada",
		"the new note":               "brain:project/deploy-v2",
		"the new note and an anchor": "brain:project/deploy-v2#top",
	}
	for name, to := range cases {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			home := extra("deploy")
			home.Level, home.Topic = "home", "decisions"
			accepted(t, s, home)
			accepted(t, s, extra("unrelated"))
			homeBefore, err := os.ReadFile(filepath.Join(s.GridHome, "knowledge", "decisions", "deploy.md"))
			if err != nil {
				t.Fatal(err)
			}
			unrelated := ssRead(t, s, "unrelated")

			ssPropose(t, s, ssNew("deploy-v2", to))
			if _, err := s.Accept("project", "deploy-v2"); err != nil {
				t.Fatalf("accept refused a %s link: %v", name, err)
			}
			// The link is published as written.
			e, err := s.Show("brain:project/deploy-v2")
			if err != nil || len(e.Note.Links) != 1 || e.Note.Links[0].To != to || e.Note.Links[0].Rel != "supersedes" {
				t.Fatalf("link not published as written: %+v, %v", e.Note.Links, err)
			}
			if got := ssSuperseded(t, s, "unrelated"); got != "" {
				t.Fatalf("unrelated note marked %q", got)
			}
			if !bytes.Equal(ssRead(t, s, "unrelated"), unrelated) {
				t.Fatal("an unrelated note was rewritten")
			}
			homeAfter, err := os.ReadFile(filepath.Join(s.GridHome, "knowledge", "decisions", "deploy.md"))
			if err != nil || !bytes.Equal(homeAfter, homeBefore) {
				t.Fatalf("the home note was rewritten: %v", err)
			}
			wantChain(t, s, "conventions", "unrelated", 1, unrelated)
			if ssLogCount(t, s) != 0 {
				t.Fatalf("a superseded line was logged:\n%s", readLog(t, s))
			}
		})
	}
}

// One accept can replace several notes; a link with an anchor names its note; a
// note that is already superseded keeps the mark it has, and the new note's own
// name is never marked.
func TestSupersedesTargetsAreMarkedOnceEach(t *testing.T) {
	s := testStore(t)
	for _, name := range []string{"deploy", "rollout", "legacy"} {
		accepted(t, s, extra(name))
	}
	ssPropose(t, s, ssNew("mid", "brain:project/legacy"))
	if _, err := s.Accept("project", "mid"); err != nil {
		t.Fatal(err)
	}
	legacy := ssRead(t, s, "legacy")

	ssPropose(t, s, ssNew("deploy-v2", "brain:project/deploy#rollback", "brain:project/rollout", "brain:project/deploy",
		"brain:project/legacy", "brain:project/deploy-v2"))
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"deploy": "brain:project/deploy-v2", "rollout": "brain:project/deploy-v2",
		"legacy": "brain:project/mid", "mid": "", "deploy-v2": ""} {
		if got := ssSuperseded(t, s, name); got != want {
			t.Errorf("%s: superseded_by %q, want %q", name, got, want)
		}
	}
	// deploy was named twice: one version, one line.
	wantChain(t, s, "conventions", "deploy", 2, ssRead(t, s, "deploy"))
	wantChain(t, s, "conventions", "rollout", 2, ssRead(t, s, "rollout"))
	wantChain(t, s, "conventions", "legacy", 2, legacy)
	if got := strings.Count(readLog(t, s), "| superseded by brain:project/deploy-v2 |"); got != 2 {
		t.Fatalf("%d lines for deploy-v2, want 2:\n%s", got, readLog(t, s))
	}
}

// A repeated Accept is the same decision: it writes nothing more, and it puts
// back an audit line that an interrupted accept never wrote.
func TestRepeatedAcceptMarksOnce(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	ssPropose(t, s, ssNew("deploy-v2", "brain:project/deploy"))
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	marked := ssRead(t, s, "deploy")
	log := readLog(t, s)

	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) || readLog(t, s) != log {
		t.Fatal("a repeated accept wrote again")
	}
	wantChain(t, s, "conventions", "deploy", 2, marked)

	// The accept stopped after the mark and before its audit line.
	lines := strings.SplitAfter(log, "\n")
	kept := []string{}
	for _, l := range lines {
		if !strings.Contains(l, "| superseded |") {
			kept = append(kept, l)
		}
	}
	if err := os.WriteFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "log.md"), []byte(strings.Join(kept, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	if ssLogCount(t, s) != 0 {
		t.Fatal("test did not remove the line")
	}
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	if readLog(t, s) != log {
		t.Fatalf("the retry did not restore the line:\n%s", readLog(t, s))
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) {
		t.Fatal("the retry rewrote the old note")
	}
	wantChain(t, s, "conventions", "deploy", 2, marked)
}

func TestDecideTaskAcceptMarksTheSupersededNoteOnce(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	task := taskWithProposal(t, t.TempDir(), ssNew("deploy-v2", "brain:project/deploy"))
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "deploy-v2", "accept", "brain:project/deploy-v2", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/deploy-v2" {
		t.Fatalf("superseded_by of the old note is %q", got)
	}
	marked := ssRead(t, s, "deploy")
	wantChain(t, s, "conventions", "deploy", 2, marked)
	if ssLogCount(t, s) != 1 {
		t.Fatalf("want one superseded line:\n%s", readLog(t, s))
	}
	if w := tdReceipt(t, task, "deploy-v2"); !w.Done {
		t.Fatal("receipt is not Done")
	}
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range idx.Topics {
		if r.Topic == "conventions" && r.Entries != 1 {
			t.Fatalf("conventions counts %d entries, want 1", r.Entries)
		}
	}

	// Done: the same call again writes nothing.
	log := readLog(t, s)
	if _, err := s.DecideTask(task, "deploy-v2", "accept", "brain:project/deploy-v2", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) || readLog(t, s) != log {
		t.Fatal("a repeated DecideTask wrote again")
	}
	wantChain(t, s, "conventions", "deploy", 2, marked)

	// Not Done (stopped after publication): finishing it changes nothing either.
	w := tdReceipt(t, task, "deploy-v2")
	w.Done = false
	tdSetReceipt(t, task, "deploy-v2", w)
	if _, err := s.DecideTask(task, "deploy-v2", "accept", "brain:project/deploy-v2", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) || readLog(t, s) != log {
		t.Fatal("finishing a decision wrote the old note again")
	}
	wantChain(t, s, "conventions", "deploy", 2, marked)
}

func TestDecideTaskMergeIntoMarksTheSupersededNoteOnce(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	accepted(t, s, extra("deploy"))
	p := extra("second")
	p.Links = []Link{ssLink("brain:project/deploy")}
	task := taskWithProposal(t, t.TempDir(), p)
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/checks" {
		t.Fatalf("superseded_by of the old note is %q, want the merge target", got)
	}
	if got := ssSuperseded(t, s, "checks"); got != "" {
		t.Fatalf("the merge target is marked %q", got)
	}
	marked := ssRead(t, s, "deploy")
	wantChain(t, s, "conventions", "deploy", 2, marked)
	if ssLogCount(t, s) != 1 {
		t.Fatalf("want one superseded line:\n%s", readLog(t, s))
	}
	log := readLog(t, s)
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) || readLog(t, s) != log {
		t.Fatal("a repeated DecideTask wrote again")
	}
	wantChain(t, s, "conventions", "deploy", 2, marked)
}

// A note that is itself superseded does not take another over: that could leave
// two notes each superseded by the other, and neither found.
func TestMergeIntoASupersededNoteMarksNothing(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	ssPropose(t, s, ssNew("deploy-v2", "brain:project/deploy"))
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	p := extra("back")
	p.Links = []Link{ssLink("brain:project/deploy-v2")}
	task := taskWithProposal(t, t.TempDir(), p)
	if _, err := s.DecideTask(task, "back", "merge-into", "brain:project/deploy", "", (&tdHistory{}).fn); err != nil {
		t.Fatal(err)
	}
	if got := ssSuperseded(t, s, "deploy-v2"); got != "" {
		t.Fatalf("deploy-v2 is marked superseded by %q", got)
	}
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/deploy-v2" {
		t.Fatalf("deploy lost its mark: %q", got)
	}
}

// Reverting the new note puts its own earlier bytes back. It does not touch the
// old note, so the old note keeps superseded_by.
func TestRevertOfTheNewNoteLeavesTheOldNoteSuperseded(t *testing.T) {
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	ssPropose(t, s, ssNew("deploy-v2", "brain:project/deploy"))
	if _, err := s.Accept("project", "deploy-v2"); err != nil {
		t.Fatal(err)
	}
	task := taskWithProposal(t, t.TempDir(), extra("more"))
	if _, err := s.DecideTask(task, "more", "merge-into", "brain:project/deploy-v2", "", (&tdHistory{}).fn); err != nil {
		t.Fatal(err)
	}
	marked := ssRead(t, s, "deploy")
	res, err := s.RevertTo("project", "conventions", "deploy-v2", 1, Origin{Kind: "session", ID: "s1"})
	if err != nil || !res.Wrote {
		t.Fatalf("revert: %+v, %v", res, err)
	}
	if !bytes.Equal(ssRead(t, s, "deploy"), marked) {
		t.Fatal("reverting the new note rewrote the old note")
	}
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/deploy-v2" {
		t.Fatalf("superseded_by after the revert: %q", got)
	}
	// The old note's own history reverts to the unmarked copy and puts it back.
	if _, err := s.RevertTo("project", "conventions", "deploy", 1, Origin{Kind: "session", ID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if got := ssSuperseded(t, s, "deploy"); got != "" {
		t.Fatalf("reverting the old note to v1 leaves superseded_by %q", got)
	}
}
