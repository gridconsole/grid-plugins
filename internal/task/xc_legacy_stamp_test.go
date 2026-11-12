package task

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// xcStore is a Store whose Grid home is grid, with the working folder root.
func xcStore(t *testing.T, root, grid string) Store {
	t.Helper()
	testgit.Isolate(t)
	return Store{Resolver: where.Resolver{GridHome: grid, Home: root, Ceiling: root},
		Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}
}

// xcCopyTree copies the folder src into dst, files and folders only.
func xcCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// xcTree lists every path under dir with a hash of each file's bytes.
func xcTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[filepath.ToSlash(rel)+"/"] = "dir"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = fmt.Sprintf("%x", sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// xcDiff names what differs between two xcTree listings, "" when they match.
func xcDiff(before, after map[string]string) string {
	var lines []string
	for p, h := range after {
		if old, ok := before[p]; !ok {
			lines = append(lines, "added "+p)
		} else if old != h {
			lines = append(lines, "changed "+p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			lines = append(lines, "removed "+p)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func xcVersion(t *testing.T, grid string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(grid, "VERSION"))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func TestXcFreshHomeStamped(t *testing.T) {
	for _, kind := range []string{"missing", "empty"} {
		t.Run(kind, func(t *testing.T) {
			root := gridhome.Real(t.TempDir())
			grid := filepath.Join(root, ".grid")
			if kind == "empty" {
				mkdir(t, grid)
			}
			if _, err := xcStore(t, root, grid).New(NewOptions{Cwd: root, Words: "First task"}); err != nil {
				t.Fatal(err)
			}
			if v, ok := xcVersion(t, grid); !ok || v != "1\n" {
				t.Fatalf("VERSION %q (present %v), want %q", v, ok, "1\n")
			}
		})
	}
}

func TestXcLegacyHomeNotStamped(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	grid := filepath.Join(root, ".grid")
	xcCopyTree(t, filepath.Join("..", "migrate", "testdata", "owner-home"), grid)
	before := xcTree(t, grid)
	_, err := xcStore(t, root, grid).New(NewOptions{Cwd: root, Words: "Second task"})
	if err == nil || !strings.Contains(err.Error(), "Run: grid migrate") {
		t.Errorf("New on an old home: error %v, want one containing %q", err, "Run: grid migrate")
	}
	if v, ok := xcVersion(t, grid); ok {
		t.Errorf("VERSION written: %q", v)
	}
	if d := xcDiff(before, xcTree(t, grid)); d != "" {
		t.Errorf("the old home changed:\n%s", d)
	}
}

func TestXcAppFilesStillFresh(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	grid := filepath.Join(root, ".grid")
	p := filepath.Join(grid, "projects", "p")
	mkdir(t, filepath.Join(p, "docs"))
	mkdir(t, filepath.Join(p, "brain"))
	if err := os.WriteFile(filepath.Join(p, "project-roles.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := xcVersion(t, grid); ok {
		t.Fatal("fixture already has VERSION")
	}
	made, err := xcStore(t, root, grid).New(NewOptions{Cwd: root, Words: "First task"})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := xcVersion(t, grid); !ok || v != "1\n" {
		t.Errorf("VERSION %q (present %v), want %q", v, ok, "1\n")
	}
	if _, err := os.Stat(filepath.Join(made.Path, "task.md")); err != nil {
		t.Errorf("task not made: %v", err)
	}
}
