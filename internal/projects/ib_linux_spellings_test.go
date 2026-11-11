package projects

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

// TestIbFspathProjectsKeepTwoFolders pins what the code does today: on a
// case-sensitive volume Alpha and aLPHA are two folders, so sameFolder and
// within (projects.go) tell them apart and a project in one does not cover the
// other. Whether two such folders are two projects on Linux is an open owner
// question with default yes (backlog-11nov-ib/INDEX.md, Owner questions).
func TestIbFspathProjectsKeepTwoFolders(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ibFsTwoFolders(t, root, "Alpha", "aLPHA")
	upper, lower := filepath.Join(root, "Alpha"), filepath.Join(root, "aLPHA")
	mkdir(t, filepath.Join(upper, "sub"))
	mkdir(t, filepath.Join(lower, "sub"))
	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"sameFolder Alpha, aLPHA", sameFolder(upper, lower), false},
		{"sameFolder aLPHA, Alpha", sameFolder(lower, upper), false},
		{"sameFolder Alpha, Alpha", sameFolder(upper, upper), true},
		{"within Alpha, aLPHA/sub", within(upper, filepath.Join(lower, "sub")), false},
		{"within aLPHA, Alpha/sub", within(lower, filepath.Join(upper, "sub")), false},
		{"within Alpha, Alpha/sub", within(upper, filepath.Join(upper, "sub")), true},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
