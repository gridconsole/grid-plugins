package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func fixture(t *testing.T) (Store, string) {
	t.Helper()
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	return Store{Resolver: where.Resolver{GridHome: filepath.Join(root, "home", ".grid"), Home: filepath.Join(root, "home"), Ceiling: root, TempRoots: []string{filepath.Join(root, "tmp")}}}, root
}
func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestAdd(t *testing.T) {
	for _, tc := range []struct {
		name           string
		repos, remotes int
	}{{"folder", 0, 0}, {"two-remotes", 1, 2}, {"two-repos", 2, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := fixture(t)
			folder := mkdir(t, filepath.Join(root, "work", "suite"))
			for i := 0; i < tc.repos; i++ {
				repo := folder
				if tc.repos > 1 {
					repo = mkdir(t, filepath.Join(folder, []string{"app", "libs/api"}[i]))
				}
				testgit.Init(t, repo, "https://example.com/org/"+[]string{"app", "api"}[i]+".git", false)
				if tc.remotes == 2 {
					testgit.Run(t, repo, "remote", "add", "upstream", "git@example.com:upstream/app.git")
				}
			}
			got, err := s.Add(folder, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Repos) != tc.repos*tc.remotes || len(got.Folders) != 1 || got.Folders[0].Path != folder {
				t.Fatalf("%+v", got)
			}
			again, err := s.Add(folder, "")
			if err != nil || again.ID != got.ID || len(again.Folders) != 1 {
				t.Fatalf("second add: %+v %v", again, err)
			}
			for _, r := range got.Repos {
				if strings.Contains(r.Remote, "git@") || strings.HasSuffix(r.Remote, ".git") {
					t.Errorf("unnormalized remote: %s", r.Remote)
				}
			}
			if tc.remotes == 2 && (got.Repos[0].Remote != "example.com/org/app" || got.Repos[1].Remote != "example.com/upstream/app") {
				t.Fatalf("lost remote: %+v", got.Repos)
			}
		})
	}
}

func TestRelinkAndRemoveKeepData(t *testing.T) {
	s, root := fixture(t)
	folder := mkdir(t, filepath.Join(root, "old"))
	testgit.Init(t, folder, "https://example.com/org/app.git", false)
	got, err := s.Add(folder, "example")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.Resolver.GridHome, "projects", got.Name, "project.md")
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\nProject prose.\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	recs, err := project.Load(s.Resolver.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	rec := recs[0]
	rec.Folders = append(rec.Folders, project.Folder{Machine: "other", Path: "/example/other"})
	rec.Keys = append(rec.Keys, project.Key{Key: "folder:/example/other", Machine: "other"})
	if err := s.write(rec); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(filepath.Dir(file), "notes", "keep.md")
	mkdir(t, filepath.Dir(note))
	if err := os.WriteFile(note, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "moved")
	if err := os.Rename(folder, moved); err != nil {
		t.Fatal(err)
	}
	linked, err := s.Relink(got.ID, moved)
	if err != nil {
		t.Fatal(err)
	}
	if linked.ID != got.ID || len(linked.Folders) != 2 || linked.Folders[1].Path != moved {
		t.Fatalf("%+v", linked)
	}
	s.Resolver.Machine = gridhome.Machine(s.Resolver.GridHome)
	res, err := s.Resolver.Resolve(where.Query{Cwd: moved})
	if err != nil || res.ID != got.ID {
		t.Fatalf("resolver: %+v %v", res, err)
	}
	for _, k := range linked.Keys {
		if strings.Contains(k.Key, folder) {
			t.Fatalf("stale local key: %+v", k)
		}
	}
	removed, err := s.Remove(got.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Folders) != 1 || removed.Folders[0].Machine != "other" {
		t.Fatalf("%+v", removed)
	}
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), "Project prose.") {
		t.Fatalf("lost body: %s %v", data, err)
	}
	if _, err := os.Stat(note); err != nil {
		t.Fatal(err)
	}
}

func TestScanReadOnly(t *testing.T) {
	s, root := fixture(t)
	discovery := mkdir(t, filepath.Join(root, "work"))
	suite := mkdir(t, filepath.Join(discovery, "suite"))
	a := mkdir(t, filepath.Join(suite, "a"))
	b := mkdir(t, filepath.Join(suite, "b"))
	testgit.Init(t, a, "https://example.com/org/a.git", false)
	testgit.Init(t, b, "https://example.com/org/b.git", false)
	mkdir(t, filepath.Join(discovery, "ideas"))
	mkdir(t, filepath.Join(discovery, ".hidden"))
	got, err := s.Scan(discovery)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Rule != where.RuleMultiRepo {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(s.Resolver.GridHome); !os.IsNotExist(err) {
		t.Fatalf("scan wrote Grid home: %v", err)
	}
	for _, p := range got {
		if _, err := s.Add(p.Path, ""); err != nil {
			t.Fatal(err)
		}
	}
	s.Resolver.Machine = gridhome.Machine(s.Resolver.GridHome)
	got, err = s.Scan(discovery)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if !p.Registered {
			t.Fatalf("not registered: %+v", p)
		}
	}
}

func TestRefusals(t *testing.T) {
	for _, kind := range []string{"bad-name", "overlap", "remote-conflict", "symlink", "missing"} {
		t.Run(kind, func(t *testing.T) {
			s, root := fixture(t)
			folder := mkdir(t, filepath.Join(root, "suite"))
			var err error
			switch kind {
			case "bad-name":
				_, err = s.Add(folder, "../escape")
			case "missing":
				_, err = s.Add(filepath.Join(root, "absent"), "")
			case "symlink":
				mkdir(t, filepath.Join(s.Resolver.GridHome, "projects"))
				if e := os.Symlink(mkdir(t, filepath.Join(root, "outside")), filepath.Join(s.Resolver.GridHome, "projects", "linked")); e != nil {
					t.Fatal(e)
				}
				_, err = s.Add(folder, "")
			case "overlap":
				child := mkdir(t, filepath.Join(folder, "app"))
				testgit.Init(t, child, "https://example.com/org/app.git", false)
				if _, e := s.Add(child, ""); e != nil {
					t.Fatal(e)
				}
				_, err = s.Add(folder, "")
			case "remote-conflict":
				a := mkdir(t, filepath.Join(root, "a"))
				b := mkdir(t, filepath.Join(root, "b"))
				testgit.Init(t, a, "https://example.com/org/a.git", false)
				testgit.Init(t, b, "https://example.com/org/b.git", false)
				if _, e := s.Add(a, ""); e != nil {
					t.Fatal(e)
				}
				if _, e := s.Add(b, ""); e != nil {
					t.Fatal(e)
				}
				testgit.Run(t, a, "remote", "add", "second", "https://example.com/org/b.git")
				_, err = s.Add(a, "")
			}
			if err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

func TestCollisionAndPersonal(t *testing.T) {
	s, root := fixture(t)
	for i, base := range []string{"one", "two"} {
		p := mkdir(t, filepath.Join(root, base, "same"))
		got, err := s.Add(p, "")
		if err != nil {
			t.Fatal(err)
		}
		want := "same"
		if i == 1 {
			want = "same-2"
		}
		if got.Name != want {
			t.Fatalf("%+v", got)
		}
	}
	mkdir(t, s.Resolver.Home)
	got, err := s.Add(s.Resolver.Home, "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Add(s.Resolver.Home, "")
	if err != nil || again.ID != got.ID || got.Name != where.Personal || len(got.Folders) != 0 {
		t.Fatalf("%+v %+v %v", got, again, err)
	}
}

func TestCloneAndChangedRemoteKeepIdentity(t *testing.T) {
	s, root := fixture(t)
	first := mkdir(t, filepath.Join(root, "one"))
	testgit.Init(t, first, "https://example.com/org/app.git", false)
	original, err := s.Add(first, "")
	if err != nil {
		t.Fatal(err)
	}
	second := mkdir(t, filepath.Join(root, "two"))
	testgit.Init(t, second, "git@example.com:org/app.git", false)
	clone, err := s.Add(second, "")
	if err != nil || clone.ID != original.ID || len(clone.Folders) != 2 {
		t.Fatalf("%+v %v", clone, err)
	}
	testgit.Run(t, second, "remote", "set-url", "origin", "https://example.com/new/app.git")
	changed, err := s.Add(second, "")
	if err != nil || changed.ID != original.ID {
		t.Fatalf("%+v %v", changed, err)
	}
	remotes := map[string]bool{}
	for _, k := range changed.Keys {
		remotes[k.Key] = true
	}
	if !remotes["remote:example.com/org/app"] || !remotes["remote:example.com/new/app"] {
		t.Fatalf("lost identity keys: %+v", changed.Keys)
	}
}

func TestScanIgnoresEnclosingRepo(t *testing.T) {
	s, root := fixture(t)
	testgit.Init(t, root, "https://example.com/org/outer.git", false)
	discovery := mkdir(t, filepath.Join(root, "work"))
	mkdir(t, filepath.Join(discovery, "child"))
	got, err := s.Scan(discovery)
	if err != nil || len(got) != 0 {
		t.Fatalf("proposed enclosing checkout: %+v %v", got, err)
	}
}
