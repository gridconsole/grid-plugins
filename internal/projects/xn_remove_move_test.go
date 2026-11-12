package projects

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Remove unregisters a project by moving its data folder whole to
// projects/.removed/<name> (<name>-2, <name>-3 when taken): nothing is deleted
// and nothing in the folder is rewritten.

// xnRmData is a path under <grid home>/projects.
func xnRmData(s Store, parts ...string) string {
	return filepath.Join(append([]string{s.Resolver.GridHome, "projects"}, parts...)...)
}

// xnRmTree reads every file under dir: its bytes by slash path relative to dir.
func xnRmTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func xnRmSame(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// xnRmEntries is List by project name.
func xnRmEntries(t *testing.T, s Store) map[string]Entry {
	t.Helper()
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Entry{}
	for _, e := range list {
		out[e.Name] = e
	}
	return out
}

func xnRmGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s: Lstat error %v, want it gone", path, err)
	}
}

// xnRmHome registers alpha and beta from their folders, then puts a task and a
// knowledge file in the data folder of each.
func xnRmHome(t *testing.T) Store {
	t.Helper()
	s, root := fixture(t)
	for _, name := range []string{"alpha", "beta"} {
		if _, err := s.Add(mkdir(t, filepath.Join(root, "work", name)), name); err != nil {
			t.Fatal(err)
		}
		refWrite(t, xnRmData(s, name, "tasks", "T-1", "task.md"), "---\nid: T-1\n---\n# "+name+" task\n")
		refWrite(t, xnRmData(s, name, "knowledge", "x", "y.md"), name+" knows y\n")
	}
	return s
}

func TestXnRmMovesAndUnlists(t *testing.T) {
	s := xnRmHome(t)
	before := xnRmTree(t, xnRmData(s, "alpha"))
	for _, f := range []string{"project.md", "tasks/T-1/task.md", "knowledge/x/y.md"} {
		if _, ok := before[f]; !ok {
			t.Fatalf("setup: no %s in %q", f, before)
		}
	}
	id := xnRmEntries(t, s)["alpha"].ID
	e, err := s.Remove("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "alpha" || e.ID != id || len(e.Folders) != 0 || len(e.Keys) != 0 {
		t.Fatalf("Remove answered %+v; want alpha %s with no folders or keys", e, id)
	}
	listed := xnRmEntries(t, s)
	if _, ok := listed["alpha"]; ok {
		t.Fatalf("List after Remove still has alpha: %+v", listed)
	}
	if _, ok := listed["beta"]; !ok {
		t.Fatalf("List after Remove lost beta: %+v", listed)
	}
	xnRmGone(t, xnRmData(s, "alpha"))
	xnRmSame(t, "projects/.removed/alpha", xnRmTree(t, xnRmData(s, ".removed", "alpha")), before)
}

func TestXnRmCollision(t *testing.T) {
	s, root := fixture(t)
	refWrite(t, xnRmData(s, ".removed", "alpha", "project.md"), "---\nid: 01JOLDER000000000000000000\n---\nan older alpha\n")
	refWrite(t, xnRmData(s, ".removed", "alpha", "tasks", "T-9", "task.md"), "older task\n")
	want := map[string]map[string]string{"alpha": xnRmTree(t, xnRmData(s, ".removed", "alpha"))}
	for _, target := range []string{"alpha-2", "alpha-3"} {
		if _, err := s.Add(mkdir(t, filepath.Join(root, "work", target)), "alpha"); err != nil {
			t.Fatal(err)
		}
		refWrite(t, xnRmData(s, "alpha", "knowledge", "n.md"), "the alpha that lands at "+target+"\n")
		want[target] = xnRmTree(t, xnRmData(s, "alpha"))
		if _, err := s.Remove("alpha"); err != nil {
			t.Fatalf("Remove for %s: %v", target, err)
		}
		xnRmGone(t, xnRmData(s, "alpha"))
		for name, files := range want {
			xnRmSame(t, "projects/.removed/"+name+" after the move to "+target, xnRmTree(t, xnRmData(s, ".removed", name)), files)
		}
	}
	es, err := os.ReadDir(xnRmData(s, ".removed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("projects/.removed holds %d entries, want alpha, alpha-2, alpha-3", len(es))
	}
}

func TestXnRmOthersUnchanged(t *testing.T) {
	s := xnRmHome(t)
	beta, betaEntry := xnRmTree(t, xnRmData(s, "beta")), xnRmEntries(t, s)["beta"]
	all := xnRmTree(t, xnRmData(s))

	_, err := s.Remove("nope")
	if err == nil || err.Error() != `project "nope" is missing or ambiguous` {
		t.Fatalf("Remove(nope) error = %v", err)
	}
	xnRmSame(t, "projects after Remove(nope)", xnRmTree(t, xnRmData(s)), all)

	// A regular file where projects/.removed goes: the move fails and alpha stays.
	refWrite(t, xnRmData(s, ".removed"), "not a folder\n")
	alpha := xnRmTree(t, xnRmData(s, "alpha"))
	if _, err := s.Remove("alpha"); err == nil {
		t.Fatal("Remove with projects/.removed a regular file did not fail")
	}
	if _, ok := xnRmEntries(t, s)["alpha"]; !ok {
		t.Fatal("alpha is not listed after a failed Remove")
	}
	xnRmSame(t, "projects/alpha after a failed Remove", xnRmTree(t, xnRmData(s, "alpha")), alpha)
	if err := os.Remove(xnRmData(s, ".removed")); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Remove("alpha"); err != nil {
		t.Fatal(err)
	}
	xnRmSame(t, "projects/beta after Remove(alpha)", xnRmTree(t, xnRmData(s, "beta")), beta)
	if got := xnRmEntries(t, s)["beta"]; !reflect.DeepEqual(got, betaEntry) {
		t.Fatalf("beta listed as %+v after Remove(alpha), was %+v", got, betaEntry)
	}
}
