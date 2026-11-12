package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// taskMergeGit runs git in dir and answers its trimmed stdout; a failure carries
// git's own words. Every git call of grid task merge goes through it, so a test
// sees each one.
var taskMergeGit = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		words := strings.Join(strings.Fields(stderr.String()+" "+string(out)), " ")
		return strings.TrimSpace(string(out)), fmt.Errorf("git %s: %s", args[0], words)
	}
	return strings.TrimSpace(string(out)), nil
}

// taskMergeBases are the branches a merge goes into when task.md's base: is not a
// branch of the checkout, in the order grid deliver takes the merge-base from
// (internal/task/deliver.go, recordBranch).
var taskMergeBases = []string{"main", "master"}

// taskMergeID is the T-NN id at the front of a task folder's name.
var taskMergeID = regexp.MustCompile(`^T-\d+`)

// runTaskMerge is grid task merge --task <task>: in the project's checkout it
// merges the branch grid deliver recorded in task.md (branch:) into its base
// branch with a merge commit. It never fetches and never pushes. Who may merge is
// the app's concern, not this verb's.
func runTaskMerge(args []string, env Env) int {
	fs := newFlags("task merge", env)
	asJSON := fs.Bool("json", false, "print JSON")
	id := fs.String("task", "", "task id or folder")
	proj := fs.String("project", "", "project name or id; the task must be in it")
	fs.Usage = func() {
		fmt.Fprint(env.Stderr, taskUsage)
		fmt.Fprintln(env.Stderr, "merge merges the task's branch (task.md branch:) into its base with a merge commit in the project's checkout; it never fetches or pushes.")
		fmt.Fprintln(env.Stderr, "The base is task.md base: when it names a branch, else the first of main and master. The checkout returns to the branch it was on.")
		fs.PrintDefaults()
	}
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 0 || strings.TrimSpace(*id) == "" {
		fmt.Fprintln(env.Stderr, "grid task merge: give the task with --task <task>")
		return 2
	}
	merged, err := taskMerge(env, *id, *proj)
	if err != nil {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid task merge: %s\n", strings.Join(strings.Fields(err.Error()), " "))
		return 1
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			OK     bool   `json:"ok"`
			Merged string `json:"merged"`
		}{true, merged})
	}
	fmt.Fprintf(env.Stdout, "merged %s\n", merged)
	return 0
}

// taskMerge does the merge and answers the commit the base branch now points at.
func taskMerge(env Env, id, proj string) (string, error) {
	if env.GridHome == "" {
		return "", errors.New("cannot find the Grid home; set GRID_HOME")
	}
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: id, Project: proj})
	if err != nil {
		return "", err
	}
	if res.TaskPath == "" {
		return "", fmt.Errorf("task %s not found", id)
	}
	name := res.Task
	if n := taskMergeID.FindString(name); n != "" {
		name = n
	}
	branch, base, err := taskMergeKeys(res.TaskPath)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", fmt.Errorf("task %s has no branch in task.md; run grid deliver in the task's checkout to record it, then run grid task merge again", name)
	}
	repo, err := taskMergeRepo(res, branch)
	if err != nil {
		return "", err
	}
	if st, err := taskMergeGit(repo, "status", "--porcelain"); err != nil {
		return "", err
	} else if st != "" {
		return "", fmt.Errorf("the checkout %s has uncommitted changes; commit or stash them, then run grid task merge again", repo)
	}
	target := taskMergeTarget(repo, base)
	if target == "" {
		return "", fmt.Errorf("no base branch for %s in %s: base %q is not a branch there and neither main nor master exists; record the base with grid deliver --base <branch>, then run grid task merge again", branch, repo, base)
	}
	if target == branch {
		return "", fmt.Errorf("the task's branch %s is its base branch; nothing to merge", branch)
	}
	// A branch already in the base needs no commit; the base's tip holds it.
	if _, err := taskMergeGit(repo, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/heads/"+target); err == nil {
		return taskMergeGit(repo, "rev-parse", "refs/heads/"+target)
	}
	start, err := taskMergeGit(repo, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || start == "" {
		if start, err = taskMergeGit(repo, "rev-parse", "HEAD"); err != nil {
			return "", err
		}
	}
	back := func() error {
		if start == target {
			return nil
		}
		_, err := taskMergeGit(repo, "checkout", "-q", start)
		return err
	}
	if start != target {
		if _, err := taskMergeGit(repo, "checkout", "-q", target); err != nil {
			return "", fmt.Errorf("cannot check out %s in %s (%v); the checkout is as it was", target, repo, err)
		}
	}
	msg := fmt.Sprintf("Merge branch '%s' (%s)", branch, name)
	if _, mergeErr := taskMergeGit(repo, "merge", "--no-ff", "--no-edit", "-m", msg, "refs/heads/"+branch); mergeErr != nil {
		conflicts, _ := taskMergeGit(repo, "diff", "--name-only", "--diff-filter=U")
		if _, err := taskMergeGit(repo, "rev-parse", "-q", "--verify", "MERGE_HEAD"); err == nil {
			if _, err := taskMergeGit(repo, "merge", "--abort"); err != nil {
				return "", fmt.Errorf("merging %s into %s failed and the merge could not be aborted (%v); run git merge --abort in %s", branch, target, err, repo)
			}
		}
		if err := back(); err != nil {
			return "", fmt.Errorf("merging %s into %s failed and %s is still checked out (%v); run git checkout %s in %s", branch, target, target, err, start, repo)
		}
		if conflicts != "" {
			return "", fmt.Errorf("merging %s into %s conflicts in %s; the merge was aborted and the checkout is as it was; merge or rebase %s on %s by hand, then run grid task merge again",
				branch, target, strings.Join(strings.Fields(conflicts), ", "), branch, target)
		}
		return "", fmt.Errorf("merging %s into %s failed (%v); the checkout is as it was; fix that, then run grid task merge again", branch, target, mergeErr)
	}
	merged, err := taskMergeGit(repo, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if err := back(); err != nil {
		return "", fmt.Errorf("merged %s into %s as %s, but %s is still checked out (%v); run git checkout %s in %s", branch, target, merged, target, err, start, repo)
	}
	return merged, nil
}

// taskMergeKeys reads the branch: and base: keys grid deliver wrote to task.md.
func taskMergeKeys(taskPath string) (branch, base string, err error) {
	data, err := os.ReadFile(filepath.Join(taskPath, "task.md"))
	if err != nil {
		return "", "", err
	}
	front, _, err := mdfile.Split(data)
	if err != nil {
		return "", "", fmt.Errorf("task.md: %w", err)
	}
	var keys struct {
		Branch string `yaml:"branch"`
		Base   string `yaml:"base"`
	}
	if err := mdfile.Load(front, &keys, false); err != nil {
		return "", "", fmt.Errorf("task.md: %w", err)
	}
	return strings.TrimSpace(keys.Branch), strings.TrimSpace(keys.Base), nil
}

// taskMergeRepo is the checkout that holds the task's branch: the project's
// folder on this machine, or the one of its repos that has the branch.
func taskMergeRepo(res where.Result, branch string) (string, error) {
	if res.Folder == "" {
		return "", fmt.Errorf("project %s has no folder on this machine; run grid task merge on the machine that has its checkout", res.Project)
	}
	dirs := []string{res.Folder}
	for _, r := range res.Repos {
		if !filepath.IsAbs(r) {
			r = filepath.Join(res.Folder, r)
		}
		if filepath.Clean(r) != filepath.Clean(res.Folder) {
			dirs = append(dirs, r)
		}
	}
	var found []string
	for _, dir := range dirs {
		root, err := taskMergeGit(dir, "rev-parse", "--show-toplevel")
		if err != nil || root == "" {
			continue
		}
		if _, err := taskMergeGit(root, "rev-parse", "-q", "--verify", "refs/heads/"+branch); err == nil && !slices.Contains(found, root) {
			found = append(found, root)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("branch %s is not in the checkout of project %s (%s); run grid task merge where the branch was made, or record it with grid deliver --branch <name>", branch, res.Project, strings.Join(dirs, ", "))
	}
	return "", fmt.Errorf("branch %s is in more than one repo of project %s (%s); merge it there with git", branch, res.Project, strings.Join(found, ", "))
}

// taskMergeTarget is the branch the task's branch goes into: task.md's base when
// it names a branch of the checkout, else the first of main and master there.
func taskMergeTarget(repo, base string) string {
	if base != "" {
		if _, err := taskMergeGit(repo, "rev-parse", "-q", "--verify", "refs/heads/"+base); err == nil {
			return base
		}
	}
	for _, name := range taskMergeBases {
		if _, err := taskMergeGit(repo, "rev-parse", "-q", "--verify", "refs/heads/"+name); err == nil {
			return name
		}
	}
	return ""
}
