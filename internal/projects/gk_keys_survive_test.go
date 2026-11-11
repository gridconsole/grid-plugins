package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/projectmd"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// project.md is a file people and agents edit, so a writer of the projects store
// (Relink, Remove, Add, Apply: all go through Store.write) keeps every front-matter
// key it does not own, with its value, in the order the file had them, and keeps
// the body byte for byte. The keys it owns are id, folders, repos and keys.

const (
	gkLands = "lands: reviewed\n"
	gkOwner = "owner: someone\n"
	gkNote  = "note: \"a: b # c\"\n"     // a quoted string
	gkTags  = "tags:\n- alpha\n- beta\n" // a list, in the layout the writers give a list
	// gkBody has a line "---" in it, and no newline at the end.
	gkBody = "# Alpha\n\nfirst line\n---\nafter the rule\n\n```\n---\n```\nlast line, no newline"
)

func gkPath(s Store, name string) string {
	return filepath.Join(s.Resolver.GridHome, "projects", name, "project.md")
}

// gkBlocks splits front matter into its top-level keys: the text of each key, by key name.
func gkBlocks(front string) map[string]string {
	out := map[string]string{}
	name := ""
	for _, line := range strings.SplitAfter(front, "\n") {
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '-' {
			name, _, _ = strings.Cut(line, ":")
		}
		out[name] += line
	}
	return out
}

// gkFront is the text between the two --- lines of a project.md that has no body.
func gkFront(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.HasPrefix(text, "---\n") || !strings.HasSuffix(text, "---\n") {
		t.Fatalf("not front matter alone:\n%q", text)
	}
	return strings.TrimSuffix(strings.TrimPrefix(text, "---\n"), "---\n")
}

// gkCompose writes the keys in order: a key the store owns from owned, any other from extras.
func gkCompose(t *testing.T, order []string, owned, extras map[string]string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\n")
	for _, name := range order {
		text, ok := owned[name]
		if !ok {
			text, ok = extras[name]
		}
		if !ok {
			t.Fatalf("no text for key %q", name)
		}
		b.WriteString(text)
	}
	b.WriteString("---\n")
	return b.String()
}

// gkSetup makes the project alpha through Add, then rewrites its project.md with
// the keys of the table below and a body: id, lands, owner, folders, note, keys, tags.
func gkSetup(t *testing.T) (s Store, root, folder string) {
	t.Helper()
	s, root = fixture(t)
	folder = mkdir(t, filepath.Join(root, "work", "alpha"))
	if _, err := s.Add(folder, "alpha"); err != nil {
		t.Fatal(err)
	}
	blocks := gkBlocks(gkFront(t, gkPath(s, "alpha")))
	if len(blocks) != 3 || blocks["id"] == "" || blocks["folders"] == "" || blocks["keys"] == "" {
		t.Fatalf("Add wrote %v; want id, folders and keys", blocks)
	}
	text := "---\n" + blocks["id"] + gkLands + gkOwner + blocks["folders"] + gkNote + blocks["keys"] + gkTags + "---\n" + gkBody
	if err := os.WriteFile(gkPath(s, "alpha"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return s, root, folder
}

// TestKeysSurviveRelinkAfterKeySet: a key set with projectmd.Key is still there
// after a relink, a remove and an add, and the keys that were there before are too.
func TestKeysSurviveRelinkAfterKeySet(t *testing.T) {
	s, root, folder := gkSetup(t)
	value := "value-01"
	if _, err := projectmd.Key(s.Resolver, "alpha", "k01", &value); err != nil {
		t.Fatal(err)
	}
	read := func(key, want string) {
		t.Helper()
		got, err := projectmd.Key(s.Resolver, "alpha", key, nil)
		if err != nil || got.Value != want {
			t.Fatalf("%s reads %#v, %v; want %q", key, got.Value, err, want)
		}
	}
	read("k01", value)
	steps := []struct {
		name string
		run  func() error
	}{
		{"relink", func() error { _, err := s.Relink("alpha", mkdir(t, filepath.Join(root, "work", "alpha2"))); return err }},
		{"remove", func() error { _, err := s.Remove("alpha"); return err }},
		{"add", func() error { _, err := s.Add(folder, "alpha"); return err }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		read("k01", value)
		read("lands", "reviewed")
		read("owner", "someone")
	}
}

// TestKeysSurviveWriters: one case per writer of Store.write. Each starts from the
// same file, id lands owner folders note keys tags then a body, and must end with
// the keys of Record as Format writes them and every other key where it was.
func TestKeysSurviveWriters(t *testing.T) {
	extras := map[string]string{"lands": gkLands, "owner": gkOwner, "note": gkNote, "tags": gkTags}
	for _, tc := range []struct {
		name  string
		order []string
		run   func(t *testing.T, s Store, root, folder string)
	}{
		{"relink", []string{"id", "lands", "owner", "folders", "note", "keys", "tags"}, func(t *testing.T, s Store, root, folder string) {
			e, err := s.Relink("alpha", mkdir(t, filepath.Join(root, "work", "alpha2")))
			if err != nil || len(e.Folders) != 1 || !strings.HasSuffix(e.Folders[0].Path, "alpha2") {
				t.Fatalf("Relink = %+v, %v", e, err)
			}
		}},
		// Remove drops folders and keys: the keys that stay keep their order.
		{"remove", []string{"id", "lands", "owner", "note", "tags"}, func(t *testing.T, s Store, root, folder string) {
			if e, err := s.Remove("alpha"); err != nil || len(e.Folders) != 0 {
				t.Fatalf("Remove = %+v, %v", e, err)
			}
		}},
		// The folder now holds a git repo, so Add writes repos, which the file did not have:
		// it goes after folders, the key of Record before it.
		{"add", []string{"id", "lands", "owner", "folders", "repos", "note", "keys", "tags"}, func(t *testing.T, s Store, root, folder string) {
			testgit.Init(t, folder, "https://example.com/org/alpha.git", false)
			if e, err := s.Add(folder, ""); err != nil || len(e.Repos) != 1 {
				t.Fatalf("Add = %+v, %v", e, err)
			}
		}},
		{"apply", []string{"id", "lands", "owner", "folders", "repos", "note", "keys", "tags"}, func(t *testing.T, s Store, root, folder string) {
			testgit.Init(t, folder, "https://example.com/org/alpha.git", false)
			out, err := s.Apply([]Proposal{{Path: folder}})
			if err != nil || len(out) != 1 || out[0].Error != "" || !out[0].Registered {
				t.Fatalf("Apply = %+v, %v", out, err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root, folder := gkSetup(t)
			tc.run(t, s, root, folder)
			b, err := os.ReadFile(gkPath(s, "alpha"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(b)
			// The keys of Record, as Format writes them for the record the file now holds.
			rec, err := project.Parse("alpha", text)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := project.Format(rec)
			if err != nil {
				t.Fatal(err)
			}
			owned := gkBlocks(strings.TrimSuffix(strings.TrimPrefix(string(canonical), "---\n"), "---\n"))
			if want := gkCompose(t, tc.order, owned, extras) + gkBody; text != want {
				t.Fatalf("project.md after %s:\n%q\nwant\n%q", tc.name, text, want)
			}
		})
	}
}

// gkGolden is the file main writes for this record, byte for byte.
const gkGolden = `---
id: 01JGOLDEN0000000000000000
folders:
- machine: m1
  path: /work/gold
repos:
- path: .
  remote: example.com/org/gold
keys:
- key: folder:/work/gold
  machine: m1
- key: remote:example.com/org/gold
---
`

// TestNoExtraKeysWrittenAsMainWrites: a file with no keys besides those of Record
// is written as it always was, whatever it held: nothing there, the same keys in
// the same order, or the same keys in another order.
func TestNoExtraKeysWrittenAsMainWrites(t *testing.T) {
	rec := project.Record{
		Name: "gold", ID: "01JGOLDEN0000000000000000",
		Folders: []project.Folder{{Machine: "m1", Path: "/work/gold"}},
		Repos:   []project.Repo{{Path: ".", Remote: "example.com/org/gold"}},
		Keys:    []project.Key{{Key: "folder:/work/gold", Machine: "m1"}, {Key: "remote:example.com/org/gold"}},
	}
	const body = "# Gold\n\ntext\n---\nmore\n"
	for _, tc := range []struct{ name, old, want string }{
		{"absent", "", gkGolden},
		{"same", gkGolden + body, gkGolden + body},
		{"another order", "---\nkeys:\n  - key: remote:example.com/org/gold\nrepos: []\nfolders:\n  - machine: m2\n    path: /elsewhere\nid: 01JGOLDEN0000000000000000\n---\n" + body, gkGolden + body},
		{"empty front matter", "---\n---\n" + body, gkGolden + body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fixture(t)
			mkdir(t, s.Resolver.GridHome)
			if tc.old != "" {
				refWrite(t, gkPath(s, "gold"), tc.old)
			}
			if err := s.write(rec); err != nil {
				t.Fatal(err)
			}
			if got := refRead(t, gkPath(s, "gold")); got != tc.want {
				t.Fatalf("project.md:\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}
