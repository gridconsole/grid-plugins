package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestPlanGolden(t *testing.T) {
	from, home := fixture(t)
	before := view(t, from)
	p := scan(t, from, home)
	var b bytes.Buffer
	if err := p.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "migrate-plan", strings.NewReplacer(from, "<SOURCE>", home, "<HOME>").Replace(b.String()))
	if !sameView(before, view(t, from)) {
		t.Fatal("dry run changed the source")
	}
	absent(t, home)
	if got := siblings(t, home); !reflect.DeepEqual(got, []string{"old-grid"}) {
		t.Fatalf("dry run left files beside the home: %v", got)
	}
	if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
		t.Fatalf("unexpected blockers: %v %v", p.Conflicts, p.Unmapped)
	}
	again := scan(t, from, home)
	if !reflect.DeepEqual(p.result, again.result) {
		t.Fatal("plans must have stable ids and bytes")
	}
}

func TestBlockedPlanGolden(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(from, "config.json"), `{"current":"alpha","workspaces":[]}`)
	put(t, filepath.Join(home, "sessions", "session-a"), "unrelated pointer\n")
	p := scan(t, from, home)
	var b bytes.Buffer
	if err := p.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "migrate-blocked-plan", strings.NewReplacer(from, "<SOURCE>", home, "<HOME>").Replace(b.String()))
	if len(p.Conflicts) == 0 || len(p.Unmapped) == 0 {
		t.Fatal("missing blockers")
	}
}

// Session pointers written through a symlinked path (macOS /var, /tmp) map like the
// resolved ones; the source and the home are spelled through the link here too.
func TestSymlinkedSourceMapsPointers(t *testing.T) {
	real := gridhome.Real(t.TempDir())
	alias := filepath.Join(gridhome.Real(t.TempDir()), "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skip(err)
	}
	copyFixture(t, filepath.Join(alias, "old-grid"))
	from, home := filepath.Join(alias, "old-grid"), filepath.Join(alias, ".grid")
	p := scan(t, from, home)
	if len(p.Unmapped) != 0 || len(p.Conflicts) != 0 {
		t.Fatalf("blocked through a symlinked path: %v %v", p.Unmapped, p.Conflicts)
	}
	if p.From != filepath.Join(real, "old-grid") {
		t.Fatalf("source was not resolved: %s", p.From)
	}
	want := filepath.Join(real, ".grid", "projects/widget/tasks/T-07-first") + "\n"
	if got := string(p.result["sessions/session-a"].Data); got != want {
		t.Fatalf("pointer = %q, want %q", got, want)
	}
}

func parseProject(t *testing.T, p *Plan, name string) project.Record {
	t.Helper()
	rec, err := project.Parse(name, string(p.result["projects/"+name+"/project.md"].Data))
	if err != nil {
		t.Fatalf("project.md of %s is not a valid record: %v", name, err)
	}
	return rec
}

func keyNames(rec project.Record) []string {
	var out []string
	for _, k := range rec.Keys {
		out = append(out, strings.SplitN(k.Key, ":", 2)[0])
	}
	return out
}

func TestIdentityFromLocalGit(t *testing.T) {
	testgit.Isolate(t)
	for _, remote := range []string{"https://example.com/team/widget.git", ""} {
		t.Run(remote, func(t *testing.T) {
			root := gridhome.Real(t.TempDir())
			repo := filepath.Join(root, "repo")
			if err := os.Mkdir(repo, 0700); err != nil {
				t.Fatal(err)
			}
			testgit.Init(t, repo, remote, true)
			from := filepath.Join(root, "old")
			put(t, filepath.Join(from, "projects", "widget", "project.md"), "---\nfolder: "+repo+"\n---\nbody stays\n")
			p := scan(t, from, filepath.Join(root, "new"))
			rec := parseProject(t, p, "widget")
			if !ulidRE.MatchString(rec.ID) {
				t.Fatalf("project id is not a ULID: %s", rec.ID)
			}
			if len(rec.Folders) != 1 || rec.Folders[0].Path != repo || rec.Folders[0].Machine == "" {
				t.Fatalf("folders: %+v", rec.Folders)
			}
			if len(rec.Repos) != 1 || rec.Repos[0].Path != "." {
				t.Fatalf("repos: %+v", rec.Repos)
			}
			// root: is the key of a repo with no remote only; a fork would share it.
			want := []string{"common", "remote"}
			if remote == "" {
				want = []string{"common", "root"}
			}
			if got := keyNames(rec); !reflect.DeepEqual(got, want) {
				t.Fatalf("keys %v, want %v", got, want)
			}
			for _, k := range rec.Keys {
				if (k.Key[:6] == "common") != (k.Machine != "") {
					t.Fatalf("only the common: key is machine bound: %+v", k)
				}
			}
			if remote != "" && rec.Repos[0].Remote != "example.com/team/widget" {
				t.Fatalf("repo remote: %q", rec.Repos[0].Remote)
			}
			if !strings.HasSuffix(string(p.result["projects/widget/project.md"].Data), "---\nbody stays\n") {
				t.Fatal("the body of project.md changed")
			}
		})
	}
}

// Two clones with different remotes share their root commit (forks do); neither is a
// duplicate of the other, as the root: key is left off a repo that has a remote.
func TestForkIsNotADuplicate(t *testing.T) {
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	from := filepath.Join(root, "old")
	for name, remote := range map[string]string{"upstream": "https://example.com/team/app.git", "fork": "https://example.com/me/app.git"} {
		repo := filepath.Join(root, "work", name)
		if err := os.MkdirAll(repo, 0700); err != nil {
			t.Fatal(err)
		}
		testgit.Init(t, repo, remote, true)
		put(t, filepath.Join(from, "projects", name, "project.md"), "---\nfolder: "+repo+"\n---\n")
	}
	p := scan(t, from, filepath.Join(root, "new"))
	if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
		t.Fatalf("a fork was flagged: %v %v", p.Conflicts, p.Unmapped)
	}
}

func TestSameRootWithoutRemoteIsAConflict(t *testing.T) {
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	from := filepath.Join(root, "old")
	for _, name := range []string{"one", "two"} {
		repo := filepath.Join(root, "work", name)
		if err := os.MkdirAll(repo, 0700); err != nil {
			t.Fatal(err)
		}
		testgit.Init(t, repo, "", true)
		put(t, filepath.Join(from, "projects", name, "project.md"), "---\nfolder: "+repo+"\n---\n")
	}
	p := scan(t, from, filepath.Join(root, "new"))
	if len(p.Conflicts) != 1 || !strings.Contains(p.Conflicts[0], "identity key also used by") {
		t.Fatalf("conflicts: %v", p.Conflicts)
	}
}

func TestMultiRepoIdentity(t *testing.T) {
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	folder := filepath.Join(root, "suite")
	for _, name := range []string{"one", "libs/two"} {
		repo := filepath.Join(folder, name)
		if err := os.MkdirAll(repo, 0700); err != nil {
			t.Fatal(err)
		}
		testgit.Init(t, repo, "https://example.com/team/"+filepath.Base(name)+".git", false)
	}
	from := filepath.Join(root, "old")
	put(t, filepath.Join(from, "projects/suite/project.md"), "---\nfolder: "+folder+"\n---\n")
	p := scan(t, from, filepath.Join(root, "new"))
	rec := parseProject(t, p, "suite")
	var repos, remotes, commons []string
	for _, r := range rec.Repos {
		repos = append(repos, r.Path+"="+r.Remote)
	}
	for _, k := range rec.Keys {
		switch strings.SplitN(k.Key, ":", 2)[0] {
		case "remote":
			remotes = append(remotes, k.Key)
		case "common":
			commons = append(commons, k.Key)
		default:
			t.Fatalf("unexpected key: %+v", k)
		}
	}
	if !reflect.DeepEqual(repos, []string{"libs/two=example.com/team/two", "one=example.com/team/one"}) {
		t.Fatalf("repos: %v", repos)
	}
	if !reflect.DeepEqual(remotes, []string{"remote:example.com/team/two", "remote:example.com/team/one"}) || len(commons) != 2 {
		t.Fatalf("keys: %+v", rec.Keys)
	}
}

func TestProjectRecordKeepsBodyAndOtherFields(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	from := filepath.Join(root, "old")
	body := "# Title\r\n\r\ntext with --- inside\r\n"
	put(t, filepath.Join(from, "projects/widget/project.md"), "---\nfolder: /example/work/widget\nowner: Example\n---\n"+body)
	p := scan(t, from, filepath.Join(root, "new"))
	data := string(p.result["projects/widget/project.md"].Data)
	if !strings.HasSuffix(data, "---\n"+body) || !strings.Contains(data, "owner: Example\n") {
		t.Fatalf("record: %q", data)
	}
	for _, old := range []string{"\nfolder:", "\nrepo:", "\nkey:"} {
		if strings.Contains(data, old) {
			t.Fatalf("old %q line left in %q", old, data)
		}
	}
}

func TestVersionGuards(t *testing.T) {
	for _, v := range []string{"2\n", "garbage\n", "-1\n"} {
		t.Run(strings.TrimSpace(v), func(t *testing.T) {
			from, home := fixture(t)
			put(t, filepath.Join(home, "VERSION"), v)
			if _, err := Scan(from, home); err == nil {
				t.Fatal("accepted unsupported version")
			}
		})
	}
	from, home := fixture(t)
	put(t, filepath.Join(home, "VERSION"), "1\n")
	p := scan(t, from, home)
	if !p.Current || len(p.Actions) != 0 {
		t.Fatal("current home should be a no-op")
	}
}

func TestUnsafeAndUnknownInputs(t *testing.T) {
	for _, kind := range []string{"top-level-symlink", "missing-project", "bad-task", "bad-session", "git-worktree", "unknown-pointer"} {
		t.Run(kind, func(t *testing.T) {
			from, home := fixture(t)
			switch kind {
			case "top-level-symlink":
				// An unknown entry is reported even when it is a link; the link is not followed.
				if err := os.Symlink(t.TempDir(), filepath.Join(from, "link")); err != nil {
					t.Skip(err)
				}
			case "missing-project":
				if err := os.Remove(filepath.Join(from, "alpha/projects/widget/project.md")); err != nil {
					t.Fatal(err)
				}
			case "bad-task":
				put(t, filepath.Join(from, "alpha/projects/widget/tasks/T-07-first/task.md"), "not frontmatter")
			case "bad-session":
				put(t, filepath.Join(from, "alpha/projects/widget/tasks/T-07-first/sessions/session-a.json"), "{")
			case "git-worktree":
				put(t, filepath.Join(from, "alpha/worktrees/widget/T-07/worker-a/.git"), "gitdir: /example/repo/.git/worktrees/worker-a\n")
			case "unknown-pointer":
				put(t, filepath.Join(from, "alpha/sessions/session-a"), "/example/missing/task\n")
			}
			p, err := Scan(from, home)
			if err == nil && len(p.Unmapped) == 0 {
				t.Fatal("unsafe input accepted")
			}
		})
	}
}

func TestSymlinksInsideAProjectAreRecorded(t *testing.T) {
	from, home := fixture(t)
	outside := t.TempDir()
	put(t, filepath.Join(outside, "secret.txt"), "not part of the home\n")
	know := filepath.Join(from, "alpha/projects/widget/knowledge")
	for name, target := range map[string]string{"outside-dir": outside, "outside-file": filepath.Join(outside, "secret.txt"), "dangling": "does-not-exist", "relative": "research/sample.md"} {
		if err := os.Symlink(target, filepath.Join(know, name)); err != nil {
			t.Skip(err)
		}
	}
	p := scan(t, from, home)
	if len(p.Unmapped) != 0 || len(p.Conflicts) != 0 {
		t.Fatalf("a symlink blocked the plan: %v %v", p.Unmapped, p.Conflicts)
	}
	found := false
	for _, n := range p.Notes {
		found = found || strings.Contains(n, "absolute symlinks")
	}
	if !found {
		t.Fatalf("absolute links are not mentioned: %v", p.Notes)
	}
	res, err := p.Apply()
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{filepath.Join(home, "projects/widget/knowledge"), filepath.Join(res.Backup, "source/alpha/projects/widget/knowledge")} {
		for name, target := range map[string]string{"outside-dir": outside, "outside-file": filepath.Join(outside, "secret.txt"), "dangling": "does-not-exist", "relative": "research/sample.md"} {
			got, err := os.Readlink(filepath.Join(root, name))
			if err != nil || got != target {
				t.Fatalf("%s/%s: link %q (%v), want %q", root, name, got, err, target)
			}
		}
	}
	for _, root := range []string{home, res.Backup} {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if d != nil && d.Name() == "secret.txt" && d.Type().IsRegular() {
				t.Errorf("the target of a link was copied: %s", path)
			}
			return nil
		})
	}
}

func TestDuplicateIdentityIsAConflict(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(from, "beta/projects/widget/project.md"), "---\nfolder: /example/work/widget\n---\n")
	p := scan(t, from, home)
	if len(p.Conflicts) != 1 || !strings.Contains(p.Conflicts[0], "identity key also used") {
		t.Fatalf("missing ambiguous identity conflict: %v", p.Conflicts)
	}
	if _, err := p.Apply(); err == nil {
		t.Fatal("ambiguous projects were applied")
	}
}

func TestRootSessionsAndDestinationNameCollision(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(from, "sessions/root-session"), filepath.Join(from, "beta/projects/notes/tasks/T-01-note")+"\n")
	existing := project.Record{Name: "widget", ID: "01K00000000000000000000009", Folders: []project.Folder{{Machine: "01OTHERMACHINE", Path: "/example/work/existing"}}}
	if err := project.Write(home, existing); err != nil {
		t.Fatal(err)
	}
	p := scan(t, from, home)
	if len(p.Unmapped) > 0 || len(p.Conflicts) > 0 {
		t.Fatalf("blocked: %v %v", p.Unmapped, p.Conflicts)
	}
	if _, ok := p.result["projects/widget-2/tasks/T-07-first/task.md"]; !ok {
		t.Fatal("existing project name was overwritten")
	}
	if _, ok := p.result["projects/widget-3/tasks/T-12-second/task.md"]; !ok {
		t.Fatal("second collision not suffixed")
	}
	if !strings.Contains(string(p.result["sessions/root-session"].Data), home) {
		t.Fatal("root session not mapped")
	}
}

func TestPartialSelectionCannotMarkWholeHomeMigrated(t *testing.T) {
	from, _ := fixture(t)
	home := from
	from = filepath.Join(from, "alpha")
	p := scan(t, from, home)
	if len(p.Conflicts) == 0 || !strings.Contains(p.Conflicts[0], "outside --from") {
		t.Fatalf("missing omitted-home conflict: %v", p.Conflicts)
	}
	before := view(t, home)
	if _, err := p.Apply(); err == nil {
		t.Fatal("partial migration applied")
	}
	if !sameView(before, view(t, home)) {
		t.Fatal("refusal changed home")
	}
}
