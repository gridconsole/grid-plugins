package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/runner"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const taskUsage = `Usage:
  grid task new "<words>" [--project <name-or-id>] [--size auto|small|normal|large] [--workflow <name>] [--kind setup] [--input <file>]... [--json]
  grid task show [<task>] [--project <name-or-id>] [--json]
  grid task attach <task> [--session ID] [--runtime claude] [--json]
  grid task list [--project <name-or-id>] [--state <state>] [--json]
  grid task move <machine> [--task <task>] [--project <project>] [--who <caller>] [--json]
  grid task stop|close|reopen <task> [--reason <text>] [--json]
  grid task rename <task> --title <text> [--project <p>] [--json]
  grid task decline <task> --reason <text> [--project <p>] [--json]
  grid task summary <task> --for owner|developer|customer --text <words> [--json]
  grid task delete <task> [--project <p>] [--json]
  grid task restore <T-NN> --project <p> [--json]
  grid task status <task> <state> [--json]
  grid task lane <task> <inbox|planning|doing|delivering> [--project <p>] [--json]
  grid task history <task> [--project <p>] [--json]
  grid task merge --task <task> [--project <p>] [--json]

<task> is an id, exact folder name, or folder path inside a project tasks directory.
`

// runTaskSummary is grid task summary: it writes one reader level's words into the
// task's summary.md, which the task page reads. It resolves the task as rename does
// and answers with the same exit codes: usage 2, rejected 3, anything else 1.
func runTaskSummary(args []string, env Env) int {
	fs := newFlags("task summary", env)
	asJSON := fs.Bool("json", false, "print JSON")
	reader := fs.String("for", "", "reader level: owner, developer or customer")
	text := fs.String("text", "", "the summary words; customer words are plain, with no Job ids")
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid task summary", fmt.Errorf("scope: %w", err))
	}
	o := task.SummaryOptions{WriteOptions: writeOptions(env, resolved.ID, "summary", ""), Reader: *reader, Text: *text,
		Role: strings.TrimSpace(env.getenv("GRID_ROLE"))}
	event, err := taskWriter(env).Summary(o)
	if err != nil {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid task summary: %v\n", err)
		switch {
		case errors.Is(err, task.ErrEditUsage):
			return 2
		case errors.Is(err, task.ErrEditRejected):
			return 3
		default:
			return 1
		}
	}
	return taskWriteResult(env, event, nil, *asJSON, resolved.Path)
}

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
		fmt.Fprint(fs.Output(), taskUsage)
		fmt.Fprintln(fs.Output(), "merge merges the task's branch (task.md branch:) into its base with a merge commit in the project's checkout; it never fetches or pushes.")
		fmt.Fprintln(fs.Output(), "The base is task.md base: when it names a branch, else the first of main and master. The checkout returns to the branch it was on.")
		fs.PrintDefaults()
	}
	helpToStdout(fs, args, env)
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

// mergeConflictError is a merge of the task's branch into its base that stopped on
// conflicts and was aborted. Target is the branch it was merged into, not the SHA
// task.md base: holds, so a rebase Job can name the branch to rebase on.
type mergeConflictError struct {
	Branch, Target string
	TargetSHA      string   // the commit Target pointed at when the merge stopped, "" when unread
	Files          []string // the files that conflict
	Changed        []string // every file the branch changed since it left Target, conflicting or not
	Repo           string   // the repo a Job names for the checkout, "" in a project of one repo
}

// jobRepo is the repo a Job names for the checkout repo: its path in the project's
// folder, and "" when the project has one repo, which a Job leaves out.
func jobRepo(res where.Result, repo string) string {
	if len(res.Repos) < 2 || res.Folder == "" {
		return ""
	}
	rel, err := filepath.Rel(res.Folder, repo)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (e *mergeConflictError) Error() string {
	return fmt.Sprintf("merging %s into %s conflicts in %s; the merge was aborted and the checkout is as it was; merge or rebase %s on %s by hand, then run grid task merge again",
		e.Branch, e.Target, strings.Join(e.Files, ", "), e.Branch, e.Target)
}

// taskMerge does the merge and answers the commit the base branch now points at.
func taskMerge(env Env, id, proj string) (string, error) {
	if env.GridHome == "" {
		return "", errors.New("cannot find the Grid home; set GRID_HOME")
	}
	if proj == "" {
		// The task's record names its project, as grid land reads it; the folder this
		// runs in does not. A task not found that way keeps the folder's project.
		if t, err := resolveTaskOperand(env, id); err == nil {
			proj = t.Project
		}
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
	var missing *branchMissingError
	var repoErr error
	if branch != "" {
		_, repoErr = taskMergeRepo(res, branch)
	}
	if branch != "" && !errors.As(repoErr, &missing) {
		sha, _, err := taskMergeOne(res, name, branch, base)
		return sha, err
	}
	// A branch no repo holds is the delivering Job's own: the work is on the
	// branches of the task's accepted builder Jobs, landed in Feed order. A rebased
	// branch of one of them lands with the rest, in its original's place.
	built, rebased, noWrite, err := builderBranches(res.TaskPath)
	if err != nil {
		return "", err
	}
	// A project with no git checkout here has no branch to land, as before builder branches.
	if len(built) == 0 || branch == "" && !hasCheckout(res) {
		if branch == "" {
			return "", noTaskBranchError(name)
		}
		return "", repoErr
	}
	merged := ""
	var skipped []string
	for _, b := range built {
		sha, landed, err := taskMergeOne(res, name, b, base)
		var conflict *mergeConflictError
		if errors.As(err, &conflict) {
			if r := rebasedOf(res, b, rebased); r != "" {
				sha, landed, err = taskMergeOne(res, name, r, base)
			}
		}
		if noWork(err, b, noWrite) {
			fmt.Fprintf(env.Stderr, "grid task merge: skipped %s: no repo of project %s holds it, and the accepted builder Job has no files to write, so it has nothing to land\n", b, res.Project)
			skipped = append(skipped, b)
			continue
		}
		if err != nil {
			return "", err
		}
		if landed {
			merged = sha
		}
	}
	// task.md names a branch no repo holds and no builder branch stands in for it.
	if branch != "" && len(skipped) == len(built) {
		return "", repoErr
	}
	// The branches are landed by now, so a history line that fails is told on stderr only.
	for _, b := range skipped {
		line := fmt.Sprintf("grid task merge skipped %s: no repo of project %s holds it and its builder Job has no files to write", b, res.Project)
		tasks := taskWriter(env)
		tasks.Project = res.Project
		if _, err := tasks.Note(writeOptions(env, name, "note", line)); err != nil {
			fmt.Fprintf(env.Stderr, "grid task merge: the skip of %s is not in the task's history: %s\n", b, oneLineErr(err))
		}
	}
	return merged, nil
}

// rebasedOf is the newest accepted rebase branch in the repo of branch that holds
// branch, as a rebase Job merges its delivered branch in; "" when none does.
func rebasedOf(res where.Result, branch string, rebased []string) string {
	repo, err := taskMergeRepo(res, branch)
	if err != nil {
		return ""
	}
	for i := len(rebased) - 1; i >= 0; i-- {
		if r, err := taskMergeRepo(res, rebased[i]); err == nil && r == repo {
			if _, err := taskMergeGit(repo, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/heads/"+rebased[i]); err == nil {
				return rebased[i]
			}
		}
	}
	return ""
}

// firstUnlanded is the first builder branch of the task, in Feed order, that is not
// in its target yet, by itself or by its rebased branch, and the repo that holds it:
// the branch a land stops on.
func firstUnlanded(res where.Result, base string, built, rebased, noWrite []string) (repo, branch string, err error) {
	in := func(repo, b, target string) bool {
		_, err := taskMergeGit(repo, "merge-base", "--is-ancestor", "refs/heads/"+b, "refs/heads/"+target)
		return err == nil
	}
	for _, b := range built {
		repo, err := taskMergeRepo(res, b)
		if noWork(err, b, noWrite) {
			continue // as taskMerge skips it
		}
		if err != nil {
			return "", "", err
		}
		target := taskMergeTarget(repo, base)
		if target == "" || in(repo, b, target) {
			continue
		}
		if r := rebasedOf(res, b, rebased); r != "" && in(repo, r, target) {
			continue
		}
		return repo, b, nil
	}
	return "", "", errAllLanded
}

// errAllLanded is a task whose every builder branch is in its target already.
var errAllLanded = errors.New("every builder branch of the task is in its target")

// noTaskBranchError is a task with no branch in task.md and no accepted builder Job:
// grid task merge has nothing to merge, and a land has nothing to land.
type noTaskBranchError string

func (e noTaskBranchError) Error() string {
	return fmt.Sprintf("task %s has no branch in task.md; run grid deliver in the task's checkout to record it, then run grid task merge again", string(e))
}

// noWork reports whether err says builder branch b is in no repo and its Job has an
// empty write list (noWrite holds those branches, as builderBranches reads them): a Job
// that only wrote brain proposals made no branch and has nothing to land. A Job that was
// to write files and has no branch lost its work, so that stays an error.
func noWork(err error, b string, noWrite []string) bool {
	var missing *branchMissingError
	return errors.As(err, &missing) && slices.Contains(noWrite, b)
}

// builderBranches is the branch of each accepted builder Job of the task, in Feed
// order, named as the worker names an attempt's branch; rebased is the branches of
// its accepted rebase Jobs, which stand in for the branch they rebased, and noWrite
// the branches in built whose Job has an empty write list.
func builderBranches(taskPath string) (built, rebased, noWrite []string, err error) {
	fid, _, err := task.FeedIdentity(taskPath)
	if err != nil {
		return nil, nil, nil, err
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: taskPath, TaskID: fid, Warn: io.Discard})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, it := range byJobNumber(snap.Items) {
		j := snap.State.Jobs[it.ID]
		if j == nil || j.Status != contract.Accepted || contract.NormalizeRole(j.Role) != "builder" {
			continue
		}
		b := runner.Branch(runner.Spawn{Task: fid, ID: it.ID, Attempt: max(j.Attempt, 1)})
		if isRebaseJob(it) {
			rebased = append(rebased, b)
		} else {
			built = append(built, b)
			if len(it.Write) == 0 {
				noWrite = append(noWrite, b)
			}
		}
	}
	return built, rebased, noWrite, nil
}

// taskMergeOne merges branch into its base in the repo that holds it and answers the
// commit the base points at after; landed is false when the base already held it.
func taskMergeOne(res where.Result, name, branch, base string) (string, bool, error) {
	repo, err := taskMergeRepo(res, branch)
	if err != nil {
		return "", false, err
	}
	if st, err := taskMergeGit(repo, "status", "--porcelain"); err != nil {
		return "", false, err
	} else if st != "" {
		return "", false, fmt.Errorf("the checkout %s has uncommitted changes; commit or stash them, then run grid task merge again", repo)
	}
	target := taskMergeTarget(repo, base)
	if target == "" {
		return "", false, fmt.Errorf("no base branch for %s in %s: base %q is not a branch there and neither main nor master exists; record the base with grid deliver --base <branch>, then run grid task merge again", branch, repo, base)
	}
	if target == branch {
		return "", false, fmt.Errorf("the task's branch %s is its base branch; nothing to merge", branch)
	}
	// A branch already in the base needs no commit; the base's tip holds it.
	if _, err := taskMergeGit(repo, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/heads/"+target); err == nil {
		tip, err := taskMergeGit(repo, "rev-parse", "refs/heads/"+target)
		return tip, false, err
	}
	start, err := taskMergeGit(repo, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || start == "" {
		if start, err = taskMergeGit(repo, "rev-parse", "HEAD"); err != nil {
			return "", false, err
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
			return "", false, fmt.Errorf("cannot check out %s in %s (%v); the checkout is as it was", target, repo, err)
		}
	}
	msg := fmt.Sprintf("Merge branch '%s' (%s)", branch, name)
	if _, mergeErr := taskMergeGit(repo, "merge", "--no-ff", "--no-edit", "-m", msg, "refs/heads/"+branch); mergeErr != nil {
		conflicts, _ := taskMergeGit(repo, "diff", "--name-only", "--diff-filter=U")
		if _, err := taskMergeGit(repo, "rev-parse", "-q", "--verify", "MERGE_HEAD"); err == nil {
			if _, err := taskMergeGit(repo, "merge", "--abort"); err != nil {
				return "", false, fmt.Errorf("merging %s into %s failed and the merge could not be aborted (%v); run git merge --abort in %s", branch, target, err, repo)
			}
		}
		if err := back(); err != nil {
			return "", false, fmt.Errorf("merging %s into %s failed and %s is still checked out (%v); run git checkout %s in %s", branch, target, target, err, start, repo)
		}
		if conflicts != "" {
			changed, _ := taskMergeGit(repo, "diff", "--name-only", target+"..."+branch)
			tip, _ := taskMergeGit(repo, "rev-parse", "refs/heads/"+target)
			return "", false, &mergeConflictError{Branch: branch, Target: target, TargetSHA: tip, Files: strings.Fields(conflicts), Changed: strings.Fields(changed), Repo: jobRepo(res, repo)}
		}
		return "", false, fmt.Errorf("merging %s into %s failed (%v); the checkout is as it was; fix that, then run grid task merge again", branch, target, mergeErr)
	}
	merged, err := taskMergeGit(repo, "rev-parse", "HEAD")
	if err != nil {
		return "", false, err
	}
	if err := back(); err != nil {
		return "", false, fmt.Errorf("merged %s into %s as %s, but %s is still checked out (%v); run git checkout %s in %s", branch, target, merged, target, err, start, repo)
	}
	return merged, true, nil
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
	dirs := taskMergeDirs(res)
	var found []string
	for _, dir := range dirs {
		root, err := taskMergeGit(dir, "rev-parse", "--show-toplevel")
		if err != nil || root == "" {
			continue
		}
		if localBranch(root, branch) && !slices.Contains(found, root) {
			found = append(found, root)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", &branchMissingError{fmt.Sprintf("branch %s is not in the checkout of project %s (%s); run grid task merge where the branch was made, or record it with grid deliver --branch <name>", branch, res.Project, strings.Join(dirs, ", "))}
	}
	return "", fmt.Errorf("branch %s is in more than one repo of project %s (%s); merge it there with git", branch, res.Project, strings.Join(found, ", "))
}

// taskMergeDirs is the project's folder on this machine and the folders of its repos.
func taskMergeDirs(res where.Result) []string {
	if res.Folder == "" {
		return nil
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
	return dirs
}

// hasCheckout reports whether the project has a git checkout on this machine.
func hasCheckout(res where.Result) bool {
	for _, dir := range taskMergeDirs(res) {
		if root, err := taskMergeGit(dir, "rev-parse", "--show-toplevel"); err == nil && root != "" {
			return true
		}
	}
	return false
}

// localBranch reports whether name, from task data, is a branch name and a local
// branch of repo: rev-parse --verify would also take refs/heads/main~1.
func localBranch(repo, name string) bool {
	if name == "" {
		return false
	}
	if _, err := taskMergeGit(repo, "check-ref-format", "--branch", name); err != nil {
		return false
	}
	_, err := taskMergeGit(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// branchMissingError is a branch that no checkout of the project holds.
type branchMissingError struct{ text string }

func (e *branchMissingError) Error() string { return e.text }

// taskMergeTarget is the branch the task's branch goes into: task.md's base when
// it names a branch of the checkout, else the first of main and master there.
func taskMergeTarget(repo, base string) string {
	if localBranch(repo, base) {
		return base
	}
	for _, name := range taskMergeBases {
		if localBranch(repo, name) {
			return name
		}
	}
	return ""
}

// inputList is a repeatable string flag: each --input adds one path.
type inputList []string

func (l *inputList) String() string { return strings.Join(*l, ",") }

func (l *inputList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

func runTask(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(env.Stdout, taskUsage)
		return 0
	}
	sub := args[0]
	if sub == "move" {
		return runTaskIntent(args, env)
	}
	switch sub {
	case "delete":
		return runTaskTrash(args, env)
	case "restore":
		return runTaskTrash(args, env)
	case "attach":
		return runTaskAttach(args[1:], env)
	case "rename":
		return runTaskWrite(args, env)
	case "decline":
		return runTaskWrite(args, env)
	case "summary":
		return runTaskSummary(args, env)
	case "stop", "close", "reopen", "status", "history":
		return runTaskWrite(args, env)
	case "lane":
		return runTaskLane(args[1:], env)
	case "merge":
		return runTaskMerge(args[1:], env)
	}
	if sub != "new" && sub != "show" && sub != "list" {
		fmt.Fprintf(env.Stderr, "grid task: unknown command %q\n%s", sub, taskUsage)
		return 2
	}
	fs := newFlags("task "+sub, env)
	asJSON := fs.Bool("json", false, "print JSON")
	var proj, size, workflow, state, kind string
	var inputs inputList
	if sub == "new" || sub == "list" {
		fs.StringVar(&proj, "project", "", "project name or id; personal selects Personal")
	}
	if sub == "show" {
		fs.StringVar(&proj, "project", "", "project name or id; look only in it")
	}
	if sub == "new" {
		fs.StringVar(&size, "size", "auto", "auto, small, normal or large")
		fs.StringVar(&workflow, "workflow", "", "workflow name; omitted leaves selection to the rules")
		fs.StringVar(&kind, "kind", "", "setup marks the project's setup task and gives its first Job the setup skill; omitted makes a plain task")
		fs.Var(&inputs, "input", "file to copy into the task's inputs folder; repeat for more files")
	}
	if sub == "list" {
		fs.StringVar(&state, "state", "", "include only tasks in this state")
	}
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), taskUsage)
		fs.PrintDefaults()
	}
	helpToStdout(fs, args, env)
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if (sub == "new" && len(pos) != 1) || (sub == "show" && len(pos) > 1) || (sub == "list" && len(pos) != 0) {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	store := task.Store{Resolver: resolver(env)}
	fail := func(err error) int {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid task %s: %v\n", sub, err)
		return 1
	}
	switch sub {
	case "new":
		// GRID_ACTOR is what the other task writers read, so a host that sets only it still names the owner.
		who := env.getenv("GRID_WHO")
		if who == "" {
			who = env.getenv("GRID_ACTOR")
		}
		o := task.NewOptions{Cwd: env.Cwd, Words: pos[0], Project: proj, Size: size, Workflow: workflow, Kind: kind,
			Who: who, Role: env.getenv("GRID_ROLE"),
			// The raw field, not env.getenv: nil keeps the host read of the binary, a lib Call reads its Environ alone.
			Getenv: env.Getenv}
		for _, in := range inputs {
			if !filepath.IsAbs(in) {
				in = filepath.Join(env.Cwd, in)
			}
			o.Inputs = append(o.Inputs, in)
		}
		if err := o.Validate(); err != nil {
			fmt.Fprintf(env.Stderr, "grid task new: %v\n", err)
			return 2
		}
		t, err := store.New(o)
		if err != nil {
			return fail(err)
		}
		// The task exists once New returns; its first Job is posted after, and a
		// failure to post it is said out loud, never left as a task nothing starts.
		postErr := postFirstJob(env, t, o)
		if *asJSON {
			code := writeJSON(env.Stdout, struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			}{t.ID, t.Path})
			if code == 0 && postErr != nil {
				return fail(fmt.Errorf("%s was created but its first Job was not posted: %w", t.ID, postErr))
			}
			return code
		}
		fmt.Fprintf(env.Stdout, "%s created\n%s\n", t.ID, gridhome.Tilde(t.Path, env.Home))
		if postErr != nil {
			return fail(fmt.Errorf("%s was created but its first Job was not posted: %w", t.ID, postErr))
		}
	case "show":
		id := ""
		if len(pos) == 1 {
			id = pos[0]
		}
		t, err := resolveTaskOperandIn(env, id, proj)
		if err != nil {
			return taskOperandError(env, "grid task show", err)
		}
		// The hash a person passes to grid approve --plan-hash; absent before a plan exists.
		hash, digest, err := taskPlanTokens(t.Path)
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid task show: no plan hash: %v\n", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				task.Task
				PlanHash   string `json:"plan_hash,omitempty"`
				PlanDigest string `json:"plan_digest,omitempty"`
				Folder     string `json:"folder"`
			}{t, hash, digest, filepath.Base(t.Path)})
		}
		fmt.Fprintf(env.Stdout, "id: %s\ntitle: %s\nproject: %s\nsize: %s\nworkflow: %s\nstate: %s\ncreated: %s\npath: %s\n",
			t.ID, t.Title, t.Project, t.Size, t.Workflow, t.State, t.Created, gridhome.Tilde(t.Path, env.Home))
		if hash != "" {
			fmt.Fprintf(env.Stdout, "plan_hash: %s\nplan digest: %s\n", hash, digest)
		}
		fmt.Fprintf(env.Stdout, "intent:\n%s", t.Intent)
		if !strings.HasSuffix(t.Intent, "\n") {
			fmt.Fprintln(env.Stdout)
		}
	case "list":
		if states := task.States(); state != "" && !slices.Contains(states, state) {
			fmt.Fprintf(env.Stderr, "grid task list: %q is not a task state; use one of: %s\n", state, strings.Join(states, ", "))
			return 2
		}
		tasks, err := store.List(env.Cwd, proj, state)
		if err != nil {
			return fail(err)
		}
		for _, t := range tasks {
			if t.Unreadable {
				fmt.Fprintf(env.Stderr, "grid task list: warning: task folder %q is unreadable\n", filepath.Base(t.Path))
			}
		}
		if *asJSON {
			return writeJSON(env.Stdout, tasks)
		}
		for _, t := range tasks {
			if t.Unreadable {
				continue
			}
			fmt.Fprintf(env.Stdout, "%s %s %s\n", t.ID, t.State, t.Title)
		}
	}
	return 0
}

func runTaskAttach(args []string, env Env) int {
	fs := newFlags("task attach", env)
	id := fs.String("session", "", "session id; defaults to GRID_SESSION_ID, then CLAUDE_SESSION_ID, then a generated id")
	runtime := fs.String("runtime", "claude", "session runtime (claude)")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	if *runtime != "claude" {
		fmt.Fprintln(env.Stderr, "grid task attach: --runtime must be claude")
		return 2
	}
	if *id == "" {
		*id = env.getenv("GRID_SESSION_ID")
	}
	if *id == "" {
		*id = env.getenv("CLAUDE_SESSION_ID")
	}
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid task attach", err)
	}
	pid, pidStart := os.Getppid(), ""
	if env.OwnerPID != 0 {
		pid = env.OwnerPID
		if !env.OwnerStart.IsZero() {
			pidStart = contract.FormatTime(env.OwnerStart)
		}
	}
	if pidStart == "" {
		pidStart = migrate.ProcessStart(pid)
	}
	result, err := taskWriter(env).Attach(task.AttachOptions{
		Cwd: env.Cwd, ID: resolved.ID, SessionID: *id, Runtime: *runtime,
		PID: pid, PIDStart: pidStart, Who: writeOptions(env, pos[0], "attach", "").Who,
		Agent: agentShell(env), BoundTask: env.getenv("GRID_TASK"),
	})
	if err != nil {
		if errors.Is(err, task.ErrEditUsage) {
			fmt.Fprintf(env.Stderr, "grid task attach: %v\n", err)
			return 2
		}
		return commandError(env, "grid task attach", err)
	}
	advice := env.Stdout
	if *asJSON {
		advice = env.Stderr
	}
	for _, holder := range result.Holders {
		fmt.Fprintf(advice, "Live holder: %s. Use grid lease take <task> --takeover --expected-epoch <gen.fence> to take over; attach does not change leases.\n", holder)
	}
	if *asJSON {
		return writeJSON(env.Stdout, result)
	}
	fmt.Fprintf(env.Stdout, "id: %s\ntitle: %s\nstate: %s\napproved: %t\nopen_questions: %d\nAttached. %s %s\n",
		result.ID, result.Title, result.State, result.Approved, result.OpenQuestions, result.ID, result.Title)
	return 0
}

// taskPlanTokens returns the hash and digest from one plan.md snapshot, or empty
// tokens when the task has no plan yet. Reading takes no lock: plan.md is replaced atomically.
func taskPlanTokens(dir string) (string, string, error) {
	text, err := plan.Store{TaskDir: dir}.Read()
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return plan.Hash(text), plan.Digest(text), nil
}

// taskArgs accepts flags before or after the single words/id argument. Values
// belonging to string flags stay attached; -- protects words starting with '-'.
func taskArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			pos = append(pos, arg)
			continue
		}
		flags = append(flags, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil || hasValue {
			continue // flag.Parse reports unknown flags and handles --help.
		}
		b, isBool := f.Value.(interface{ IsBoolFlag() bool })
		if (!isBool || !b.IsBoolFlag()) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return pos, fs.Parse(flags)
}
