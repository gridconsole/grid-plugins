package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	from := filepath.Join(root, "old-grid")
	home := filepath.Join(root, ".grid")
	files, err := snapshot("../../testdata/migrate/old-grid")
	if err != nil {
		t.Fatal(err)
	}
	for k, e := range files {
		e.Data = []byte(strings.ReplaceAll(string(e.Data), "/example/old-grid", from))
		files[k] = e
	}
	if err := writeTree(from, files); err != nil {
		t.Fatal(err)
	}
	return from, home
}
func scan(t *testing.T, from, home string) *Plan {
	t.Helper()
	p, err := Scan(from, home)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestPlanGolden(t *testing.T) {
	from, home := fixture(t)
	before, _ := snapshot(from)
	p := scan(t, from, home)
	var b bytes.Buffer
	if err := p.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "migrate-plan", b.String())
	after, _ := snapshot(from)
	if !equal(before, after) {
		t.Fatal("dry run changed the source")
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("dry run created the destination")
	}
	if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
		t.Fatalf("unexpected blockers: %v %v", p.Conflicts, p.Unmapped)
	}
	again := scan(t, from, home)
	if !equal(p.result, again.result) {
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
	golden.Assert(t, "migrate-blocked-plan", b.String())
	if len(p.Conflicts) == 0 || len(p.Unmapped) == 0 {
		t.Fatal("missing blockers")
	}
}
func TestIdentityFromLocalGit(t *testing.T) {
	testgit.Isolate(t)
	for _, remote := range []string{"https://example.com/team/widget.git", ""} {
		t.Run(remote, func(t *testing.T) {
			root := t.TempDir()
			repo := filepath.Join(root, "repo")
			if err := os.Mkdir(repo, 0700); err != nil {
				t.Fatal(err)
			}
			testgit.Init(t, repo, remote, true)
			from := filepath.Join(root, "old")
			put(t, filepath.Join(from, "projects", "widget", "project.md"), "---\nfolder: "+repo+"\n---\n")
			p := scan(t, from, filepath.Join(root, "new"))
			r, err := parseRecord(p.result["projects/widget/project.md"].Data)
			if err != nil {
				t.Fatal(err)
			}
			keys := strings.Join(r.values("key"), "\n")
			if !strings.Contains(keys, "root:") {
				t.Fatalf("missing root key: %s", keys)
			}
			if remote != "" && !strings.Contains(keys, "remote:example.com/team/widget") {
				t.Fatalf("missing remote key: %s", keys)
			}
			if !ulidRE.MatchString(r.value("id")) {
				t.Fatal("project id is not a ULID")
			}
		})
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
	for _, kind := range []string{"symlink", "missing-project", "bad-task", "bad-session", "git-worktree", "unknown-pointer"} {
		t.Run(kind, func(t *testing.T) {
			from, home := fixture(t)
			switch kind {
			case "symlink":
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
	put(t, filepath.Join(home, "projects/widget/project.md"), "---\nid: 01K00000000000000000000009\nfolder: /example/work/existing\n---\n")
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

func TestMultiRepoIdentity(t *testing.T) {
	testgit.Isolate(t)
	root := t.TempDir()
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
	r, err := parseRecord(p.result["projects/suite/project.md"].Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.values("repo")) != 2 || strings.Join(r.values("key"), ",") != "remote:example.com/team/one,remote:example.com/team/two" {
		t.Fatalf("bad multi-repo identity: %s", p.result["projects/suite/project.md"].Data)
	}
}
