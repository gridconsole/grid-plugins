// Package testgit makes real git repos in temp folders for tests, isolated from the
// machine's git configuration. Tests only.
package testgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate skips the test without git, and keeps the machine's git config, signing
// and dates out of the repos a test makes. It uses t.Setenv, so the test is not parallel.
func Isolate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-01T00:00:00Z")
}

// Run runs git in dir and returns its trimmed output.
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.name=Example", "-c", "user.email=dev@example.com"}
	out, err := exec.Command("git", append(base, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Init makes a repo in dir (which must exist). A remote of "" adds none;
// commit true adds one empty commit, which becomes the root commit.
func Init(t *testing.T, dir, remote string, commit bool) {
	t.Helper()
	Run(t, dir, "init", "-q", "-b", "main")
	if remote != "" {
		Run(t, dir, "remote", "add", "origin", remote)
	}
	if commit {
		Run(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	}
}

// TempInRepo points TMPDIR at a folder inside a fresh git repo, as on a machine whose
// temp folder sits in a checkout, and returns that repo. Call it before the test's
// first t.TempDir, which then lands inside the repo.
func TempInRepo(t *testing.T) string {
	t.Helper()
	Isolate(t)
	outer, err := os.MkdirTemp("", "grid-outer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(outer) })
	Init(t, outer, "https://example.com/org/outer.git", true)
	tmp := filepath.Join(outer, "tmp")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	return outer
}
