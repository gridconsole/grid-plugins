package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestRepoMarkers(t *testing.T) {
	testgit.Isolate(t)
	for _, kind := range []string{"empty marker", "broken pointer", "repository", "worktree"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			switch kind {
			case "empty marker":
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "broken pointer":
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: missing\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "repository":
				testgit.Init(t, dir, "", false)
			case "worktree":
				testgit.Init(t, dir, "", true)
				work := filepath.Join(t.TempDir(), "work")
				testgit.Run(t, dir, "worktree", "add", "--detach", work)
				dir = work
			}
			want := kind == "repository" || kind == "worktree"
			if got := IsRepoRoot(dir); got != want {
				t.Fatalf("IsRepoRoot = %v, want %v", got, want)
			}
		})
	}
}

func TestIsRepoRootSameFolderOtherSpelling(t *testing.T) {
	testgit.Isolate(t)
	parent := t.TempDir()
	repo := filepath.Join(parent, "Repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, repo, "", true)
	inner := filepath.Join(repo, "inner")
	if err := os.Mkdir(inner, 0o700); err != nil {
		t.Fatal(err)
	}
	if IsRepoRoot(inner) {
		t.Fatal("a folder below the root is not a root")
	}
	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(parent, "link")
		if err := os.Symlink(repo, link); err != nil {
			t.Skipf("no symlinks: %v", err)
		}
		if !IsRepoRoot(link) {
			t.Fatal("a symlink to the root is the root")
		}
	})
	t.Run("other case", func(t *testing.T) {
		lower := filepath.Join(parent, "repo")
		a, errA := os.Stat(repo)
		b, errB := os.Stat(lower)
		if errA != nil || errB != nil || !os.SameFile(a, b) {
			t.Skip("case-sensitive file system")
		}
		if !IsRepoRoot(lower) {
			t.Fatal("the same folder spelled in another case is the root")
		}
	})
}
