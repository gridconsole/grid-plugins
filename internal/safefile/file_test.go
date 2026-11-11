package safefile

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newRoot returns an os.Root on a fresh "level" folder inside a temp dir. The
// parent holds a file that lies outside the root.
func newRoot(t *testing.T) (*os.Root, string, string) {
	t.Helper()
	parent := t.TempDir()
	level := filepath.Join(parent, "level")
	if err := os.Mkdir(level, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(level)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root, level, parent
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// symlink creates a link or skips: symlink rows are skipped on Windows because
// creating a symlink there needs a privilege the test runner may not have, and
// the rows only prove how Lstat reports links.
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated rights on Windows")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestReadPlainFile(t *testing.T) {
	root, level, _ := newRoot(t)
	write(t, filepath.Join(level, "sub", "a.md"), "hello")
	for _, p := range []string{"sub/a.md", filepath.Join("sub", "a.md"), "./sub/a.md"} {
		b, err := Read(root, p)
		if err != nil || string(b) != "hello" {
			t.Fatalf("Read(%q) = %q, %v; want hello", p, b, err)
		}
		if err := NoSymlinks(root, p); err != nil {
			t.Fatalf("NoSymlinks(%q) = %v, want nil", p, err)
		}
	}
}

func TestReadMissingFile(t *testing.T) {
	root, _, _ := newRoot(t)
	// NoSymlinks lets a missing path through; Read then fails on Lstat.
	if err := NoSymlinks(root, "nope/x.md"); err != nil {
		t.Fatalf("NoSymlinks on missing path = %v, want nil", err)
	}
	if _, err := Read(root, "nope/x.md"); !os.IsNotExist(err) {
		t.Fatalf("Read missing = %v, want not-exist", err)
	}
}

func TestReadRefusesSymlinkToFileInside(t *testing.T) {
	root, level, _ := newRoot(t)
	write(t, filepath.Join(level, "real.md"), "real")
	symlink(t, "real.md", filepath.Join(level, "link.md"))
	assertSymlinkRefused(t, root, "link.md")
}

func TestReadRefusesSymlinkToFileOutside(t *testing.T) {
	root, level, parent := newRoot(t)
	write(t, filepath.Join(parent, "secret.md"), "secret")
	symlink(t, filepath.Join(parent, "secret.md"), filepath.Join(level, "out.md"))
	assertSymlinkRefused(t, root, "out.md")
}

func TestReadRefusesSymlinkedFolderInPath(t *testing.T) {
	root, level, _ := newRoot(t)
	write(t, filepath.Join(level, "realdir", "a.md"), "a")
	symlink(t, "realdir", filepath.Join(level, "linkdir"))
	assertSymlinkRefused(t, root, "linkdir/a.md")
}

func TestReadRefusesSymlinkedFolderOutside(t *testing.T) {
	root, level, parent := newRoot(t)
	write(t, filepath.Join(parent, "elsewhere", "a.md"), "a")
	symlink(t, filepath.Join(parent, "elsewhere"), filepath.Join(level, "linkdir"))
	assertSymlinkRefused(t, root, "linkdir/a.md")
}

func TestReadRefusesDanglingSymlink(t *testing.T) {
	root, level, _ := newRoot(t)
	symlink(t, "gone.md", filepath.Join(level, "dangling.md"))
	assertSymlinkRefused(t, root, "dangling.md")
}

func assertSymlinkRefused(t *testing.T, root *os.Root, path string) {
	t.Helper()
	want := "brain paths cannot be symbolic links"
	if err := NoSymlinks(root, path); err == nil || err.Error() != want {
		t.Fatalf("NoSymlinks(%q) = %v, want %q", path, err, want)
	}
	if b, err := Read(root, path); err == nil || err.Error() != want || b != nil {
		t.Fatalf("Read(%q) = %q, %v; want refusal %q", path, b, err, want)
	}
}

// file.go:18: !filepath.IsLocal(path) refuses "brain path must stay within its level".
func TestReadRefusesPathsOutsideLevel(t *testing.T) {
	root, level, parent := newRoot(t)
	write(t, filepath.Join(parent, "secret.md"), "secret")
	write(t, filepath.Join(level, "in.md"), "in")
	paths := []string{
		"../secret.md",
		"sub/../../secret.md",
		"..",
		"",
		filepath.Join(parent, "secret.md"), // absolute
	}
	if runtime.GOOS != "windows" {
		// "in.md/.." is local after Clean, so only the escaping forms belong here.
		paths = append(paths, "/etc/passwd")
	}
	want := "brain path must stay within its level"
	for _, p := range paths {
		if err := NoSymlinks(root, p); err == nil || err.Error() != want {
			t.Errorf("NoSymlinks(%q) = %v, want %q", p, err, want)
		}
		if b, err := Read(root, p); err == nil || err.Error() != want || b != nil {
			t.Errorf("Read(%q) = %q, %v; want %q", p, b, err, want)
		}
	}
}

// file.go:46: only regular files of at most 1 MiB are read.
func TestReadRefusesDirectory(t *testing.T) {
	root, level, _ := newRoot(t)
	if err := os.Mkdir(filepath.Join(level, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "d"); err == nil || !strings.Contains(err.Error(), "regular files") {
		t.Fatalf("Read(dir) = %v, want regular-file refusal", err)
	}
}

func TestReadSizeLimit(t *testing.T) {
	root, level, _ := newRoot(t)
	const limit = 1 << 20
	write(t, filepath.Join(level, "max.bin"), strings.Repeat("x", limit))
	b, err := Read(root, "max.bin")
	if err != nil || !bytes.Equal(b, bytes.Repeat([]byte("x"), limit)) {
		t.Fatalf("Read at exactly 1 MiB: len=%d err=%v, want full read", len(b), err)
	}
	write(t, filepath.Join(level, "big.bin"), strings.Repeat("x", limit+1))
	if b, err := Read(root, "big.bin"); err == nil || !strings.Contains(err.Error(), "at most 1 MiB") || b != nil {
		t.Fatalf("Read at 1 MiB + 1: len=%d err=%v, want size refusal", len(b), err)
	}
}
