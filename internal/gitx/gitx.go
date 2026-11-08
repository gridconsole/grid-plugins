// Package gitx wraps the few git facts project resolution needs. Every function
// degrades to "no answer" when git is missing or the folder is not a repo.
package gitx

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func run(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Version returns the output of git --version.
func Version() (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New("git not found on PATH")
	}
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsRepoRoot verifies a .git marker with Git; an empty folder or dangling
// worktree pointer is not a repository.
func IsRepoRoot(dir string) bool {
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	root, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil || root == "" {
		return false
	}
	// SameFile, not a string compare: Git may report another case on a
	// case-insensitive file system, or the real path behind a symlink.
	rootInfo, rootErr := os.Stat(root)
	dirInfo, dirErr := os.Stat(dir)
	return rootErr == nil && dirErr == nil && os.SameFile(rootInfo, dirInfo)
}

// FindRoot returns the nearest folder at or above dir that is a repo root. The walk
// stops at ceiling when dir is below it; "" walks to the filesystem root.
func FindRoot(dir, ceiling string) (string, bool) {
	for d := dir; ; d = filepath.Dir(d) {
		if IsRepoRoot(d) {
			return d, true
		}
		if d == ceiling || filepath.Dir(d) == d {
			return "", false
		}
	}
}

// CommonDir returns the main repo's git folder, shared by the repo and all its worktrees.
func CommonDir(root string) (string, error) {
	out, err := run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return gridhome.Real(out), nil
}

// Remotes returns the configured remote URLs, as written.
func Remotes(root string) []string {
	out, err := run(root, "config", "--get-regexp", `^remote\..*\.url$`)
	if err != nil {
		return nil
	}
	var urls []string
	for _, line := range strings.Split(out, "\n") {
		if _, v, ok := strings.Cut(line, " "); ok && strings.TrimSpace(v) != "" {
			urls = append(urls, strings.TrimSpace(v))
		}
	}
	return urls
}

// RootCommit returns the first root commit of HEAD, or "" with no commits.
func RootCommit(root string) string {
	out, err := run(root, "rev-list", "--max-parents=0", "HEAD")
	if err != nil || out == "" {
		return ""
	}
	roots := strings.Fields(out)
	sort.Strings(roots)
	return roots[0]
}
