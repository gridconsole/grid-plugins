package jobcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// prTask is the fixture task folder.
func prTask(e Env) string { return filepath.Join(e.Cwd, "T-01") }

// prWrite writes one file, replacing whatever is at path.
func prWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// prProjectMD is the path of the fixture project's record.
func prProjectMD(e Env) string {
	return filepath.Join(e.GridHome, "projects", jobtest.Project, "project.md")
}

// prRecord writes a project record with the fixture folder on this machine and the given repo paths.
func prRecord(t *testing.T, e Env, repos ...string) {
	t.Helper()
	text := fmt.Sprintf("---\nfolders:\n  - {machine: %s, path: %q}\n", jobtest.Machine, filepath.Join(e.GridHome, "work"))
	if len(repos) > 0 {
		text += "repos:\n"
		for _, r := range repos {
			text += fmt.Sprintf("  - {path: %q}\n", r)
		}
	}
	prWrite(t, prProjectMD(e), text+"---\n")
}

// prTaskMD rewrites task.md with the fixture's front matter plus extra lines.
func prTaskMD(t *testing.T, e Env, extra string) {
	t.Helper()
	text := "---\nid: T-01\ntitle: Export documentation\nproject: demo\nstate: waiting\ncreated: 2026-01-01T00:00:00Z\nowner: member\n" + extra + "---\n# Export documentation\n"
	prWrite(t, filepath.Join(prTask(e), "task.md"), text)
}

// prBase is the project folder as roots sees it, symlinks resolved.
func prBase(t *testing.T, e Env) string {
	t.Helper()
	b, err := filepath.EvalSymlinks(filepath.Join(e.GridHome, "work"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// prJobsListing lists everything under the task's jobs folder; a missing folder is empty.
func prJobsListing(t *testing.T, e Env) string {
	t.Helper()
	var names []string
	root := filepath.Join(prTask(e), "jobs")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		names = append(names, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(names, "\n")
}

// prNoRepos fails unless sources found no project folders and no repos.
func prNoRepos(t *testing.T, w *world) {
	t.Helper()
	if len(w.files.Folders) != 0 || len(w.files.Repos) != 0 || len(w.repos) != 0 || w.files.Repo != "" {
		t.Errorf("want no folders and no repos; got folders %v repo %q repos %v names %v", w.files.Folders, w.files.Repo, w.files.Repos, w.repos)
	}
}

// task.md that is a directory: sources returns the read error as it is. render reaches sources
// (loadJob does not read task.md), so this runs through grid job render, LintOptions and sources.
func TestSourcesTaskMdUnreadable(t *testing.T) {
	e, out, errs := fixture(t)
	task := prTask(e)
	if err := os.Remove(filepath.Join(task, "task.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(task, "task.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := prJobsListing(t, e)

	_, err := sources(task, e)
	if err == nil {
		t.Fatal("sources: no error for a task.md that is a directory")
	}
	if strings.HasPrefix(err.Error(), "task.md: ") || !strings.Contains(err.Error(), filepath.Join(task, "task.md")) {
		t.Errorf("sources: %q, want the bare read error naming the task.md path", err)
	}
	if _, lerr := LintOptions(task, e); lerr == nil || lerr.Error() != err.Error() {
		t.Errorf("LintOptions: %v, want %q", lerr, err)
	}
	r := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if r.OK || r.Error != err.Error() {
		t.Errorf("render: ok %v error %q, want refused with %q", r.OK, r.Error, err)
	}
	if after := prJobsListing(t, e); after != before {
		t.Errorf("render wrote under jobs:\nbefore %q\nafter  %q", before, after)
	}
}

// task.md without a front matter block: the "task.md: " wrapper of frontmatter.Parse's error.
func TestSourcesTaskMdDoesNotParse(t *testing.T) {
	e, out, errs := fixture(t)
	task := prTask(e)
	prWrite(t, filepath.Join(task, "task.md"), "no front matter here\n")
	before := prJobsListing(t, e)

	_, err := sources(task, e)
	if err == nil || err.Error() != "task.md: line 1: missing frontmatter block" {
		t.Errorf("sources: %v, want task.md: line 1: missing frontmatter block", err)
	}
	r := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if r.OK || !strings.HasPrefix(r.Error, "task.md: ") {
		t.Errorf("render: ok %v error %q, want an error starting with task.md: ", r.OK, r.Error)
	}
	if after := prJobsListing(t, e); after != before {
		t.Errorf("render wrote under jobs:\nbefore %q\nafter  %q", before, after)
	}
}

// projectOf names the project only from the task's place under the Grid home and has no error.
func TestProjectOfPlaces(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{"projects/demo/tasks/T-01", "projects/demo/notes/a", "projects/demo/tasks"} {
		if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	elsewhere := filepath.Join(t.TempDir(), "projects", "demo", "tasks", "T-01")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, task, home, want string
	}{
		{"grid home empty", filepath.Join(home, "projects", "demo", "tasks", "T-01"), "", ""},
		{"task outside the grid home", elsewhere, home, ""},
		{"under the project but not under tasks", filepath.Join(home, "projects", "demo", "notes", "a"), home, ""},
		{"the tasks folder itself", filepath.Join(home, "projects", "demo", "tasks"), home, ""},
		{"projects/<p>/tasks/<task>", filepath.Join(home, "projects", "demo", "tasks", "T-01"), home, "demo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := projectOf(c.task, c.home); got != c.want {
				t.Errorf("projectOf(%q, %q) = %q, want %q", c.task, c.home, got, c.want)
			}
		})
	}
}

// With no project named, sources skips roots even when a record and a folder exist.
func TestSourcesSkipsRootsWithoutProject(t *testing.T) {
	e, _, _ := fixture(t)
	prRecord(t, e, "sub/repo")
	elsewhere := filepath.Join(t.TempDir(), "T-01")
	prWrite(t, filepath.Join(elsewhere, "task.md"), "---\nid: T-01\nowner: member\n---\n")

	t.Run("task outside the grid home", func(t *testing.T) {
		w, err := sources(elsewhere, e)
		if err != nil {
			t.Fatal(err)
		}
		if w.project != "" {
			t.Errorf("project %q, want none", w.project)
		}
		prNoRepos(t, w)
	})
	t.Run("grid home empty", func(t *testing.T) {
		e2 := e
		e2.GridHome = ""
		w, err := sources(prTask(e), e2)
		if err != nil {
			t.Fatal(err)
		}
		if w.project != "" {
			t.Errorf("project %q, want none", w.project)
		}
		prNoRepos(t, w)
	})
}

func TestRootsProjectRecord(t *testing.T) {
	t.Run("no project.md is no error and no folders", func(t *testing.T) {
		e, _, _ := fixture(t)
		if err := os.Remove(prProjectMD(e)); err != nil {
			t.Fatal(err)
		}
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		if w.project != jobtest.Project {
			t.Errorf("project %q, want %q", w.project, jobtest.Project)
		}
		prNoRepos(t, w)
	})
	t.Run("project.md a directory", func(t *testing.T) {
		e, _, _ := fixture(t)
		if err := os.Remove(prProjectMD(e)); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(prProjectMD(e), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := sources(prTask(e), e)
		if err == nil || strings.HasPrefix(err.Error(), "project ") || !strings.Contains(err.Error(), prProjectMD(e)) {
			t.Errorf("got %v, want the bare read error naming %s", err, prProjectMD(e))
		}
	})
	t.Run("project.md does not parse", func(t *testing.T) {
		e, _, _ := fixture(t)
		prWrite(t, prProjectMD(e), "folders: nothing\n")
		_, err := sources(prTask(e), e)
		if err == nil || err.Error() != "project demo: no front matter between --- lines" {
			t.Errorf("got %v, want project demo: no front matter between --- lines", err)
		}
	})
	t.Run("project.md front matter is not yaml", func(t *testing.T) {
		e, _, _ := fixture(t)
		prWrite(t, prProjectMD(e), "---\nfolders: [\n---\n")
		_, err := sources(prTask(e), e)
		if err == nil || !strings.HasPrefix(err.Error(), "project demo: front matter: ") {
			t.Errorf("got %v, want an error starting with project demo: front matter: ", err)
		}
	})
	t.Run("a folder only on another machine is no error and no repos", func(t *testing.T) {
		e, _, _ := fixture(t)
		prWrite(t, prProjectMD(e), fmt.Sprintf("---\nfolders:\n  - {machine: other, path: %q}\nrepos:\n  - {path: sub/repo}\n---\n", filepath.Join(e.GridHome, "work")))
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		prNoRepos(t, w)
	})
	t.Run("a record with no folders is no error and no repos", func(t *testing.T) {
		e, _, _ := fixture(t)
		prWrite(t, prProjectMD(e), "---\nrepos:\n  - {path: sub/repo}\n---\n")
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		prNoRepos(t, w)
	})
}

func TestRootsRepoPaths(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "elsewhere")
	for _, p := range []string{"../x", outside} {
		t.Run("refused "+p, func(t *testing.T) {
			e, _, _ := fixture(t)
			prRecord(t, e, p)
			_, err := sources(prTask(e), e)
			want := fmt.Sprintf("project demo: repo path %q must stay inside the project folder", p)
			if err == nil || err.Error() != want {
				t.Errorf("got %v, want %s", err, want)
			}
		})
	}
	t.Run("a bad path after a good one is refused", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e, "ok", "../x")
		_, err := sources(prTask(e), e)
		if want := `project demo: repo path "../x" must stay inside the project folder`; err == nil || err.Error() != want {
			t.Errorf("got %v, want %s", err, want)
		}
	})
	t.Run("sub/repo inside is accepted", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e, "sub/repo")
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(prBase(t, e), "sub", "repo")
		if w.files.Repo != want || w.files.Repos["sub/repo"] != want || len(w.files.Repos) != 1 || len(w.repos) != 1 || w.repos[0] != "sub/repo" {
			t.Errorf("repo %q repos %v names %v, want sub/repo -> %q", w.files.Repo, w.files.Repos, w.repos, want)
		}
	})
	t.Run("the first repo is the primary one and . names no repo", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e, ".", "b/c")
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		base := prBase(t, e)
		if w.files.Repo != base {
			t.Errorf("primary repo %q, want %q", w.files.Repo, base)
		}
		if len(w.files.Repos) != 1 || w.files.Repos["b/c"] != filepath.Join(base, "b", "c") {
			t.Errorf("repos %v, want only b/c", w.files.Repos)
		}
	})
	// On Linux a backslash is not a separator, so this one is compiled here and needs a Windows run:
	// GOOS=windows go test -c -o prrecord.test.exe ./internal/job/jobcmd/ then run it on Windows with -test.run TestRootsRepoPaths.
	t.Run("windows forms are refused", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("compiled, needs Windows run")
		}
		for _, p := range []string{`..\x`, `D:\other`} {
			e, _, _ := fixture(t)
			prRecord(t, e, p)
			_, err := sources(prTask(e), e)
			want := fmt.Sprintf("project demo: repo path %q must stay inside the project folder", p)
			if err == nil || err.Error() != want {
				t.Errorf("%s: got %v, want %s", p, err, want)
			}
		}
	})
}

// A relative prototype is joined to the project folder; an absolute one is kept.
func TestRootsPrototype(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "proto", "index.html")
	t.Run("relative", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e)
		prTaskMD(t, e, "prototype: design/proto.html\n")
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(prBase(t, e), "design", "proto.html"); w.files.Prototype != want {
			t.Errorf("prototype %q, want %q", w.files.Prototype, want)
		}
	})
	t.Run("absolute", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e)
		prTaskMD(t, e, fmt.Sprintf("prototype: %q\n", abs))
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		if w.files.Prototype != abs {
			t.Errorf("prototype %q, want %q", w.files.Prototype, abs)
		}
	})
	t.Run("none named", func(t *testing.T) {
		e, _, _ := fixture(t)
		prRecord(t, e)
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		if w.files.Prototype != "" {
			t.Errorf("prototype %q, want none", w.files.Prototype)
		}
	})
	t.Run("no folder on this machine leaves it unset", func(t *testing.T) {
		e, _, _ := fixture(t)
		prWrite(t, prProjectMD(e), "---\n---\n")
		prTaskMD(t, e, "prototype: design/proto.html\n")
		w, err := sources(prTask(e), e)
		if err != nil {
			t.Fatal(err)
		}
		if w.files.Prototype != "" {
			t.Errorf("prototype %q, want none", w.files.Prototype)
		}
	})
}
