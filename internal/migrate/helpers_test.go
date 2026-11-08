package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
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

// fixture copies testdata/migrate/old-grid into a fresh folder, with the example
// paths in it pointing at the copy. The folder is spelled with its real path, so the
// tests do not depend on TMPDIR being a symlink or not.
func fixture(t *testing.T) (from, home string) {
	t.Helper()
	return fixtureIn(t, gridhome.Real(t.TempDir()))
}

func fixtureIn(t *testing.T, root string) (from, home string) {
	t.Helper()
	from = filepath.Join(root, "old-grid")
	copyFixture(t, from)
	return from, filepath.Join(root, ".grid")
}

// wipe empties root, so a test can build the same paths again.
func wipe(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

func copyFixture(t *testing.T, to string) {
	t.Helper()
	src := "../../testdata/migrate/old-grid"
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		put(t, filepath.Join(to, rel), strings.ReplaceAll(string(data), "/example/old-grid", to))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scan(t *testing.T, from, home string, opts ...Option) *Plan {
	t.Helper()
	p, err := Scan(from, home, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// view lists a folder as one line per path: type, permissions and a checksum or link
// target. It leaves out times, so a copy compares equal to its original.
func view(t *testing.T, root string) map[string]string { return viewAs(t, root, nil) }

// viewAs is view with the text of every file passed through repl first.
func viewAs(t *testing.T, root string, repl *strings.Replacer) map[string]string {
	t.Helper()
	s, err := snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for rel, e := range s.m {
		switch {
		case e.Mode.IsDir():
			out[rel] = fmt.Sprintf("dir %o", e.Mode.Perm())
		case e.Mode&fs.ModeSymlink != 0:
			out[rel] = "link " + e.Link
		default:
			sum, err := hashFile(filepath.Join(root, filepath.FromSlash(rel)))
			if repl != nil {
				sum = hashBytes([]byte(repl.Replace(read(t, root, rel))))
			}
			if err != nil {
				t.Fatal(err)
			}
			out[rel] = fmt.Sprintf("file %o %s", e.Mode.Perm(), sum)
		}
	}
	return out
}

func sameView(a, b map[string]string) bool { return reflect.DeepEqual(a, b) }

// without drops paths from a view.
func without(v map[string]string, paths ...string) map[string]string {
	out := map[string]string{}
	for k, s := range v {
		out[k] = s
	}
	for _, p := range paths {
		delete(out, p)
	}
	return out
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// siblings lists what sits beside the home, which a dry run must leave as it was.
func siblings(t *testing.T, home string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(home))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist (%v)", path, err)
	}
}
