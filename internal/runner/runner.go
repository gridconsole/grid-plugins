// Package runner starts one agent per Job the task holder was assigned. It reads the
// spawn lines the stream prints (orchestrator.Stream), makes a git worktree for the
// attempt, picks an account (a Claude config folder) from its windows, starts the
// agent with the Job's rendered brief, and posts the ack and the worker-exit on the
// task's Feed. It adds no cap, delay or rate hold on Jobs: every spawn starts at once.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gridconsole/grid-plugins/internal/account"
	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// Spawn is one spawn line of the stream: a Job assigned to the holder.
type Spawn struct {
	Task      string   `json:"task"`
	TaskDir   string   `json:"task_dir"`
	ID        string   `json:"id"`
	Holder    string   `json:"holder"`
	Fence     int64    `json:"fence"`
	Gen       int64    `json:"gen"`
	Attempt   int      `json:"attempt"`
	Assign    string   `json:"assign"`
	TaskEpoch string   `json:"task_epoch"`
	Model     string   `json:"model,omitempty"`
	Effort    string   `json:"effort,omitempty"`
	Tools     []string `json:"tools,omitempty"`
	Repo      string   `json:"repo,omitempty"`
}

// Account is one account the runner may start an agent under: a Claude config folder
// and the windows `grid account windows` reads for it. Default is the account of
// ~/.claude, which leaves CLAUDE_CONFIG_DIR out.
type Account struct {
	ID      string
	Dir     string
	Default bool
	Windows account.AccountWindows
}

// Attempt is the runner's record of one attempt, written beside its log before the
// agent starts and again when it ends.
type Attempt struct {
	Task      string `json:"task"`
	Job       string `json:"job"`
	Attempt   int    `json:"attempt"`
	Fence     int64  `json:"fence"`
	Branch    string `json:"branch"`
	Worktree  string `json:"worktree"`
	Base      string `json:"base"`
	Account   string `json:"account,omitempty"`
	ConfigDir string `json:"config_dir,omitempty"`
	// Why is the reason the account was picked, in words a person reads.
	Why       string   `json:"why"`
	Model     string   `json:"model,omitempty"`
	Effort    string   `json:"effort,omitempty"`
	Command   []string `json:"command"`
	Log       string   `json:"log"`
	StartedAt string   `json:"started_at"`
	ExitCode  *int     `json:"exit_code,omitempty"`
	Exit      string   `json:"exit,omitempty"`
	Code      any      `json:"code,omitempty"`
	EndedAt   string   `json:"ended_at,omitempty"`
}

// Runner starts agents for spawn lines. Home is the Grid home: worktrees go under
// Home/worktrees/<task>/<job>-a<n> and attempt records and logs under
// Home/runs/<task>/<job>/a<n>.
type Runner struct {
	Home   string
	Member string // the holder's member id, the author of every post
	Role   string // the author role of every post
	// Command is the agent command and its first arguments; empty is claude -p.
	Command []string
	// PermissionMode goes to claude as --permission-mode when set; Grid writes no default.
	PermissionMode string
	// Repo is the repo a Job runs in when its spawn line names none.
	Repo string
	// Env is the environment the agent starts from; nil is the process environment.
	Env []string
	// Render answers the prompt of the Job: its rendered brief.
	Render func(ctx context.Context, sp Spawn) (string, error)
	// Base answers the Job's base, or "" to start from the repo's HEAD.
	Base func(sp Spawn) string
	// Accounts answers the accounts with their windows; nil or none starts on the default.
	Accounts func() ([]Account, error)
	// Finish is held while an exit and the control after it are posted. The caller
	// holds the same lock around each stream pass, so a pass never sees the exit
	// alone. Nil uses a lock of the runner's own.
	Finish *sync.Mutex
	Diag   io.Writer
	Now    func() time.Time

	wg   sync.WaitGroup
	mu   sync.Mutex
	runs map[string]bool
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) diag(format string, args ...any) {
	if r.Diag != nil {
		fmt.Fprintf(r.Diag, "grid run: "+format+"\n", args...)
	}
}

// Wait returns once every agent the runner started has ended and its exit is posted.
func (r *Runner) Wait() { r.wg.Wait() }

// Start starts the agent of one spawn line in its own goroutine and returns at once. A
// spawn already started by this runner (same task, Job and fence) is not started twice.
// When ctx ends, the agent is interrupted and its exit is posted as worker_stopped
// unless it ended done with a result.
func (r *Runner) Start(ctx context.Context, sp Spawn) {
	key := fmt.Sprintf("%s\x00%s\x00%d", sp.TaskDir, sp.ID, sp.Fence)
	r.mu.Lock()
	if r.runs == nil {
		r.runs = map[string]bool{}
	}
	if r.runs[key] {
		r.mu.Unlock()
		return
	}
	r.runs[key] = true
	r.mu.Unlock()
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.run(ctx, sp); err != nil {
			r.diag("task %s: job %s attempt %d: %s", sp.Task, sp.ID, sp.Attempt, oneLine(err))
		}
	}()
}

// Branch is the branch an attempt works on: grid-<task>-<job>-a<n>.
func Branch(sp Spawn) string { return fmt.Sprintf("grid-%s-%s-a%d", sp.Task, sp.ID, sp.Attempt) }

func (r *Runner) attemptDir(sp Spawn) string {
	return filepath.Join(r.Home, "runs", sp.Task, sp.ID, fmt.Sprintf("a%d", sp.Attempt))
}

// AttemptPath is the attempt record of a spawn under home.
func AttemptPath(home string, sp Spawn) string {
	return filepath.Join(home, "runs", sp.Task, sp.ID, fmt.Sprintf("a%d", sp.Attempt), "attempt.json")
}

func (r *Runner) run(ctx context.Context, sp Spawn) error {
	rec := Attempt{Task: sp.Task, Job: sp.ID, Attempt: sp.Attempt, Fence: sp.Fence, Branch: Branch(sp),
		Model: sp.Model, Effort: sp.Effort, StartedAt: contract.FormatTime(r.now())}
	dir := r.attemptDir(sp)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rec.Log = filepath.Join(dir, "agent.log")
	prompt, err := r.prepare(ctx, sp, &rec)
	if err != nil {
		// The Job did not start: ack the tenure and end it as a crash that names the cause,
		// so the tick retries it as any failed attempt.
		r.diag("task %s: job %s: prepare failed: %s", sp.Task, sp.ID, oneLine(err))
		if aerr := r.ack(sp); aerr != nil {
			return aerr
		}
		return r.finish(sp, "crash", "prepare_failed", false)
	}
	rec.Command = r.command(sp, prompt)
	if err := writeAttempt(dir, rec); err != nil {
		return err
	}
	if err := r.ack(sp); err != nil {
		return err
	}
	logf, err := os.OpenFile(rec.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return r.finish(sp, "crash", "prepare_failed", false)
	}
	cmd := exec.Command(rec.Command[0], rec.Command[1:]...)
	cmd.Dir = rec.Worktree
	cmd.Env = r.agentEnv(sp, rec.ConfigDir)
	cmd.Stdout, cmd.Stderr = logf, logf
	ownGroup(cmd)
	if err := cmd.Start(); err != nil {
		_ = logf.Close()
		r.diag("task %s: job %s: start: %s", sp.Task, sp.ID, oneLine(err))
		return r.finish(sp, "crash", "start_failed", false)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	var werr error
	select {
	case werr = <-done:
	case <-ctx.Done():
		// grid run was stopped: the agent is asked to stop and its end is waited for.
		stopped = true
		interrupt(cmd)
		werr = <-done
	}
	_ = logf.Close()
	code := 0
	if werr != nil {
		var exit *exec.ExitError
		if !errors.As(werr, &exit) {
			return r.finish(sp, "crash", "wait_failed", stopped)
		}
		code = exit.ExitCode()
	}
	rec.ExitCode = &code
	reason, exitCode := r.classify(sp, rec.Log, code, stopped)
	rec.Exit, rec.Code, rec.EndedAt = reason, exitCode, contract.FormatTime(r.now())
	_ = writeAttempt(dir, rec)
	return r.finish(sp, reason, exitCode, stopped)
}

// prepare makes the attempt's worktree, picks its account and renders its prompt.
func (r *Runner) prepare(ctx context.Context, sp Spawn, rec *Attempt) (string, error) {
	repo := sp.Repo
	if repo == "" {
		repo = r.Repo
	}
	if repo == "" {
		return "", errors.New("the Job names no repo and grid run was given none")
	}
	base := ""
	if r.Base != nil {
		base = r.Base(sp)
	}
	if base == "" {
		base = "HEAD"
	}
	rec.Worktree = filepath.Join(r.Home, "worktrees", sp.Task, fmt.Sprintf("%s-a%d", sp.ID, sp.Attempt))
	sha, err := worktree(ctx, repo, rec.Worktree, rec.Branch, base)
	if err != nil {
		return "", err
	}
	rec.Base = sha
	acct, why := r.pick()
	rec.Account, rec.Why = acct.ID, why
	if !acct.Default {
		rec.ConfigDir = acct.Dir
	}
	if r.Render == nil {
		return "", errors.New("no brief renderer")
	}
	return r.Render(ctx, sp)
}

// worktree makes dir a worktree of repo on branch, from base. A worktree already on
// that branch (a usage-limit retry keeps its attempt number) is used as it is, and a
// branch whose worktree was removed is checked out again. It answers the base sha.
func worktree(ctx context.Context, repo, dir, branch, base string) (string, error) {
	if head, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && head == branch {
		return git(ctx, dir, "rev-parse", "HEAD")
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return "", err
	}
	if _, err := git(ctx, repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		if _, err := git(ctx, repo, "worktree", "add", dir, branch); err != nil {
			return "", err
		}
		return git(ctx, dir, "rev-parse", "HEAD")
	}
	sha, err := git(ctx, repo, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("base %s: %w", base, err)
	}
	if _, err := git(ctx, repo, "worktree", "add", "-b", branch, dir, sha); err != nil {
		return "", err
	}
	return sha, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// pick chooses the account an agent starts under, from the windows of each Claude
// account: the one with the most room (the lowest use of its fullest window). With no
// numbers at all it is the default account; when every account is full it is the one
// whose window resets first, and the agent still starts. It never holds the Job.
func (r *Runner) pick() (Account, string) {
	var list []Account
	if r.Accounts != nil {
		got, err := r.Accounts()
		if err != nil {
			return Account{Default: true}, "default account: the account windows could not be read (" + oneLine(err) + ")"
		}
		for _, a := range got {
			if a.Windows.Kind == "" || a.Windows.Kind == account.Claude {
				list = append(list, a)
			}
		}
	}
	type scored struct {
		a     Account
		used  float64
		reset int64
		words string
	}
	var known []scored
	for _, a := range list {
		if a.Windows.Source == "none" || len(a.Windows.Windows) == 0 {
			continue
		}
		s := scored{a: a}
		var parts []string
		for _, w := range a.Windows.Windows {
			if w.Used > s.used {
				s.used = w.Used
			}
			if w.Used >= 100 && w.ResetsAt > 0 && (s.reset == 0 || w.ResetsAt > s.reset) {
				s.reset = w.ResetsAt
			}
			part := fmt.Sprintf("%s %s%%", w.Label, strconv.FormatFloat(w.Used, 'f', -1, 64))
			if w.ResetsAt > 0 {
				part += ", resets " + time.Unix(w.ResetsAt, 0).UTC().Format("2006-01-02 15:04Z")
			}
			parts = append(parts, part)
		}
		s.words = strings.Join(parts, "; ")
		known = append(known, s)
	}
	if len(known) == 0 {
		for _, a := range list {
			if a.Default {
				return a, "default account " + a.ID + ": no account has window numbers"
			}
		}
		if len(list) > 0 {
			return list[0], "account " + list[0].ID + ": no account has window numbers and none is the default"
		}
		return Account{Default: true}, "default account: no account is registered"
	}
	sort.SliceStable(known, func(i, j int) bool { return known[i].used < known[j].used })
	best := known[0]
	if best.used < 100 {
		return best.a, fmt.Sprintf("account %s has the most room of %d: %s", best.a.ID, len(known), best.words)
	}
	sort.SliceStable(known, func(i, j int) bool { return known[i].reset < known[j].reset })
	best = known[0]
	return best.a, fmt.Sprintf("every account is full; account %s resets first: %s", best.a.ID, best.words)
}

// command is the agent command: options first and the prompt last, after --, so a
// brief that starts with front matter is not read as an option.
func (r *Runner) command(sp Spawn, prompt string) []string {
	cmd := append([]string{}, r.Command...)
	if len(cmd) == 0 {
		cmd = []string{"claude", "-p", "--output-format", "stream-json", "--verbose"}
	}
	if sp.Model != "" {
		cmd = append(cmd, "--model", sp.Model)
	}
	if sp.Effort != "" {
		cmd = append(cmd, "--effort", sp.Effort)
	}
	if len(sp.Tools) > 0 {
		cmd = append(cmd, "--allowedTools", strings.Join(sp.Tools, ","))
	}
	if r.PermissionMode != "" {
		cmd = append(cmd, "--permission-mode", r.PermissionMode)
	}
	return append(cmd, "--", prompt)
}

// sessionKeys belong to the session grid run was started from, not to a Job.
var sessionKeys = []string{"GRID_FEED_TASK", "GRID_WAKE", "GRID_WORKER", "CLAUDE_CONFIG_DIR", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT"}

func (r *Runner) agentEnv(sp Spawn, configDir string) []string {
	base := r.Env
	if base == nil {
		base = os.Environ()
	}
	values := map[string]string{}
	var order []string
	for _, e := range base {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if _, seen := values[k]; !seen {
			order = append(order, k)
		}
		values[k] = v
	}
	for _, k := range sessionKeys {
		delete(values, k)
	}
	set := func(k, v string) {
		if _, seen := values[k]; !seen {
			order = append(order, k)
		}
		values[k] = v
	}
	set("GRID_HOME", r.Home)
	set("GRID_WORKER_ID", sp.Holder)
	set("GRID_MEMBER", r.Member)
	set("GRID_ROLE", "worker")
	set("GRID_TASK", sp.TaskDir)
	set("GRID_JOB", sp.ID)
	set("GRID_ATTEMPT", strconv.Itoa(sp.Attempt))
	set("GRID_LEASE", sp.ID)
	set("GRID_LEASE_EPOCH", fmt.Sprintf("%d.%d", sp.Gen, sp.Fence))
	set("GRID_FENCE", strconv.FormatInt(sp.Fence, 10))
	set("GRID_ASSIGN", sp.Assign)
	if sp.TaskEpoch != "" {
		set("GRID_TASK_EPOCH", sp.TaskEpoch)
	} else {
		delete(values, "GRID_TASK_EPOCH")
	}
	if configDir != "" {
		set("CLAUDE_CONFIG_DIR", configDir)
	}
	out := make([]string, 0, len(values))
	for _, k := range order {
		if v, ok := values[k]; ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// Exit codes the runner writes on a worker-exit. worker_stopped is the code the tick
// reads to put a Job back to Ready (orchestrator yc_worker_stopped.go); no_result marks
// an agent that ended done with nothing posted.
const (
	CodeWorkerStopped = "worker_stopped"
	CodeNoResult      = "no_result"
)

// classify reads how an agent ended. A usage limit in its output is usage_limit with
// the reset time (unix seconds) as the code, which the tick reads to make the Job
// Ready at that time. A stop by grid run is cancelled/worker_stopped unless the agent
// still ended done with a result. Exit 0 is done (code no_result when the attempt
// posted no result); any other code is a crash.
func (r *Runner) classify(sp Spawn, log string, code int, stopped bool) (string, any) {
	if reset, limited := UsageLimit(log); limited {
		if reset > 0 {
			return "usage_limit", reset
		}
		return "usage_limit", nil
	}
	has := r.hasResult(sp)
	if stopped && !(code == 0 && has) {
		return "cancelled", CodeWorkerStopped
	}
	if code != 0 {
		return "crash", code
	}
	if !has {
		return "done", CodeNoResult
	}
	return "done", 0
}

func (r *Runner) hasResult(sp Spawn) bool {
	snap, err := orchestrator.Load(feed.Store{TaskDir: sp.TaskDir, TaskID: sp.Task})
	if err != nil {
		return false
	}
	j := snap.State.Jobs[sp.ID]
	if j == nil {
		return false
	}
	for _, id := range j.Results {
		rec := snap.Records[id]
		if rec.Attempt == nil || *rec.Attempt == sp.Attempt {
			return true
		}
	}
	return false
}

var (
	resetField = regexp.MustCompile(`"(?:resetsAt|resets_at|reset_at|resetAt)"\s*:\s*"?([0-9]{9,})`)
	resetPipe  = regexp.MustCompile(`(?i)usage[ _]limit[ _]reached\|([0-9]{9,})`)
)

// UsageLimit reads an agent's output for usage_limit_reached and answers the reset
// time in unix seconds, 0 when the output names none. The reset is read from the line
// that says the limit was reached (a resetsAt field or the "usage limit reached|<epoch>"
// form), else from the last rejected rate_limit_event of the run.
func UsageLimit(log string) (int64, bool) {
	f, err := os.Open(log)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	limited := false
	var reset, rejected int64
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lower := bytes.ToLower(line)
			if bytes.Contains(lower, []byte("usage_limit_reached")) || bytes.Contains(lower, []byte("usage limit reached")) {
				limited = true
				if m := resetPipe.FindSubmatch(line); m != nil {
					reset, _ = strconv.ParseInt(string(m[1]), 10, 64)
				} else if m := resetField.FindSubmatch(line); m != nil {
					reset, _ = strconv.ParseInt(string(m[1]), 10, 64)
				}
			}
			if bytes.Contains(line, []byte("rate_limit_event")) && bytes.Contains(line, []byte(`"rejected"`)) {
				var ev struct {
					Info struct {
						Status   string  `json:"status"`
						ResetsAt float64 `json:"resetsAt"`
					} `json:"rate_limit_info"`
				}
				if json.Unmarshal(line, &ev) == nil && ev.Info.Status == "rejected" && ev.Info.ResetsAt > 0 {
					rejected = int64(ev.Info.ResetsAt)
				}
			}
		}
		if err != nil {
			break
		}
	}
	if !limited {
		return 0, false
	}
	if reset == 0 {
		reset = rejected
	}
	return reset, true
}

func (r *Runner) jobWriter(sp Spawn) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: r.Member, Role: r.Role}, Worker: sp.Holder, Lease: sp.ID, Gen: sp.Gen, Fence: sp.Fence}
}

// taskWriter is the holder's current task-lease writer for the spawn's task.
func (r *Runner) taskWriter(sp Spawn) (feed.Writer, error) {
	t, err := (lease.TaskStore{Feed: feed.Store{TaskDir: sp.TaskDir, TaskID: sp.Task}}).Read("task")
	if err != nil {
		return feed.Writer{}, err
	}
	if t.Holder == nil || *t.Holder != sp.Holder {
		return feed.Writer{}, feed.ErrStaleLease
	}
	return feed.Writer{Author: feed.Author{ID: r.Member, Role: r.Role}, Worker: sp.Holder, Lease: "task", Gen: t.Gen, Fence: int64(t.Fence)}, nil
}

func (r *Runner) ack(sp Spawn) error {
	body := map[string]any{}
	if sp.Model != "" {
		body["model"] = sp.Model
	}
	_, err := orchestrator.Write(feed.Store{TaskDir: sp.TaskDir, TaskID: sp.Task}, r.jobWriter(sp),
		feed.Input{Type: "ack", Job: sp.ID, Attempt: sp.Attempt, After: []string{sp.Assign}, Body: body})
	return err
}

// finish posts the worker-exit, and the control that follows it when the exit needs
// one: a worker stop puts the Job back to Ready, and a done exit with no result parks
// it for the lead with a loop-detected wake, so it is neither submitted nor retried.
func (r *Runner) finish(sp Spawn, reason string, code any, _ bool) error {
	lock := r.Finish
	if lock == nil {
		lock = &r.mu
	}
	lock.Lock()
	defer lock.Unlock()
	store := feed.Store{TaskDir: sp.TaskDir, TaskID: sp.Task}
	body := map[string]any{"reason": reason}
	if code != nil {
		body["code"] = code
	}
	exit, err := orchestrator.Write(store, r.jobWriter(sp), feed.Input{Type: "worker-exit", Job: sp.ID, Attempt: sp.Attempt, Body: body})
	if err != nil {
		return err
	}
	var follow []feed.Input
	switch {
	case reason == "cancelled" && code == CodeWorkerStopped:
		follow = append(follow, feed.Input{Type: "control", Re: exit.ID, Body: map[string]any{"target": sp.ID, "status": string(contract.Ready), "reason": "worker stopped"}})
	case reason == "done" && code == CodeNoResult:
		follow = append(follow,
			feed.Input{Type: "control", Re: exit.ID, Body: map[string]any{"target": sp.ID, "status": string(contract.Parked), "reason": CodeNoResult}},
			feed.Input{Type: "loop-detected", Job: sp.ID, Body: map[string]any{"signal": CodeNoResult, "threshold": 1,
				"error": sp.ID + " attempt " + strconv.Itoa(sp.Attempt) + " ended done with no result: stuck for the lead"}})
	}
	if len(follow) == 0 {
		return nil
	}
	w, err := r.taskWriter(sp)
	if err != nil {
		return err
	}
	for _, in := range follow {
		if _, err := orchestrator.Write(store, w, in); err != nil {
			return err
		}
	}
	return nil
}

func writeAttempt(dir string, rec Attempt) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(dir, "attempt.json"), append(b, '\n'), 0o600)
}

func oneLine(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }
