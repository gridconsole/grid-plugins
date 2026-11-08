// Package land executes only the command named by the project's deploy topic.
package land

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/eval"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/taskhistory"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Rule is the lands mapping of one Markdown entry in knowledge/deploy/.
type Rule struct {
	Target       string `json:"target" yaml:"target"`
	Branch       string `json:"branch" yaml:"branch"`
	Command      string `json:"command" yaml:"command"`
	NeedsApprove bool   `json:"needs_approve" yaml:"needs-approve"`
}
type Plan struct {
	Task    string `json:"task"`
	Project string `json:"project"`
	Hash    string `json:"plan_hash"`
	Rule    Rule   `json:"lands"`
}
type Result struct {
	Plan      Plan                `json:"plan"`
	Status    string              `json:"status"`
	Execution *eval.ProcessResult `json:"execution,omitempty"`
}

// Approval is a local gate record written by the person/app gate writer, never
// by land. It is not a Feed post. Agent and guest tiers cannot approve landing.
type Approval struct {
	PlanHash string             `json:"plan_hash"`
	Approved bool               `json:"approved"`
	Tier     contract.TrustTier `json:"tier"`
}
type Store struct {
	Resolver where.Resolver
	Timeout  time.Duration // zero uses ten minutes
	Now      func() time.Time
}

func (s Store) prepare(cwd, id string) (Plan, task.Task, string, error) {
	p := Plan{}
	t, err := (task.Store{Resolver: s.Resolver}).Show(cwd, id)
	if err != nil {
		return p, t, "", err
	}
	if t.Project == gridhome.Personal {
		return p, t, "", fmt.Errorf("deploy topic requires a project")
	}
	p.Task, p.Project = t.ID, t.Project
	root, err := os.OpenRoot(t.Path)
	if err != nil {
		return p, t, "", err
	}
	defer root.Close()
	hash, err := safefile.Read(root, "plan.hash")
	if err != nil {
		return p, t, "", fmt.Errorf("plan.hash: %w", err)
	}
	// The hash belongs to the plan writer. Remove its line ending only; never
	// compute, split, decode, or otherwise reinterpret the opaque value.
	p.Hash = strings.TrimSuffix(strings.TrimSuffix(string(hash), "\n"), "\r")
	if strings.TrimSpace(p.Hash) == "" || strings.ContainsAny(p.Hash, "\r\n\x00") {
		return p, t, "", fmt.Errorf("plan.hash must contain one nonempty opaque value")
	}
	p.Rule, err = s.rule(t.Project)
	if err != nil {
		return p, t, "", err
	}
	res, err := s.Resolver.Resolve(where.Query{Cwd: cwd, Project: t.Project})
	if err != nil {
		return p, t, "", err
	}
	return p, t, res.Folder, nil
}

func (s Store) rule(project string) (Rule, error) {
	var rule Rule
	root, err := os.OpenRoot(s.Resolver.GridHome)
	if err != nil {
		return rule, err
	}
	defer root.Close()
	dir := filepath.Join("projects", project, "knowledge", "deploy")
	if err = safefile.NoSymlinks(root, dir); err != nil {
		return rule, fmt.Errorf("deploy topic: %w", err)
	}
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		return rule, fmt.Errorf("deploy topic: %w", err)
	}
	found := false
	for _, e := range entries {
		if e.IsDir() || e.Name() == "TOPIC.md" || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := safefile.Read(root, filepath.Join(dir, e.Name()))
		if err != nil {
			return rule, fmt.Errorf("deploy topic: %w", err)
		}
		front, _, err := mdfile.Split(b)
		if err != nil {
			return rule, fmt.Errorf("deploy topic %s: %w", e.Name(), err)
		}
		var entry struct {
			Lands map[string]any `yaml:"lands"`
		}
		if err = mdfile.Load(front, &entry, false); err != nil {
			return rule, fmt.Errorf("deploy topic %s: %w", e.Name(), err)
		}
		if entry.Lands == nil {
			continue
		}
		if found {
			return rule, fmt.Errorf("deploy topic has more than one lands rule")
		}
		found = true
		if len(entry.Lands) != 4 {
			return rule, fmt.Errorf("deploy topic lands requires exactly target, branch, command and needs-approve")
		}
		var ok bool
		if rule.Target, ok = entry.Lands["target"].(string); !ok {
			return rule, fmt.Errorf("deploy topic lands.target must be a string")
		}
		if rule.Branch, ok = entry.Lands["branch"].(string); !ok {
			return rule, fmt.Errorf("deploy topic lands.branch must be a string")
		}
		if rule.Command, ok = entry.Lands["command"].(string); !ok {
			return rule, fmt.Errorf("deploy topic lands.command must be a string")
		}
		if rule.NeedsApprove, ok = entry.Lands["needs-approve"].(bool); !ok {
			return rule, fmt.Errorf("deploy topic lands.needs-approve must be a boolean")
		}
		for _, v := range []string{rule.Target, rule.Branch, rule.Command} {
			if strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\r\n\x00") {
				return rule, fmt.Errorf("deploy topic lands values must be nonempty single-line strings")
			}
		}
	}
	if !found {
		return rule, fmt.Errorf("deploy topic has no lands rule")
	}
	return rule, nil
}

// Run is read-only for a dry run. Execution always requires approval for the
// current opaque plan hash, including when needs-approve is false: this field
// describes the deploy rule and cannot bypass the task's approval boundary.
func (s Store) Run(ctx context.Context, cwd, id string, dry bool) (Result, error) {
	p, t, folder, err := s.prepare(cwd, id)
	result := Result{Plan: p, Status: "planned"}
	if err != nil {
		return result, err
	}
	if dry {
		return result, nil
	}
	if folder == "" {
		return result, fmt.Errorf("deploy topic: project has no folder on this machine")
	}
	root, err := os.OpenRoot(t.Path)
	if err != nil {
		return result, err
	}
	defer root.Close()
	if err = safefile.NoSymlinks(root, ".land.lock"); err != nil {
		return result, err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return result, err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return result, err
	}
	current, _, currentFolder, err := s.prepare(cwd, id)
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(p, current) || folder != currentFolder {
		return result, fmt.Errorf("land plan changed; review it again")
	}
	b, err := safefile.Read(root, filepath.Join("approvals", "land.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return result, fmt.Errorf("land refused: approval for the current plan hash is required")
		}
		return result, err
	}
	var approval Approval
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if !json.Valid(b) {
		return result, fmt.Errorf("land approval is invalid JSON")
	}
	if err = decoder.Decode(&approval); err != nil {
		return result, fmt.Errorf("land approval: %w", err)
	}
	if !approval.Approved || approval.Tier != contract.PersonTeam || approval.PlanHash != p.Hash {
		return result, fmt.Errorf("land refused: a team approval for the current plan hash is required")
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	if timeout < 0 {
		return result, fmt.Errorf("land timeout must be positive")
	}
	started := time.Now()
	if s.Now != nil {
		started = s.Now()
	}
	// Record the attempt before invoking a command that may change external state.
	if err = taskhistory.Add(t.Path, taskfile.HistoryLine(started.UTC().Format(time.RFC3339Nano), "land started", fmt.Sprintf("plan %q; target %q; branch %q", p.Hash, p.Rule.Target, p.Rule.Branch), "grid")); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := []string{"sh", "-c", p.Rule.Command}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/d", "/s", "/c", p.Rule.Command}
	}
	env := os.Environ()
	// Replace rather than duplicate variables inherited from an earlier invocation.
	for _, pair := range []struct{ key, value string }{{"GRID_LAND_TASK", p.Task}, {"GRID_LAND_TARGET", p.Rule.Target}, {"GRID_LAND_BRANCH", p.Rule.Branch}, {"GRID_PLAN_HASH", p.Hash}} {
		prefix := pair.key + "="
		next := env[:0]
		for _, v := range env {
			if !strings.HasPrefix(v, prefix) {
				next = append(next, v)
			}
		}
		env = append(next, prefix+pair.value)
	}
	execution := eval.RunProcess(ctx, argv, folder, env, "")
	result.Execution = &execution
	result.Status = "landed"
	if execution.Status != "passed" {
		result.Status = "failed"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Status = "timed-out"
	}
	detail := fmt.Sprintf("plan %q; target %q; branch %q; exit %d", p.Hash, p.Rule.Target, p.Rule.Branch, execution.ExitCode)
	historyErr := taskhistory.Add(t.Path, taskfile.HistoryLine(started.UTC().Format(time.RFC3339Nano), "land "+result.Status, detail, "grid"))
	if execution.Status != "passed" {
		return result, errors.Join(fmt.Errorf("land %s: %s", result.Status, execution.Error), historyErr)
	}
	return result, historyErr
}
