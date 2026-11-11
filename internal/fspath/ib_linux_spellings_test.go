package fspath

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// ibFsTwoFolders makes the folders root/first and root/second and returns when
// the volume keeps them as two entries. It skips, never fails, when the volume
// folds case, when it refuses the second name because it normalises it to the
// first (an already-exists error, for example APFS), or when the listing does
// not hold two entries.
func ibFsTwoFolders(t *testing.T, root, first, second string) {
	t.Helper()
	if foldsCase(t, root) {
		t.Skip("volume folds case: two spellings are one folder")
	}
	if err := os.Mkdir(filepath.Join(root, first), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, second), 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			t.Skipf("volume normalises %q to %q: %v", second, first, err)
		}
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Skipf("volume lists %d entries for %q and %q, not two", len(entries), first, second)
	}
}

// ibFsRoot is the symlink-free temp folder the spellings are made in.
func ibFsRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestIbFspathTwoCaseSpellings pins what the code does today: on a
// case-sensitive volume Alpha and aLPHA are two folders and OnDiskCase keeps
// each spelling as given, whichever the directory lists first. Whether two such
// folders are two projects on Linux is an open owner question with default yes
// (backlog-11nov-ib/INDEX.md, Owner questions).
func TestIbFspathTwoCaseSpellings(t *testing.T) {
	root := ibFsRoot(t)
	ibFsTwoFolders(t, root, "Alpha", "aLPHA")
	gamma := filepath.Join(root, "Gamma")
	if err := os.Mkdir(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, "Alpha"),
		filepath.Join(root, "aLPHA"),
		filepath.Join(root, "Alpha", "x"),
		filepath.Join(root, "aLPHA", "x"),
		gamma,
	} {
		if got := OnDiskCase(p); got != p {
			t.Errorf("OnDiskCase(%+q) = %+q, want it unchanged", p, got)
		}
	}
}

// TestIbFspathTwoUnicodeForms pins what the code does today: on a volume that
// stores names as given, café in NFC and in NFD are two folders and, with the
// EqualFold filter off for a non-ASCII name, only os.SameFile keeps them apart.
// Whether two such folders are two projects on Linux is an open owner question
// with default yes (backlog-11nov-ib/INDEX.md, Owner questions).
func TestIbFspathTwoUnicodeForms(t *testing.T) {
	root := ibFsRoot(t)
	nfc, nfd := "café", "café"
	ibFsTwoFolders(t, root, nfc, nfd)
	for _, name := range []string{nfc, nfd} {
		for _, p := range []string{filepath.Join(root, name), filepath.Join(root, name, "x")} {
			if got := OnDiskCase(p); got != p {
				t.Errorf("OnDiskCase(%+q) = %+q, want it byte for byte as given", p, got)
			}
		}
	}
}
