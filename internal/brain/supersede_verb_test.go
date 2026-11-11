package brain

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Tests for Store.Supersede: marking a note superseded after both notes are
// already accepted.

const svReason = "moved to v2 pipeline"

var svOrigin = Origin{Kind: "session", ID: "abc123"}

// svTree reads every file under the Grid home, apart from the lock file that
// opening a level creates, so a refusal can be shown to have written nothing.
func svTree(t *testing.T, s Store) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(s.GridHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(s.GridHome, path)
		if strings.HasSuffix(filepath.ToSlash(rel), filepath.ToSlash(lockName)) {
			return nil
		}
		b, err := os.ReadFile(path)
		tree[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// svPair accepts deploy and deploy-v2, neither linked to the other.
func svPair(t *testing.T) Store {
	t.Helper()
	s := testStore(t)
	accepted(t, s, extra("deploy"))
	accepted(t, s, extra("deploy-v2"))
	return s
}

func svSupersedes(n Note) []Link {
	var out []Link
	for _, l := range n.Links {
		if l.Rel == "supersedes" {
			out = append(out, l)
		}
	}
	return out
}

func TestSupersedeMarksTheOldNoteAndLinksTheNewOne(t *testing.T) {
	s := svPair(t)
	oldBefore, newBefore := ssRead(t, s, "deploy"), ssRead(t, s, "deploy-v2")

	res, err := s.Supersede("deploy", "deploy-v2", svReason, svOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Wrote || res.Old != "brain:project/deploy" || res.New != "brain:project/deploy-v2" || res.Versions.Old != 2 || res.Versions.New != 2 {
		t.Fatalf("result: %+v", res)
	}

	// The old file is still there and only superseded_by changed.
	want := ssParse(t, oldBefore)
	want.SupersededBy = "brain:project/deploy-v2"
	oldAfter := ssRead(t, s, "deploy")
	if got := ssParse(t, oldAfter); !reflect.DeepEqual(got, want) {
		t.Fatalf("old note changed beyond superseded_by:\n got %#v\nwant %#v", got, want)
	}

	// The new note gained one link with the reason as its why, and nothing else
	// but the time it was updated.
	newAfter := ssRead(t, s, "deploy-v2")
	got := ssParse(t, newAfter)
	links := svSupersedes(got)
	if len(links) != 1 || links[0].To != "brain:project/deploy" || links[0].Why != svReason || links[0].Summary != want.Summary {
		t.Fatalf("new note links: %+v", got.Links)
	}
	wantNew := ssParse(t, newBefore)
	wantNew.Links, wantNew.Updated = got.Links, got.Updated
	if !reflect.DeepEqual(got, wantNew) || len(got.Links) != len(ssParse(t, newBefore).Links)+1 {
		t.Fatalf("new note changed beyond the link:\n got %#v\nwant %#v", got, wantNew)
	}

	// One new version and one audit line per changed file.
	oldV := wantChain(t, s, "conventions", "deploy", 2, oldAfter)
	newV := wantChain(t, s, "conventions", "deploy-v2", 2, newAfter)
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "superseded", "deploy", "session:abc123",
		"superseded by brain:project/deploy-v2: "+svReason, "conventions", "v2", oldV[1].Hash+"\n")
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "linked", "deploy-v2", "session:abc123",
		"supersedes brain:project/deploy: "+svReason, "conventions", "v2", newV[1].Hash+"\n")
	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range changes {
		if c.Status == "superseded" && c.Name == "deploy" && strings.HasSuffix(c.Reason, svReason) && c.Version == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("Changes has no superseded line with the reason: %+v", changes)
	}

	// Show still reads the old note; Search and the index leave it out.
	if e, err := s.Show("brain:project/deploy"); err != nil || e.Note.SupersededBy != "brain:project/deploy-v2" {
		t.Fatalf("show of the old note: %+v, %v", e.Note.SupersededBy, err)
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
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range idx.Topics {
		if r.Topic == "conventions" && r.Entries != 1 {
			t.Fatalf("conventions counts %d entries, want 1", r.Entries)
		}
	}
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "index.md"))
	if err != nil || strings.Contains(string(b), "| 2 |") {
		t.Fatalf("index.md was not rebuilt without the superseded note: %v\n%s", err, b)
	}
}

func TestSupersedeTwiceWritesNothingTheSecondTime(t *testing.T) {
	s := svPair(t)
	if _, err := s.Supersede("deploy", "deploy-v2", svReason, svOrigin); err != nil {
		t.Fatal(err)
	}
	before := svTree(t, s)
	res, err := s.Supersede("deploy", "brain:project/deploy-v2", svReason, svOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if res.Wrote || res.Versions.Old != 2 || res.Versions.New != 2 {
		t.Fatalf("second result: %+v", res)
	}
	if after := svTree(t, s); !reflect.DeepEqual(before, after) {
		t.Fatalf("the second call wrote something:\n%v\n%v", before, after)
	}
	// A different reason for the same pair is the same pair: still nothing.
	if res, err := s.Supersede("deploy", "deploy-v2", "a different reason entirely", svOrigin); err != nil || res.Wrote {
		t.Fatalf("third call: %+v, %v", res, err)
	}
	if after := svTree(t, s); !reflect.DeepEqual(before, after) {
		t.Fatal("the third call wrote something")
	}
}

// A link the new note already has is not added again; the old note is still marked.
func TestSupersedeKeepsALinkTheNewNoteAlreadyHas(t *testing.T) {
	s := testStore(t)
	// A link to a note that is not there yet is a plain link, so accept marks nothing.
	accepted(t, s, ssNew("deploy-v2", "brain:project/deploy"))
	accepted(t, s, extra("deploy"))
	newBefore := ssRead(t, s, "deploy-v2")

	res, err := s.Supersede("deploy", "deploy-v2", svReason, svOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Wrote || res.Versions.Old != 2 || res.Versions.New != 1 {
		t.Fatalf("result: %+v", res)
	}
	if got := ssRead(t, s, "deploy-v2"); string(got) != string(newBefore) {
		t.Fatalf("the new note was rewritten:\n%s", got)
	}
	if n := len(svSupersedes(ssParse(t, ssRead(t, s, "deploy-v2")))); n != 1 {
		t.Fatalf("new note has %d supersedes links, want 1", n)
	}
	if got := ssSuperseded(t, s, "deploy"); got != "brain:project/deploy-v2" {
		t.Fatalf("superseded_by: %q", got)
	}
	log := readLog(t, s)
	if strings.Count(log, "| linked |") != 0 || strings.Count(log, "| superseded |") != 1 {
		t.Fatalf("want one superseded line and no linked line:\n%s", log)
	}
}

func TestSupersedeRefusesAndWritesNothing(t *testing.T) {
	s := svPair(t)
	accepted(t, s, extra("deploy-v3"))
	home := extra("home-note")
	home.Level, home.Topic = "home", "decisions" // conventions is a project topic
	accepted(t, s, home)
	// deploy-v4 replaces deploy-v2 through the accept path, so deploy-v2 is superseded.
	ssPropose(t, s, ssNew("deploy-v4", "brain:project/deploy-v2"))
	if _, err := s.Accept("project", "deploy-v4"); err != nil {
		t.Fatal(err)
	}
	accepted(t, s, extra("other"))

	long := strings.Repeat("a few words ", 14) // 168 characters
	for _, c := range []struct {
		name, old, by, reason, want string
	}{
		{"unknown old", "missing", "deploy-v3", svReason, "not found"},
		{"unknown new", "deploy", "missing", svReason, "not found"},
		{"same note", "deploy", "deploy", svReason, "cannot supersede itself"},
		{"same note by id", "deploy", "brain:project/deploy", svReason, "cannot supersede itself"},
		{"new note superseded", "other", "deploy-v2", svReason, "itself superseded"},
		{"old note superseded by another", "deploy-v2", "deploy-v3", svReason, "already superseded"},
		{"project old, home new", "deploy", "brain:home/home-note", svReason, "different levels"},
		{"home old, project new", "brain:home/home-note", "deploy-v3", svReason, "different levels"},
		{"empty reason", "deploy", "deploy-v3", "", "reason must"},
		{"blank reason", "deploy", "deploy-v3", "   ", "reason must"},
		{"one-word reason", "deploy", "deploy-v3", "superseded", "reason must"},
		{"short reason", "deploy", "deploy-v3", "too old", "reason must"},
		{"long reason", "deploy", "deploy-v3", long, "reason must"},
		{"two-line reason", "deploy", "deploy-v3", "moved to v2\npipeline", "reason must"},
		{"secret-shaped reason", "deploy", "deploy-v3", "see the password: hunter2 for it", "secret"},
		{"bad id", "../deploy", "deploy-v3", svReason, "plain name"},
	} {
		t.Run(c.name, func(t *testing.T) {
			before := svTree(t, s)
			res, err := s.Supersede(c.old, c.by, c.reason, svOrigin)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want one containing %q", err, c.want)
			}
			if res.Wrote {
				t.Fatalf("a refusal reports a write: %+v", res)
			}
			if after := svTree(t, s); !reflect.DeepEqual(before, after) {
				t.Fatalf("a refusal wrote something")
			}
		})
	}
}

// A bare name takes the store's default level, so a bare old note and a home
// new note are a cross-level pair, and the home pair works on its own.
func TestSupersedeAtHomeLevel(t *testing.T) {
	s := testStore(t)
	for _, name := range []string{"home-old", "home-new"} {
		n := extra(name)
		n.Level, n.Topic = "home", "decisions"
		accepted(t, s, n)
	}
	home := s
	home.ReadLevel, home.Project = "home", ""
	res, err := home.Supersede("home-old", "home-new", svReason, svOrigin)
	if err != nil || !res.Wrote || res.Old != "brain:home/home-old" || res.Versions.Old != 2 || res.Versions.New != 2 {
		t.Fatalf("home pair: %+v, %v", res, err)
	}
	e, err := home.Show("brain:home/home-old")
	if err != nil || e.Note.SupersededBy != "brain:home/home-new" {
		t.Fatalf("home-old: %+v, %v", e.Note.SupersededBy, err)
	}
	if _, err := s.Supersede("home-old", "brain:home/home-new", svReason, svOrigin); err == nil || !strings.Contains(err.Error(), "different levels") {
		t.Fatalf("bare name at the project default level against a home note: %v", err)
	}
}
