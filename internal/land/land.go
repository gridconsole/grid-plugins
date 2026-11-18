// Package land executes only the command named by the project's deploy topic.
package land

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/eval"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Rule is the lands mapping of one Markdown entry in knowledge/deploy/ (grid/deploy/
// when the brain folder key is set).
type Rule struct {
	Target       string   `json:"target" yaml:"target"`
	Branch       string   `json:"branch" yaml:"branch"`
	Command      string   `json:"command" yaml:"command"`
	NeedsApprove bool     `json:"needs_approve" yaml:"needs-approve"`
	Env          []string `json:"env,omitempty" yaml:"env,omitempty"`
}
type Plan struct {
	Task    string `json:"task"`
	Project string `json:"project"`
	Hash    string `json:"plan_hash"`
	Rule    Rule   `json:"lands"`
	Gate    string `json:"approval_gate"`
}
type Result struct {
	Plan      Plan                `json:"plan"`
	Status    string              `json:"status"`
	Execution *eval.ProcessResult `json:"execution,omitempty"`
}

type Store struct {
	Resolver where.Resolver
	Timeout  time.Duration // zero uses ten minutes
	Now      func() time.Time
	Who      contract.Author // recorded on the history lines; empty is the local person
	// Getenv answers the variables the deploy command may see (the allow list and
	// the rule's env names). Nil reads the host, as the binary does.
	Getenv func(string) string
	// Merge lands the delivered branch of task id when a ship answer finds no
	// deploy topic, and answers the commit the base now points at; "" with no error
	// is a task with no branch. Nil merges nothing.
	Merge func(id string) (string, error)
}

func (s Store) tasks() task.Store { return task.Store{Resolver: s.Resolver, Now: s.Now} }

func (s Store) prepare(cwd, id string) (Plan, task.Task, string, error) {
	p := Plan{}
	t, err := s.tasks().Show(cwd, id)
	if err != nil {
		return p, t, "", err
	}
	if t.Project == gridhome.Personal {
		return p, t, "", fmt.Errorf("deploy topic requires a project")
	}
	p.Task, p.Project = t.ID, t.Project
	p.Gate, err = s.gate(t)
	if err != nil {
		return p, t, "", err
	}
	// The same hash grid approve records, so the approval check compares like with like.
	text, err := plan.Store{TaskDir: t.Path}.Read()
	if err != nil {
		return p, t, "", err
	}
	p.Hash = plan.Hash(text)
	p.Rule, err = s.rule(t.Project)
	if err != nil {
		return p, t, "", err
	}
	// Either policy can require approval; a workflow cannot waive the deploy rule.
	if p.Gate == "none" && p.Rule.NeedsApprove {
		p.Gate = "plan"
	}
	res, err := s.Resolver.Resolve(where.Query{Cwd: cwd, Project: t.Project})
	if err != nil {
		return p, t, "", err
	}
	return p, t, res.Folder, nil
}

// ErrNoDeployTopic is the error rule gives when the project has no deploy topic:
// knowledge/deploy is missing, or it holds no entry with a lands: block. A topic
// that exists but is unreadable or malformed is a different error.
var ErrNoDeployTopic = errors.New("no deploy topic")

// noDeployTopic names the project without the file system text. A missing folder
// still matches os.ErrNotExist, as Approve reads it; a folder with no lands entry
// does not, so Approve keeps refusing it as before.
type noDeployTopic struct {
	project string
	missing error // the not-exist error of a missing folder, nil for no lands entry
}

func (e noDeployTopic) Error() string {
	return fmt.Sprintf("project %s has no deploy topic (knowledge/deploy/<name>.md with a lands: block)", e.project)
}

func (e noDeployTopic) Is(target error) bool { return target == ErrNoDeployTopic }

func (e noDeployTopic) Unwrap() error { return e.missing }

// brainFolder is where the project's deploy rule and workflow notes are read:
// the folder to open and the name of the folder inside a project level that holds
// the config topics. With the brain folder key set it is the brain folder and
// "grid" (projects/<p>/grid/deploy); with the key absent it is the Grid home and
// "knowledge" (projects/<p>/knowledge/deploy), as before.
func (s Store) brainFolder() (root, config string) {
	getenv := s.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	b := brain.Store{GridHome: s.Resolver.GridHome}.WithConfig(s.Resolver.Home, getenv)
	if b.Vault() {
		return b.Root(), "grid"
	}
	return b.Root(), "knowledge"
}

// projectTopic is the path below the folder brainFolder names of one config topic
// of a project.
func projectTopic(config, project, topic string) string {
	return filepath.Join("projects", project, config, topic)
}

func (s Store) rule(project string) (Rule, error) {
	var rule Rule
	folder, config := s.brainFolder()
	root, err := os.OpenRoot(folder)
	if err != nil {
		return rule, err
	}
	defer root.Close()
	dir := projectTopic(config, project, "deploy")
	if err = safefile.NoSymlinks(root, dir); err != nil {
		return rule, fmt.Errorf("deploy topic: %w", err)
	}
	entries, err := fs.ReadDir(root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return rule, noDeployTopic{project: project, missing: err}
	}
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
		for key := range entry.Lands {
			if key != "target" && key != "branch" && key != "command" && key != "needs-approve" && key != "env" {
				return rule, fmt.Errorf("unknown deploy lands field %q", key)
			}
		}
		if len(entry.Lands) < 4 {
			return rule, fmt.Errorf("deploy topic lands requires target, branch, command and needs-approve (env is optional)")
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
		if value, present := entry.Lands["env"]; present {
			names, ok := value.([]any)
			if !ok {
				return rule, fmt.Errorf("deploy topic lands.env must be a list of variable names")
			}
			for _, name := range names {
				value, ok := name.(string)
				if !ok || !envName.MatchString(value) {
					return rule, fmt.Errorf("deploy topic lands.env contains an invalid variable name")
				}
				rule.Env = append(rule.Env, value)
			}
		}
		for _, v := range []string{rule.Target, rule.Branch, rule.Command} {
			if strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\r\n\x00") {
				return rule, fmt.Errorf("deploy topic lands values must be nonempty single-line strings")
			}
		}
	}
	if !found {
		return rule, noDeployTopic{project: project}
	}
	return rule, nil
}

// Run is read-only for a dry run. The task workflow selects the approval gate.
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
		return result, fmt.Errorf("%w: land plan changed; review it again", ErrRejected)
	}
	if p.Gate != "none" {
		approval, err := s.tasks().ApprovalForGate(cwd, t.ID, p.Gate)
		if err != nil {
			return result, fmt.Errorf("%w: %w", ErrRejected, err)
		}
		if approval.PlanHash != p.Hash || approval.LandsHash != ruleHash(p.Rule) {
			return result, fmt.Errorf("%w: the approval is for another plan or lands rule; review it again", ErrRejected)
		}
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	if timeout < 0 {
		return result, fmt.Errorf("land timeout must be positive")
	}
	// Record the attempt before invoking a command that may change external state.
	if err = s.note(cwd, t.ID, fmt.Sprintf("land started: plan %q; target %q; branch %q", p.Hash, p.Rule.Target, p.Rule.Branch)); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := []string{"sh", "-c", p.Rule.Command}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/d", "/s", "/c", p.Rule.Command}
	}
	execution := runProcess(ctx, argv, folder, landEnvironment(p, s.Getenv))
	result.Execution = &execution
	result.Status = "landed"
	if execution.Status != "passed" {
		result.Status = "failed"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Status = "timed-out"
	}
	detail := fmt.Sprintf("plan %q; target %q; branch %q; exit %d", p.Hash, p.Rule.Target, p.Rule.Branch, execution.ExitCode)
	historyErr := s.note(cwd, t.ID, "land "+result.Status+": "+detail)
	if execution.Status != "passed" {
		return result, errors.Join(fmt.Errorf("land %s: %s", result.Status, execution.Error), historyErr)
	}
	return result, historyErr
}

// note uses the same inode lock as hook history appenders, never the task
// journal's read/replace path, which can overwrite a concurrent hook append.
func (s Store) note(cwd, id, text string) error {
	t, err := s.tasks().Show(cwd, id)
	if err != nil {
		return err
	}
	// AppendHistory intentionally does not create history for uninitialized tasks.
	// Land has resolved a real task and initializes an empty file without replacing it.
	f, err := fslock.OpenNoFollow(filepath.Join(t.Path, "history.md"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		if err := f.Close(); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	who := s.Who.ID
	if who == "" {
		who = "local"
	}
	event, detail, _ := strings.Cut(text, ": ")
	return task.AppendHistory(t.Path, event, detail, who, "grid land")
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
