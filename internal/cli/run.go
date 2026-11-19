package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gridconsole/grid-plugins/internal/account"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobcmd"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/runner"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// runRun is grid run: the task holder's stream (the one grid worker next prints), with
// an agent started for every Job it is assigned. Each agent runs in its own worktree
// on grid-<task>-<job>-a<n>, under the account with the most room, with the Job's
// rendered brief; grid run posts the ack and the worker-exit. Every spawn starts at
// once: grid run adds no cap, delay or hold on Jobs. SIGINT stops it: no new agent
// starts, each running agent is interrupted and its exit is posted.
func runRun(args []string, env Env) int {
	fs := newFlags("run", env)
	command := fs.String("command", "", "the agent command and its first arguments, split on spaces; empty is claude -p --output-format stream-json --verbose")
	permission := fs.String("permission-mode", "", "the --permission-mode claude is started with; empty leaves it out")
	repo := fs.String("repo", "", "the repo a Job runs in when its project names none; empty is the repo of the current folder")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	member, machine, role, code := workerHolder(env, "run")
	if code != 0 {
		return code
	}
	start, err := session.ProcessStart(os.Getpid())
	if err != nil {
		return livenessError(env, err)
	}
	if *repo == "" {
		*repo = env.Cwd
	}
	diag := workerDiag(env)
	ctx, cancel := env.signalContext(os.Interrupt)
	defer cancel()

	var finish sync.Mutex
	r := &runner.Runner{
		Home: env.GridHome, Member: member, Role: role,
		Command: strings.Fields(*command), PermissionMode: *permission, Repo: *repo,
		Env:      gridRunAgentEnv(env),
		Render:   func(_ context.Context, sp runner.Spawn) (string, error) { return gridRunRender(env, sp) },
		Base:     func(sp runner.Spawn) string { return gridRunBase(env, sp) },
		Accounts: func() ([]runner.Account, error) { return gridRunAccounts(env) },
		Finish:   &finish, Diag: diag,
	}
	// The stream's lines are printed as grid worker next prints them; each spawn line
	// also starts its agent. The stream writes one line per Write.
	out := writerFunc(func(p []byte) (int, error) {
		n, err := env.Stdout.Write(p)
		var line struct {
			Spawn *runner.Spawn `json:"spawn"`
		}
		if json.Unmarshal(bytes.TrimSpace(p), &line) == nil && line.Spawn != nil && ctx.Err() == nil {
			r.Start(ctx, *line.Spawn)
		}
		return n, err
	})
	s := workerStream(env, member, machine, role, os.Getpid(), contract.FormatTime(start), out, diag)
	err = gridRunPasses(ctx, s, &finish)
	// The stream has stopped: wait for every agent to end and its exit to be posted.
	r.Wait()
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid run: %v\n", err)
		return 1
	}
	return 1
}

// gridRunPasses passes the stream over the home until ctx ends, holding finish over each
// pass, so the runner's exit post and the control that follows it land together. The
// pause between passes is the stream's own (one second). Helpers of grid run are not
// named run*, which the command inventory reads as parser routes.
func gridRunPasses(ctx context.Context, s *orchestrator.Stream, finish *sync.Mutex) error {
	for {
		finish.Lock()
		err := s.Pass(ctx)
		finish.Unlock()
		if err != nil {
			return err
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// gridRunAgentEnv is the environment an agent starts from: grid run's own process
// environment, with the keys a caller's Getenv gives over it.
func gridRunAgentEnv(env Env) []string {
	out := os.Environ()
	if env.Getenv != nil {
		// A test or embedding caller gives its own environment: carry its GRID_ and PATH keys.
		for _, k := range []string{"PATH", "HOME", "GRID_MEMBER", "GRID_OWNER"} {
			if v := env.Getenv(k); v != "" {
				out = append(out, k+"="+v)
			}
		}
	}
	return out
}

// gridRunRender renders the Job's brief as grid job render does and answers it.
func gridRunRender(env Env, sp runner.Spawn) (string, error) {
	var stdout, stderr bytes.Buffer
	je := jobcmdEnv(env, resolver(env))
	je.Stdout, je.Stderr = &stdout, &stderr
	if code := jobcmd.Run([]string{"render", sp.TaskDir, sp.ID, "--json"}, je); code != 0 {
		return "", fmt.Errorf("grid job render %s: exit %d: %s", sp.ID, code, strings.TrimSpace(stderr.String()+" "+stdout.String()))
	}
	b, err := os.ReadFile(filepath.Join(sp.TaskDir, "jobs", sp.ID, "brief.md"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// gridRunBase is the base the Job's record names in its base field, or "" for the repo's HEAD.
func gridRunBase(env Env, sp runner.Spawn) string {
	snap, err := orchestrator.Load(feed.Store{TaskDir: sp.TaskDir, TaskID: sp.Task, Warn: env.Stderr})
	if err != nil {
		return ""
	}
	base := ""
	for _, rec := range snap.Records {
		if rec.Type == "job" && rec.Body["id"] == sp.ID {
			if b, ok := rec.Body["base"].(string); ok {
				base = b
			}
		}
	}
	if _, ok := job.HeadBaseTarget(base); ok {
		// An unresolved head:J-n stays as it is: git refuses it, so the Job never starts from HEAD.
		if resolved, err := orchestrator.ResolveBase(snap, sp.Task, base); err == nil {
			return resolved
		}
	}
	return base
}

// gridRunAccounts is the accounts grid account windows reads, with each one's config folder.
func gridRunAccounts(env Env) ([]runner.Account, error) {
	aenv := accountCoreEnv(env)
	list, err := account.Status(aenv, false)
	if err != nil {
		return nil, err
	}
	windows, err := account.Windows(aenv, "")
	if err != nil {
		return nil, err
	}
	byID := map[string]account.AccountWindows{}
	for _, w := range windows {
		byID[w.ID] = w
	}
	out := make([]runner.Account, 0, len(list))
	for _, a := range list {
		w, ok := byID[a.ID]
		if !ok {
			w = account.AccountWindows{ID: a.ID, Kind: a.Kind, Source: "none"}
		}
		out = append(out, runner.Account{ID: a.ID, Dir: a.Dir, Default: a.Default, Windows: w})
	}
	return out, nil
}

var _ io.Writer = writerFunc(nil)
