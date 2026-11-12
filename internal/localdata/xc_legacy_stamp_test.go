package localdata

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

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

func TestXcFreshHomeStamped(t *testing.T) {
	for _, kind := range []string{"missing", "empty"} {
		t.Run(kind, func(t *testing.T) {
			home := filepath.Join(gridhome.Real(t.TempDir()), ".grid")
			if kind == "empty" {
				if err := os.MkdirAll(home, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			unlock, err := Lock(home, ".xc.lock")
			if err != nil {
				t.Fatal(err)
			}
			unlock()
			b, err := os.ReadFile(filepath.Join(home, "VERSION"))
			if err != nil || string(b) != "1\n" {
				t.Fatalf("VERSION %q, %v; want %q", b, err, "1\n")
			}
		})
	}
}

func TestXcLegacyHomeNotStamped(t *testing.T) {
	// Lock reaches Init; Machine and Init are the other doors to the stamp.
	doors := map[string]func(home string) error{
		"Lock": func(home string) error {
			unlock, err := Lock(home, ".xc.lock")
			if err == nil {
				unlock()
			}
			return err
		},
		"Machine": func(home string) error { _, err := Machine(home); return err },
		"Init":    Init,
	}
	for name, door := range doors {
		t.Run(name, func(t *testing.T) {
			home := filepath.Join(gridhome.Real(t.TempDir()), ".grid")
			xcCopyTree(t, filepath.Join("..", "migrate", "testdata", "owner-home"), home)
			before := xcTree(t, home)
			err := door(home)
			if err == nil || !strings.Contains(err.Error(), "Run: grid migrate") {
				t.Errorf("%s on an old home: error %v, want one containing %q", name, err, "Run: grid migrate")
			}
			if b, err := os.ReadFile(filepath.Join(home, "VERSION")); err == nil {
				t.Errorf("VERSION written: %q", b)
			}
			if d := xcDiff(before, xcTree(t, home)); d != "" {
				t.Errorf("the old home changed:\n%s", d)
			}
		})
	}
}
