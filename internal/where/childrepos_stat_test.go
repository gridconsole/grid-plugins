package where

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestChildReposStatNoGit: childRepos decides a repo by a stat of <dir>/.git, so 200
// child repos are found in under a second with no git on PATH.
func TestChildReposStatNoGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	var want []string
	for i := 0; i < 200; i++ {
		// Spread them over both levels: root/a-N and root/group-M/b-N.
		p := filepath.Join(root, fmt.Sprintf("r%03d", i))
		if i%2 == 1 {
			p = filepath.Join(root, fmt.Sprintf("group%d", i%10), fmt.Sprintf("r%03d", i))
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if i%4 == 3 {
			// worktree / submodule style pointer file
			if err := os.WriteFile(filepath.Join(p, ".git"), []byte("gitdir: /elsewhere/.git/worktrees/x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Mkdir(filepath.Join(p, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		want = append(want, p)
	}
	// Not repos: no marker, a .git file without a gitdir line, an empty file.
	for name, content := range map[string]*string{"plain": nil, "junk": ptr("hello\n"), "empty": ptr("")} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if content != nil {
			if err := os.WriteFile(filepath.Join(p, ".git"), []byte(*content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Three levels down is out of reach.
	deep := filepath.Join(root, "x", "y", "z")
	if err := os.MkdirAll(filepath.Join(deep, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sort.Strings(want)

	start := time.Now()
	got := childRepos(root)
	took := time.Since(start)

	if took > time.Second {
		t.Errorf("childRepos took %v, want under 1s", took)
	}
	if len(got) != len(want) {
		t.Fatalf("childRepos found %d repos, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("repo %d = %s, want %s (order is sorted)", i, got[i], want[i])
		}
	}
}

func ptr(s string) *string { return &s }
