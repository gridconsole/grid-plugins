// Package jobcmd implements grid job render, lint and show without depending on
// the CLI dispatcher or the task-file writer.
package jobcmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

type Env struct {
	Cwd         string
	Home        string
	GridHome    string
	Stdout      io.Writer
	Stderr      io.Writer
	ResolveTask func(string) (string, error)
}
type flags struct {
	json   bool
	task   string
	budget int
	args   []string
	help   bool
}
type report struct {
	OK       bool           `json:"ok"`
	ID       string         `json:"id,omitempty"`
	Path     string         `json:"path,omitempty"`
	Inputs   string         `json:"inputs,omitempty"`
	Job      *job.Job       `json:"job,omitempty"`
	Brief    string         `json:"brief,omitempty"`
	Findings []lint.Finding `json:"findings"`
	Error    string         `json:"error,omitempty"`
}

const usage = "Usage: grid job render <task> <item> [--budget N] [--json]\n       grid job lint <file> [--task <task>] [--json]\n       grid job show [<task> <job>|<job>|<file>] [--task <task>] [--json]\n"

func Run(args []string, e Env) int {
	f, err := parse(args)
	if err != nil {
		return output(e, f, report{Error: err.Error(), Findings: []lint.Finding{}}, 2)
	}
	if f.help || len(f.args) == 0 {
		if _, err := io.WriteString(e.Stdout, usage); err != nil {
			return 1
		}
		return 0
	}
	action, args := f.args[0], f.args[1:]
	fail := func(err error, code int) int {
		return output(e, f, report{Error: err.Error(), Findings: []lint.Finding{}}, code)
	}
	switch action {
	case "render":
		if len(args) != 2 || f.task != "" {
			return fail(errors.New("render requires <task> <item>"), 2)
		}
		task, err := resolveTask(args[0], e)
		if err != nil {
			return fail(err, 1)
		}
		j, err := job.LoadPlan(task, args[1])
		if err != nil {
			return fail(err, 1)
		}
		files, repos, err := sources(task, e)
		if err != nil {
			return fail(err, 1)
		}
		generation, err := indexGeneration(files)
		if err != nil {
			return fail(err, 1)
		}
		// Check planned instructions before resolving or writing any artifacts.
		meta, err := job.Metadata(j)
		if err != nil {
			return fail(err, 1)
		}
		skeleton := meta + "\n## Intent\n## Job\n## Context\n## Out of scope\n## Open\n## Acceptance\n## Outcome\n## Verdict\n## Follow-up\n"
		findings := lint.Check(skeleton, lint.Options{Resolver: files, Repos: repos})
		if lint.HasErrors(findings) {
			return output(e, f, report{ID: j.ID, Findings: findings}, 1)
		}
		rendered, err := job.Render(j, job.RenderOptions{Resolver: files, Generation: generation, BrainTokens: f.budget})
		if err != nil {
			return fail(err, 1)
		}
		after, err := indexGeneration(files)
		if err != nil {
			return fail(err, 1)
		}
		if generation != after {
			return fail(errors.New("brain index changed during render; retry with a stable generation"), 1)
		}
		if generation == "" {
			b, _ := json.Marshal(rendered.Inputs.Sources)
			sum := sha256.Sum256(b)
			rendered.Inputs.Generation = "content:" + hex.EncodeToString(sum[:])
		}
		path, err := save(task, j, rendered)
		if err != nil {
			return fail(err, 1)
		}
		return output(e, f, report{OK: true, ID: j.ID, Path: display(path, e), Inputs: display(filepath.Join(filepath.Dir(path), "inputs.json"), e), Findings: findings}, 0)
	case "lint":
		if len(args) != 1 || f.budget != 0 {
			return fail(errors.New("lint requires <file>"), 2)
		}
		path := absolute(e.Cwd, args[0])
		b, err := os.ReadFile(path)
		if err != nil {
			return fail(err, 1)
		}
		task := findTask(filepath.Dir(path))
		if f.task != "" {
			task, err = resolveTask(f.task, e)
			if err != nil {
				return fail(err, 1)
			}
		}
		if task == "" {
			task = filepath.Dir(path)
		}
		files, repos, err := sources(task, e)
		if err != nil {
			return fail(err, 1)
		}
		findings := lint.Check(string(b), lint.Options{Resolver: files, Repos: repos})
		ok := !lint.HasErrors(findings)
		code := 0
		if !ok {
			code = 1
		}
		return output(e, f, report{OK: ok, Path: display(path, e), Findings: findings}, code)
	case "show":
		if len(args) > 2 || f.budget != 0 {
			return fail(errors.New("show accepts a saved brief file or a task and Job id"), 2)
		}
		path, err := showPath(args, f, e)
		if err != nil {
			return fail(err, 1)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fail(err, 1)
		}
		j, _, err := job.DecodeBrief(string(b))
		if err != nil {
			return fail(err, 1)
		}
		return output(e, f, report{OK: true, ID: j.ID, Path: display(path, e), Job: &j, Brief: string(b), Findings: []lint.Finding{}}, 0)
	default:
		return fail(fmt.Errorf("unknown job command %q; use render, lint or show", action), 2)
	}
}
func parse(args []string) (flags, error) {
	f := flags{}
	for _, a := range args {
		if a == "--json" {
			f.json = true
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
		case a == "--help" || a == "-h":
			f.help = true
		case a == "--task" || a == "--budget":
			if i+1 == len(args) {
				return f, fmt.Errorf("%s needs a value", a)
			}
			i++
			if a == "--task" {
				f.task = args[i]
			} else {
				n, err := strconv.Atoi(args[i])
				if err != nil || n < 1 || n > 2000 {
					return f, errors.New("--budget must be between 1 and 2000")
				}
				f.budget = n
			}
		case strings.HasPrefix(a, "-"):
			return f, fmt.Errorf("unknown flag %s", a)
		default:
			f.args = append(f.args, a)
		}
	}
	return f, nil
}
func output(e Env, f flags, r report, code int) int {
	if r.Findings == nil {
		r.Findings = []lint.Finding{}
	}
	if f.json {
		enc := json.NewEncoder(e.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(r); err != nil {
			return 1
		}
		return code
	}
	var b strings.Builder
	if r.Error != "" {
		if _, err := fmt.Fprintf(e.Stderr, "job: %s\n", r.Error); err != nil {
			return 1
		}
		return code
	}
	if r.Brief != "" {
		b.WriteString(r.Brief)
	} else {
		if r.ID != "" {
			fmt.Fprintf(&b, "id: %s\n", r.ID)
		}
		if r.Path != "" {
			fmt.Fprintf(&b, "path: %s\n", r.Path)
		}
		if r.Inputs != "" {
			fmt.Fprintf(&b, "inputs: %s\n", r.Inputs)
		}
		fmt.Fprintf(&b, "ok: %t\n", r.OK)
	}
	for _, finding := range r.Findings {
		fmt.Fprintf(&b, "%s:%d: %s [%s]: %s; fix: %s\n", r.Path, finding.Line, finding.Severity, finding.Rule, finding.Message, finding.Fix)
	}
	if _, err := io.WriteString(e.Stdout, b.String()); err != nil {
		return 1
	}
	return code
}
func absolute(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}
func display(p string, e Env) string { return gridhome.Tilde(p, e.Home) }
func resolveTask(s string, e Env) (string, error) {
	p := absolute(e.Cwd, s)
	if stat, err := os.Stat(filepath.Join(p, "plan.md")); err == nil && !stat.IsDir() {
		return filepath.Abs(p)
	}
	if s == "" {
		if p := findTask(e.Cwd); p != "" {
			return p, nil
		}
	}
	if e.ResolveTask != nil {
		p, err := e.ResolveTask(s)
		if err != nil {
			return "", err
		}
		if p != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("cannot find task %q; pass a task folder with plan.md", s)
}
func findTask(dir string) string {
	for {
		if s, e := os.Stat(filepath.Join(dir, "plan.md")); e == nil && !s.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
func showPath(args []string, f flags, e Env) (string, error) {
	if len(args) == 1 && !job.ValidID(args[0]) {
		p := absolute(e.Cwd, args[0])
		if s, err := os.Stat(p); err == nil && !s.IsDir() {
			return p, nil
		}
	}
	taskArg, id := f.task, ""
	if len(args) == 2 {
		if taskArg != "" {
			return "", errors.New("give the task once")
		}
		taskArg, id = args[0], args[1]
	} else if len(args) == 1 {
		id = args[0]
	}
	task, err := resolveTask(taskArg, e)
	if err != nil {
		return "", err
	}
	if id == "" {
		paths, err := filepath.Glob(filepath.Join(task, "jobs", "J-*", "brief.md"))
		if err != nil {
			return "", err
		}
		if len(paths) != 1 {
			return "", fmt.Errorf("task has %d saved Jobs; specify a Job id", len(paths))
		}
		return job.Within(task, strings.TrimPrefix(paths[0], task+string(filepath.Separator)))
	}
	if !job.ValidID(id) {
		return "", errors.New("invalid Job id")
	}
	return job.Within(task, filepath.Join("jobs", id, "brief.md"))
}

// Task-local root overrides are a small adapter, not the shared task schema.
// Relative roots are resolved against the task folder, never the process cwd.
func sources(task string, e Env) (*job.Files, []string, error) {
	f := &job.Files{Task: task, Repo: task, Repos: map[string]string{}, Brain: map[string]string{}}
	if e.GridHome != "" {
		f.Brain["home"] = filepath.Join(e.GridHome, "knowledge")
	}
	if filepath.Base(filepath.Dir(task)) == "tasks" {
		f.Brain["project"] = filepath.Join(filepath.Dir(filepath.Dir(task)), "knowledge")
	}
	f.Brain["task"] = filepath.Join(task, "knowledge")
	b, err := os.ReadFile(filepath.Join(task, "task.md"))
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if err == nil {
		d, err := frontmatter.Parse(string(b))
		if err != nil {
			return nil, nil, fmt.Errorf("task.md: %w", err)
		}
		var cfg struct {
			Repo      string            `json:"repo"`
			Prototype string            `json:"prototype"`
			Repos     map[string]string `json:"repos"`
			Brain     map[string]string `json:"brain"`
		}
		if err = frontmatter.Into(d.Data, &cfg); err != nil {
			return nil, nil, fmt.Errorf("task.md source roots: %w", err)
		}
		if cfg.Repo != "" {
			f.Repo = absolute(task, cfg.Repo)
		}
		if cfg.Prototype != "" {
			f.Prototype = absolute(task, cfg.Prototype)
		}
		for k, v := range cfg.Repos {
			f.Repos[k] = absolute(task, v)
		}
		for k, v := range cfg.Brain {
			f.Brain[k] = absolute(task, v)
		}
	}
	repos := []string{}
	for k := range f.Repos {
		repos = append(repos, k)
	}
	sort.Strings(repos)
	return f, repos, nil
}
func indexGeneration(f *job.Files) (string, error) {
	generations := map[string]json.RawMessage{}
	for level, root := range f.Brain {
		b, e := os.ReadFile(filepath.Join(root, "backlinks.json"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		var index struct {
			Generation json.RawMessage `json:"generation"`
		}
		if e = json.Unmarshal(b, &index); e != nil {
			return "", fmt.Errorf("%s backlinks index: %w", level, e)
		}
		if len(index.Generation) == 0 {
			return "", fmt.Errorf("%s backlinks index has no generation", level)
		}
		generations[level] = index.Generation
	}
	if len(generations) == 0 {
		return "", nil
	}
	b, e := json.Marshal(generations)
	return string(b), e
}
