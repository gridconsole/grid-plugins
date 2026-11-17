package brain

import (
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/mdfile"
)

// pbFmPerson stands in for a person editing a filed note in Obsidian: it adds keys
// Grid does not know (lists, a comment), CRLF line endings throughout and the
// given lines at the end of the body, writes the file back and returns its bytes.
func pbFmPerson(t *testing.T, s Store, topic, name, tail string) []byte {
	t.Helper()
	path := keRnPath(s, "knowledge/"+topic+"/"+name+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(b), "---\n", "---\n# my own comment\ntags: [one, two]\naliases:\n  - the "+name+"\ncssclasses:\n  - wide\nrating: 5 # stars\n", 1)
	text = strings.ReplaceAll(text+tail, "\n", "\r\n")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	// Grid renames and rewrites only a note whose newest stored version is the
	// file, so the person's edit is stored as a version, as a saved edit is.
	root, err := s.scope("project", true)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, _, err := recordVersion(root, topic, name, []byte(text)); err != nil {
		t.Fatal(err)
	}
	return []byte(text)
}

// pbFmFront is the front matter of a note read as plain YAML, the way Obsidian's
// Properties editor reads it.
func pbFmFront(t *testing.T, data []byte) map[string]any {
	t.Helper()
	front, _, err := mdfile.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := mdfile.Load(front, &m, false); err != nil {
		t.Fatal(err)
	}
	return m
}

func pbFmLists(t *testing.T, label string, data []byte) {
	t.Helper()
	m := pbFmFront(t, data)
	for _, k := range []string{"tags", "aliases", "cssclasses"} {
		if _, ok := m[k].([]any); !ok {
			t.Errorf("%s: %s is %T, want a list", label, k, m[k])
		}
	}
}

// Rows 1, 2, 3: a rename of a note a person edited changes only the edited key,
// keeps the person's keys, comment, lists and line endings, and rewrites the links
// that point at it in another person-edited note with their alias and heading.
func TestPbFmRenameKeepsWhatAPersonWrote(t *testing.T) {
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "a"))
	keRnAccept(t, s, keRnNote("decisions", "carrier"))
	orig := pbFmPerson(t, s, "decisions", "a", "")
	carrier := pbFmPerson(t, s, "decisions", "carrier",
		"\nSee [[a|the first]] and [[a#why]] and [[decisions/a#why|alias]] and ![[a]].\n")

	if _, err := s.Rename("project", "decisions", "a", "b", keRnRev(t, s, "decisions", "a"), keRnAna, ""); err != nil {
		t.Fatal(err)
	}

	got := keRnRead(t, s, "knowledge/decisions/b.md")
	want := strings.Replace(string(orig), "name: a\r\n", "name: b\r\n", 1)
	if string(got) != want {
		t.Errorf("rename changed more than the name key:\n got %q\nwant %q", got, want)
	}
	pbFmLists(t, "renamed note", got)

	gotCarrier := keRnRead(t, s, "knowledge/decisions/carrier.md")
	wantCarrier := strings.NewReplacer("[[a|the first]]", "[[b|the first]]", "[[a#why]]", "[[b#why]]",
		"[[decisions/a#why|alias]]", "[[decisions/b#why|alias]]", "![[a]]", "![[b]]").Replace(string(carrier))
	if string(gotCarrier) != wantCarrier {
		t.Errorf("links were not rewritten alone:\n got %q\nwant %q", gotCarrier, wantCarrier)
	}
	pbFmLists(t, "carrier", gotCarrier)
}

// Row 1: marking a note superseded changes only its superseded_by key.
func TestPbFmSupersedeChangesOnlyItsKey(t *testing.T) {
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "old"))
	keRnAccept(t, s, keRnNote("decisions", "new"))
	orig := pbFmPerson(t, s, "decisions", "old", "\nKept body.\n")

	if _, err := s.Supersede("brain:project/old", "brain:project/new", "moved on", keRnAna); err != nil {
		t.Fatal(err)
	}
	got := string(keRnRead(t, s, "knowledge/decisions/old.md"))
	want := strings.Replace(string(orig), "superseded_by: ''\r\n", "superseded_by: brain:project/new\r\n", 1)
	if want == string(orig) || got != want {
		t.Errorf("supersede changed more than superseded_by:\n got %q\nwant %q", got, want)
	}
	pbFmLists(t, "superseded note", []byte(got))
}

// Row 5: a person note with no front matter that links to a renamed note gets its
// link rewritten and nothing else: no front matter is added, and a read or the
// rename of the note it links to leaves its other bytes alone.
func TestPbFmRenameLeavesANoteWithoutFrontMatter(t *testing.T) {
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "a"))
	person := "Just my own text.\r\n\r\nSee [[a|the first]].\r\n"
	path := keRnPath(s, "knowledge/decisions/f1.md")
	if err := os.WriteFile(path, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Show("brain:project/f1")
	if b, _ := os.ReadFile(path); string(b) != person {
		t.Fatalf("a read changed the note: %q", b)
	}
	if _, err := s.Rename("project", "decisions", "a", "b", keRnRev(t, s, "decisions", "a"), keRnAna, ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(person, "[[a|the first]]", "[[b|the first]]", 1); string(b) != want {
		t.Errorf("rename changed more than the link in a note without front matter: %q, want %q", b, want)
	}
}

// Row 4: a note Grid files names its author in a flat source and has no origin
// key and no nested value, and its links are a ## Links section that reads back.
func TestPbFmFiledNoteIsFlat(t *testing.T) {
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "a"))
	linked := keRnNote("decisions", "b", keRnLink("brain:project/a"))
	linked.Source = "fixture"
	keRnAccept(t, s, linked)

	for name, wantSource := range map[string]string{"a": "job:T-01/J-01", "b": "T-01/J-01"} {
		b := keRnRead(t, s, "knowledge/decisions/"+name+".md")
		m := pbFmFront(t, b)
		if _, ok := m["origin"]; ok {
			t.Errorf("%s: front matter has an origin key:\n%s", name, b)
		}
		if src, _ := m["source"].(string); !strings.Contains(src, wantSource) {
			t.Errorf("%s: source %q does not name %q", name, m["source"], wantSource)
		}
		for k, v := range m {
			switch v.(type) {
			case map[string]any, []any:
				t.Errorf("%s: %s is %T, Properties needs a flat value", name, k, v)
			}
		}
	}
	b := keRnRead(t, s, "knowledge/decisions/b.md")
	if !strings.Contains(string(b), "\n## Links\n") || strings.Contains(string(b), "\nlinks:") {
		t.Errorf("links are not a Links section:\n%s", b)
	}
	n, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Links) != 1 || n.Links[0].To != "brain:project/a" || n.Links[0].Rel != "related" {
		t.Errorf("links did not read back: %+v", n.Links)
	}
}
