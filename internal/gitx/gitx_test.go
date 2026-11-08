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
