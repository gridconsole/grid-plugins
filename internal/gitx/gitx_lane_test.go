package gitx

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func mkdirs(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// plainFolder makes a non-repo folder and tells git to look no higher than its
// parent, so a build machine whose temp folder sits in a checkout cannot leak in.
func plainFolder(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	dir := mkdirs(t, filepath.Join(parent, "plain"))
	real, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", real)
	return dir
}

func TestVersion(t *testing.T) {
	testgit.Isolate(t)
	t.Run("git not on PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		got, err := Version()
		if err == nil || !strings.Contains(err.Error(), "git not found") {
			t.Fatalf("Version() = %q, %v; want a git not found error", got, err)
		}
		if got != "" {
			t.Fatalf("Version() = %q, want empty text with the error", got)
		}
	})
	t.Run("real PATH", func(t *testing.T) {
		got, err := Version()
		if err != nil || !strings.HasPrefix(got, "git version") {
			t.Fatalf("Version() = %q, %v; want text starting \"git version\"", got, err)
		}
	})
}

func TestFindRoot(t *testing.T) {
	testgit.Isolate(t)
	repo := t.TempDir()
	testgit.Init(t, repo, "", true)
	deep := mkdirs(t, filepath.Join(repo, "a", "b", "c"))

	t.Run("nested folder finds the repo root", func(t *testing.T) {
		got, ok := FindRoot(deep, repo)
		if !ok || got != repo {
			t.Fatalf("FindRoot(%q, %q) = %q, %v; want %q, true", deep, repo, got, ok, repo)
		}
	})
	t.Run("ceiling below the repo root", func(t *testing.T) {
		ceiling := filepath.Join(repo, "a")
		got, ok := FindRoot(deep, ceiling)
		if ok || got != "" {
			t.Fatalf("FindRoot(%q, %q) = %q, %v; want \"\", false", deep, ceiling, got, ok)
		}
	})
	t.Run("plain folder stops at its ceiling", func(t *testing.T) {
		x := t.TempDir()
		y := mkdirs(t, filepath.Join(x, "y"))
		got, ok := FindRoot(y, x)
		if ok || got != "" {
			t.Fatalf("FindRoot(%q, %q) = %q, %v; want \"\", false", y, x, got, ok)
		}
	})
	t.Run("empty ceiling walks up to the repo root", func(t *testing.T) {
		got, ok := FindRoot(deep, "")
		if !ok || got != repo {
			t.Fatalf("FindRoot(%q, \"\") = %q, %v; want %q, true", deep, got, ok, repo)
		}
	})
	t.Run("linked worktree subfolder gives the worktree root", func(t *testing.T) {
		work := filepath.Join(t.TempDir(), "work")
		testgit.Run(t, repo, "worktree", "add", "--detach", work)
		sub := mkdirs(t, filepath.Join(work, "sub", "deeper"))
		got, ok := FindRoot(sub, "")
		if !ok || got != work {
			t.Fatalf("FindRoot(%q, \"\") = %q, %v; want worktree root %q, true", sub, got, ok, work)
		}
		if got == repo {
			t.Fatal("got the main repo, not the worktree")
		}
	})
}

func TestFindRootWalksToVolumeRoot(t *testing.T) {
	testgit.Isolate(t)
	// With an empty ceiling and no repo anywhere, the walk must end at the volume
	// root instead of looping. A ceiling for git keeps any checkout above the temp
	// folder out of the answer, and a missing .git is checked first by IsRepoRoot.
	plain := plainFolder(t)
	dir := mkdirs(t, filepath.Join(plain, "p", "q"))
	got, ok := FindRoot(dir, plain)
	if ok || got != "" {
		t.Fatalf("FindRoot = %q, %v; want \"\", false", got, ok)
	}
	// The volume root itself is the end of the walk for any dir.
	vol := filepath.VolumeName(dir) + string(filepath.Separator)
	if _, ok := FindRoot(vol, ""); ok {
		t.Skipf("the volume root %q is itself a repository", vol)
	}
}

func TestCommonDir(t *testing.T) {
	testgit.Isolate(t)
	repo := t.TempDir()
	testgit.Init(t, repo, "", true)
	work := filepath.Join(t.TempDir(), "work")
	testgit.Run(t, repo, "worktree", "add", "--detach", work)
	want := gridhome.Real(filepath.Join(repo, ".git"))

	for name, root := range map[string]string{"repo": repo, "worktree": work} {
		t.Run(name, func(t *testing.T) {
			got, err := CommonDir(root)
			if err != nil {
				t.Fatalf("CommonDir(%q): %v", root, err)
			}
			if got != want {
				t.Fatalf("CommonDir(%q) = %q, want %q", root, got, want)
			}
			if !filepath.IsAbs(got) {
				t.Fatalf("CommonDir(%q) = %q, not absolute", root, got)
			}
			if got != filepath.Clean(got) {
				t.Fatalf("CommonDir(%q) = %q, not in filepath form", root, got)
			}
			if runtime.GOOS == "windows" && strings.Contains(got, "/") {
				t.Fatalf("CommonDir(%q) = %q, has a forward slash on Windows", root, got)
			}
		})
	}
	t.Run("non-repo", func(t *testing.T) {
		dir := plainFolder(t)
		got, err := CommonDir(dir)
		if err == nil {
			t.Fatalf("CommonDir(%q) = %q, want an error", dir, got)
		}
	})
}

func TestRemotes(t *testing.T) {
	testgit.Isolate(t)
	t.Run("origin and upstream", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "https://example.com/org/repo.git", false)
		testgit.Run(t, repo, "remote", "add", "upstream", "git@example.com:up/repo.git")
		got := Remotes(repo)
		want := []string{"https://example.com/org/repo.git", "git@example.com:up/repo.git"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Remotes = %q, want %q", got, want)
		}
	})
	t.Run("empty value is skipped", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "https://example.com/org/repo.git", false)
		testgit.Run(t, repo, "config", "remote.blank.url", "")
		testgit.Run(t, repo, "remote", "add", "upstream", "https://example.com/up/repo.git")
		got := Remotes(repo)
		want := []string{"https://example.com/org/repo.git", "https://example.com/up/repo.git"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Remotes = %q, want %q", got, want)
		}
	})
	t.Run("no remotes", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "", false)
		if got := Remotes(repo); got != nil {
			t.Fatalf("Remotes = %q, want nil", got)
		}
	})
	t.Run("non-repo", func(t *testing.T) {
		dir := plainFolder(t)
		if got := Remotes(dir); got != nil {
			t.Fatalf("Remotes = %q, want nil", got)
		}
	})
	t.Run("Windows path comes back as written", func(t *testing.T) {
		repo := t.TempDir()
		url := `C:\repos\x`
		testgit.Init(t, repo, url, false)
		got := Remotes(repo)
		if !reflect.DeepEqual(got, []string{url}) {
			t.Fatalf("Remotes = %q, want %q", got, []string{url})
		}
	})
}

func TestRootCommit(t *testing.T) {
	testgit.Isolate(t)
	t.Run("empty repo", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "", false)
		if got := RootCommit(repo); got != "" {
			t.Fatalf("RootCommit = %q, want \"\"", got)
		}
	})
	t.Run("one commit", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "", true)
		want := testgit.Run(t, repo, "rev-parse", "HEAD")
		if got := RootCommit(repo); got != want {
			t.Fatalf("RootCommit = %q, want %q", got, want)
		}
	})
	t.Run("two root commits give the smaller sha", func(t *testing.T) {
		repo := t.TempDir()
		testgit.Init(t, repo, "", true)
		first := testgit.Run(t, repo, "rev-parse", "HEAD")
		testgit.Run(t, repo, "checkout", "-q", "--orphan", "other")
		// Dates are pinned, so the message alone makes this a second object.
		testgit.Run(t, repo, "commit", "-q", "--allow-empty", "-m", "second root")
		second := testgit.Run(t, repo, "rev-parse", "HEAD")
		testgit.Run(t, repo, "checkout", "-q", "main")
		testgit.Run(t, repo, "merge", "-q", "--allow-unrelated-histories", "-m", "join", "other")
		if first == second {
			t.Fatal("the two root commits are one object")
		}
		roots := strings.Fields(testgit.Run(t, repo, "rev-list", "--max-parents=0", "HEAD"))
		if len(roots) != 2 {
			t.Fatalf("fixture has %d root commits, want 2: %q", len(roots), roots)
		}
		want := min(first, second)
		if got := RootCommit(repo); got != want {
			t.Fatalf("RootCommit = %q, want the smaller of %q and %q", got, first, second)
		}
	})
}
