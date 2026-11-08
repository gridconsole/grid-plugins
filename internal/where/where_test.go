package where

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// machine is the fixture's own machine id; other is a second computer.
const machine, other = "01LAPTOP", "01DESK"

type fixture struct {
	t                         *testing.T
	root, home, gridHome, tmp string
	r                         Resolver
}

// newFixture makes a fake machine under one temp folder: a home, a Grid home in it
// and a temp folder of its own, so folders beside them are plain folders. The fixture
// root is the git ceiling, so a repo around the real temp folder is never seen.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	f := &fixture{t: t, root: root, home: filepath.Join(root, "home"), tmp: filepath.Join(root, "tmp")}
	f.gridHome = filepath.Join(f.home, ".grid")
	for _, d := range []string{f.home, f.gridHome, f.tmp} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.r = Resolver{GridHome: f.gridHome, Home: f.home, TempRoots: []string{f.tmp}, Machine: machine, Ceiling: root}
	return f
}

func (f *fixture) dir(parts ...string) string {
	f.t.Helper()
	p := filepath.Join(append([]string{f.root}, parts...)...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// record writes projects/<rec.Name>/project.md.
func (f *fixture) record(rec project.Record) {
	f.t.Helper()
	if err := project.Write(f.gridHome, rec); err != nil {
		f.t.Fatal(err)
	}
}

// here lists folders on the fixture's machine.
func here(paths ...string) []project.Folder {
	var out []project.Folder
	for _, p := range paths {
		out = append(out, project.Folder{Machine: machine, Path: p})
	}
	return out
}

func keys(ks ...string) []project.Key {
	var out []project.Key
	for _, k := range ks {
		out = append(out, project.NewKey(machine, k))
	}
	return out
}

func (f *fixture) task(project, task string) string {
	f.t.Helper()
	p := filepath.Join(f.gridHome, "projects", project, "tasks", task)
	if err := os.MkdirAll(filepath.Join(p, "artifacts"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// personalTask makes <grid home>/personal/tasks/<task>, where Personal tasks live.
func (f *fixture) personalTask(task string) string {
	f.t.Helper()
	p := filepath.Join(f.gridHome, "personal", "tasks", task)
	if err := os.MkdirAll(filepath.Join(p, "artifacts"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func wantPersonalTask(rule Rule, folder, task, path string) Result {
	w := want(rule, Personal, folder)
	w.Task, w.TaskPath = task, path
	return w
}

func want(rule Rule, name, folder string) Result {
	return Result{Project: name, Rule: rule, Folder: folder, Repos: []string{}, Keys: []string{}}
}

type resolveCase struct {
	name  string
	setup func(f *fixture) (Query, Result)
}

func resolveCases() []resolveCase {
	return []resolveCase{
		{"rule 1: explicit project by name", func(f *fixture) (Query, Result) {
			folder := f.dir("work", "example")
			f.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: here(folder)})
			w := want(RuleExplicit, "example", folder)
			w.ID, w.Record = "01EXAMPLE", true
			return Query{Cwd: f.tmp, Project: "example"}, w
		}},
		{"rule 1: explicit project by id", func(f *fixture) (Query, Result) {
			folder := f.dir("work", "example")
			f.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: here(folder)})
			w := want(RuleExplicit, "example", folder)
			w.ID, w.Record = "01EXAMPLE", true
			return Query{Cwd: f.tmp, Project: "01EXAMPLE"}, w
		}},
		{"rule 1: explicit task by path beats the folder", func(f *fixture) (Query, Result) {
			f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			path := f.task("example", "T-07-first")
			w := want(RuleExplicit, "example", "")
			w.ID, w.Record, w.Task, w.TaskPath = "01EXAMPLE", true, "T-07-first", path
			return Query{Cwd: f.dir("work", "elsewhere"), Task: filepath.Join(path, "artifacts")}, w
		}},
		{"rule 1: explicit task id with a project", func(f *fixture) (Query, Result) {
			f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			path := f.task("example", "T-07-first")
			w := want(RuleExplicit, "example", "")
			w.ID, w.Record, w.Task, w.TaskPath = "01EXAMPLE", true, "T-07-first", path
			return Query{Cwd: f.tmp, Task: "T-07", Project: "example"}, w
		}},
		{"rule 1: explicit task id in the folder's project", func(f *fixture) (Query, Result) {
			folder := f.dir("work", "example")
			f.record(project.Record{Name: "example", Folders: here(folder)})
			path := f.task("example", "T-07-first")
			w := want(RuleExplicit, "example", folder)
			w.Record, w.Task, w.TaskPath = true, "T-07-first", path
			return Query{Cwd: filepath.Join(folder), Task: "T-07-first"}, w
		}},
		{"rule 1: explicit project personal needs no record", func(f *fixture) (Query, Result) {
			return Query{Cwd: f.tmp, Project: "Personal"}, want(RuleExplicit, Personal, f.tmp)
		}},
		{"rule 1: explicit Personal task by path", func(f *fixture) (Query, Result) {
			path := f.personalTask("T-03-note")
			return Query{Cwd: f.dir("work", "elsewhere"), Task: filepath.Join(path, "artifacts")}, wantPersonalTask(RuleExplicit, "", "T-03-note", path)
		}},
		{"rule 1: explicit Personal task id with project personal", func(f *fixture) (Query, Result) {
			path := f.personalTask("T-03-note")
			return Query{Cwd: f.tmp, Task: "T-03", Project: "personal"}, wantPersonalTask(RuleExplicit, "", "T-03-note", path)
		}},
		{"rule 1: explicit Personal task id from inside a Personal task", func(f *fixture) (Query, Result) {
			path := f.personalTask("T-03-note")
			return Query{Cwd: path, Task: "T-03"}, wantPersonalTask(RuleExplicit, "", "T-03-note", path)
		}},
		{"rule 2: inside a Personal task folder", func(f *fixture) (Query, Result) {
			path := f.personalTask("T-03-note")
			return Query{Cwd: filepath.Join(path, "artifacts")}, wantPersonalTask(RuleTask, "", "T-03-note", path)
		}},
		{"rule 2: inside a task folder", func(f *fixture) (Query, Result) {
			f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			path := f.task("example", "T-07-first")
			w := want(RuleTask, "example", "")
			w.ID, w.Record, w.Task, w.TaskPath = "01EXAMPLE", true, "T-07-first", path
			return Query{Cwd: filepath.Join(path, "artifacts")}, w
		}},
		{"rule 2: a task of a project with no record yet", func(f *fixture) (Query, Result) {
			path := f.task("new-one", "T-01-start")
			w := want(RuleTask, "new-one", "")
			w.Task, w.TaskPath = "T-01-start", path
			return Query{Cwd: path}, w
		}},
		{"rule 2: a project folder in the Grid home is Personal", func(f *fixture) (Query, Result) {
			f.record(project.Record{Name: "p1", ID: "01P1"})
			dir := filepath.Join(f.gridHome, "projects", "p1")
			return Query{Cwd: dir}, want(RulePersonal, Personal, dir)
		}},
		{"rule 2: a Grid home folder with repos below is Personal", func(f *fixture) (Query, Result) {
			wt := filepath.Join(f.gridHome, "projects", "p1", "worktrees")
			testgit.Init(f.t, f.dir("home", ".grid", "projects", "p1", "worktrees", "app"), "", false)
			return Query{Cwd: wt}, want(RulePersonal, Personal, wt)
		}},
		{"rule 2: the Grid home itself is Personal", func(f *fixture) (Query, Result) {
			return Query{Cwd: f.gridHome}, want(RulePersonal, Personal, f.gridHome)
		}},
		{"rule 3: nearest project record wins", func(f *fixture) (Query, Result) {
			outer, inner := f.dir("work"), f.dir("work", "inner")
			f.record(project.Record{Name: "outer", ID: "01OUTER", Folders: here(outer)})
			f.record(project.Record{Name: "inner", ID: "01INNER", Folders: here(inner), Keys: keys("remote:example.com/org/inner")})
			w := want(RuleRecord, "inner", inner)
			w.ID, w.Record, w.Keys = "01INNER", true, []string{"remote:example.com/org/inner"}
			return Query{Cwd: f.dir("work", "inner", "sub")}, w
		}},
		{"rule 3: a record beats the git repo below it", func(f *fixture) (Query, Result) {
			suite := f.dir("work", "suite")
			f.record(project.Record{Name: "suite", ID: "01SUITE", Folders: here(suite)})
			repo := f.dir("work", "suite", "app")
			testgit.Init(f.t, repo, "", true)
			w := want(RuleRecord, "suite", suite)
			w.ID, w.Record = "01SUITE", true
			return Query{Cwd: f.dir("work", "suite", "app", "src")}, w
		}},
		{"rule 3: a folder recorded on another machine does not count", func(f *fixture) (Query, Result) {
			dir := f.dir("work", "example")
			f.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: other, Path: dir}}})
			return Query{Cwd: dir}, want(RuleFolder, "example-2", dir)
		}},
		{"rule 4: a folder of child repos is a multi-repo project", func(f *fixture) (Query, Result) {
			suite := f.dir("work", "suite")
			a, b := f.dir("work", "suite", "a"), f.dir("work", "suite", "libs", "b")
			f.dir("work", "suite", "docs")
			testgit.Init(f.t, a, "", false)
			testgit.Init(f.t, b, "", false)
			w := want(RuleMultiRepo, "suite", suite)
			w.Repos = []string{a, b}
			return Query{Cwd: suite}, w
		}},
		{"rule 4: repos deeper than two levels do not count", func(f *fixture) (Query, Result) {
			top := f.dir("work", "deep")
			testgit.Init(f.t, f.dir("work", "deep", "x", "y", "z"), "", false)
			return Query{Cwd: top}, want(RuleFolder, "deep", top)
		}},
		{"rule 4: home with child repos is Personal", func(f *fixture) (Query, Result) {
			testgit.Init(f.t, f.dir("home", "code", "x"), "", false)
			return Query{Cwd: f.home}, want(RulePersonal, Personal, f.home)
		}},
		{"rule 4: a temp subfolder with child repos is Personal", func(f *fixture) (Query, Result) {
			dir := f.dir("tmp", "scratch")
			testgit.Init(f.t, f.dir("tmp", "scratch", "x"), "", false)
			return Query{Cwd: dir}, want(RulePersonal, Personal, dir)
		}},
		{"rule 5: git repo with a remote keyed by common dir and remote", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "app")
			testgit.Init(f.t, repo, "git@example.com:Org/App.git", true)
			w := want(RuleGit, "app", repo)
			w.Keys = []string{"common:" + filepath.Join(repo, ".git"), "remote:example.com/org/app"}
			return Query{Cwd: f.dir("work", "app", "pkg")}, w
		}},
		{"rule 5: a fork with another remote is not the original project", func(f *fixture) (Query, Result) {
			orig := f.dir("work", "app")
			testgit.Init(f.t, orig, "https://example.com/org/app.git", true)
			sha := testgit.Run(f.t, orig, "rev-list", "--max-parents=0", "HEAD")
			f.record(project.Record{Name: "app-record", ID: "01APP", Folders: here(orig),
				Keys: keys("common:"+filepath.Join(orig, ".git"), "remote:example.com/org/app", "root:"+sha)})
			fork := filepath.Join(f.dir("forks"), "fork-app")
			testgit.Run(f.t, f.root, "clone", "-q", orig, fork)
			testgit.Run(f.t, fork, "remote", "set-url", "origin", "https://example.com/someone/app.git")
			w := want(RuleGit, "fork-app", fork)
			w.Keys = []string{"common:" + filepath.Join(fork, ".git"), "remote:example.com/someone/app"}
			return Query{Cwd: fork}, w
		}},
		{"rule 5: a repo with no remote finds its record by root commit", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "moved")
			testgit.Init(f.t, repo, "", true)
			sha := testgit.Run(f.t, repo, "rev-list", "--max-parents=0", "HEAD")
			f.record(project.Record{Name: "local-record", ID: "01LOCAL", Keys: keys("root:" + sha)})
			w := want(RuleGit, "local-record", repo)
			w.ID, w.Record = "01LOCAL", true
			w.Keys = []string{"common:" + filepath.Join(repo, ".git"), "root:" + sha}
			return Query{Cwd: repo}, w
		}},
		{"rule 5: a common-dir key of another machine does not match", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "fresh")
			testgit.Init(f.t, repo, "", false)
			common := "common:" + filepath.Join(repo, ".git")
			f.record(project.Record{Name: "desk", ID: "01DESK", Keys: []project.Key{{Key: common, Machine: other}}})
			w := want(RuleGit, "fresh", repo)
			w.Keys = []string{common}
			return Query{Cwd: repo}, w
		}},
		{"rule 5: git repo without a remote", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "local")
			testgit.Init(f.t, repo, "", true)
			sha := testgit.Run(f.t, repo, "rev-list", "--max-parents=0", "HEAD")
			w := want(RuleGit, "local", repo)
			w.Keys = []string{"common:" + filepath.Join(repo, ".git"), "root:" + sha}
			return Query{Cwd: repo}, w
		}},
		{"rule 5: git repo without commits", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "fresh")
			testgit.Init(f.t, repo, "", false)
			w := want(RuleGit, "fresh", repo)
			w.Keys = []string{"common:" + filepath.Join(repo, ".git")}
			return Query{Cwd: repo}, w
		}},
		{"rule 5: a worktree finds the record of its main repo by remote", func(f *fixture) (Query, Result) {
			repo := f.dir("work", "app")
			testgit.Init(f.t, repo, "https://example.com/org/app.git", true)
			f.record(project.Record{Name: "app-record", ID: "01APP", Keys: keys("remote:example.com/org/app")})
			wt := filepath.Join(f.dir("wt"), "job-1")
			testgit.Run(f.t, repo, "worktree", "add", "-q", wt, "-b", "job-1")
			w := want(RuleGit, "app-record", repo)
			w.ID, w.Record = "01APP", true
			w.Keys = []string{"common:" + filepath.Join(repo, ".git"), "remote:example.com/org/app"}
			return Query{Cwd: wt}, w
		}},
		{"rule 6: a plain folder is its own project", func(f *fixture) (Query, Result) {
			dir := f.dir("notes", "ideas")
			return Query{Cwd: dir}, want(RuleFolder, "ideas", dir)
		}},
		{"rule 6: a taken project name gets a suffix", func(f *fixture) (Query, Result) {
			f.record(project.Record{Name: "ideas", Folders: here(f.dir("elsewhere", "ideas"))})
			dir := f.dir("notes", "ideas")
			return Query{Cwd: dir}, want(RuleFolder, "ideas-2", dir)
		}},
		{"rule 6: home is Personal", func(f *fixture) (Query, Result) {
			return Query{Cwd: f.home}, want(RulePersonal, Personal, f.home)
		}},
		{"rule 6: a temp folder is Personal", func(f *fixture) (Query, Result) {
			return Query{Cwd: f.tmp}, want(RulePersonal, Personal, f.tmp)
		}},
		{"rule 6: a folder below a temp folder is Personal", func(f *fixture) (Query, Result) {
			dir := f.dir("tmp", "x", "y")
			return Query{Cwd: dir}, want(RulePersonal, Personal, dir)
		}},
		{"rule 6: the filesystem root is Personal", func(f *fixture) (Query, Result) {
			return Query{Cwd: string(filepath.Separator)}, want(RulePersonal, Personal, string(filepath.Separator))
		}},
	}
}

func TestResolve(t *testing.T) {
	for _, tc := range resolveCases() {
		t.Run(tc.name, func(t *testing.T) { runCase(t, newFixture(t), tc) })
	}
}

// The same rules hold when the temp folder, and so every fixture folder, is inside a
// git checkout: the ceiling keeps that repo out of the walk.
func TestResolveWithTempInsideAGitRepo(t *testing.T) {
	for _, tc := range resolveCases() {
		t.Run(tc.name, func(t *testing.T) {
			outer := gridhome.Real(testgit.TempInRepo(t))
			f := newFixture(t)
			if root, ok := gitx.FindRoot(f.root, ""); !ok || root != outer {
				t.Fatalf("fixture %s is not inside the repo %s", f.root, outer)
			}
			runCase(t, f, tc)
		})
	}
}

func runCase(t *testing.T, f *fixture, tc resolveCase) {
	t.Helper()
	q, want := tc.setup(f)
	got, err := f.r.Resolve(q)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func equal(a, b Result) bool {
	return a.Project == b.Project && a.ID == b.ID && a.Rule == b.Rule && a.Folder == b.Folder &&
		a.Record == b.Record && a.Task == b.Task && a.TaskPath == b.TaskPath &&
		strings.Join(a.Repos, "\n") == strings.Join(b.Repos, "\n") &&
		strings.Join(a.Keys, "\n") == strings.Join(b.Keys, "\n")
}

func TestResolveRefusals(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(f *fixture) Query
		wantErr string
	}{
		{"task path outside the Grid home", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Task: f.dir("elsewhere", "tasks", "T-01-x")}
		}, "outside the Grid home"},
		{"task path through a symlink out of the Grid home", func(f *fixture) Query {
			out := f.dir("elsewhere", "T-01-x")
			link := filepath.Join(f.gridHome, "projects", "p", "tasks", "T-01-x")
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				f.t.Fatal(err)
			}
			if err := os.Symlink(out, link); err != nil {
				f.t.Skip("symlinks not available")
			}
			return Query{Cwd: f.tmp, Task: link}
		}, "outside the Grid home"},
		{"task path relative with dots out of the Grid home", func(f *fixture) Query {
			return Query{Cwd: f.gridHome, Task: "../../elsewhere"}
		}, "outside the Grid home"},
		{"task path written with forward slashes", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Task: "elsewhere/T-01-x"}
		}, "outside the Grid home"},
		{"a broken project record", func(f *fixture) Query {
			path := filepath.Join(f.gridHome, "projects", "bad", "project.md")
			f.dir("home", ".grid", "projects", "bad")
			if err := os.WriteFile(path, []byte("---\nfolder: a\nfolder: b\n---\n"), 0o644); err != nil {
				f.t.Fatal(err)
			}
			return Query{Cwd: f.tmp}
		}, "project.md"},
		{"no Grid home", func(f *fixture) Query {
			f.r.GridHome = ""
			return Query{Cwd: f.tmp}
		}, "no Grid home"},
		{"task path inside the Grid home but not a task folder", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Task: f.gridHome}
		}, "not inside a task folder"},
		{"task folder that does not exist", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Task: filepath.Join(f.gridHome, "projects", "p", "tasks", "T-99-gone")}
		}, "does not exist"},
		{"unknown project", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Project: "nope"}
		}, "is not in"},
		{"task id with no project", func(f *fixture) Query {
			return Query{Cwd: f.dir("notes"), Task: "T-07"}
		}, "needs --project"},
		{"task id that is not there", func(f *fixture) Query {
			f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			f.task("example", "T-01-a")
			return Query{Cwd: f.tmp, Task: "T-02", Project: "example"}
		}, "not found"},
		{"Personal task under another project", func(f *fixture) Query {
			f.record(project.Record{Name: "one", ID: "01ONE"})
			return Query{Cwd: f.tmp, Task: f.personalTask("T-01-a"), Project: "one"}
		}, "belongs to project personal, not one"},
		{"Personal task id that is not there", func(f *fixture) Query {
			f.personalTask("T-01-a")
			return Query{Cwd: f.tmp, Task: "T-02", Project: "personal"}
		}, "not found in project personal"},
		{"Personal task folder that does not exist", func(f *fixture) Query {
			return Query{Cwd: f.tmp, Task: filepath.Join(f.gridHome, "personal", "tasks", "T-99-gone")}
		}, "does not exist"},
		{"task of another project", func(f *fixture) Query {
			f.record(project.Record{Name: "one", ID: "01ONE"})
			f.record(project.Record{Name: "two", ID: "01TWO"})
			return Query{Cwd: f.tmp, Task: f.task("one", "T-01-a"), Project: "two"}
		}, "belongs to project one"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.r.Resolve(tc.setup(f))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
