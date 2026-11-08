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

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

type Env struct {
	Resolver    where.Resolver
	Cwd         string
	Home        string
	GridHome    string
	Machine     string // this computer's id; it picks the project's folders
	Stdout      io.Writer
	Stderr      io.Writer
	ResolveTask func(string) (string, error)
}
type flags struct {
	json   bool
	task   string
	dir    string
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

const usage = "Usage: grid job render <task> <item> [--budget N] [--json]\n       grid job lint <file> [--task <task>] [--json]\n       grid job show [<task> <job>|<job>|<file>] [--task <task>] [--json]\n       grid job trace <pointer> [--sha256 <digest>] [--project <name-or-id>] [--json]\n       grid job verify <job> [--task <task>] [--dir <path>] [--json]\n       grid job cycles <items.json> [--json]\n       grid job floor <items.json> [--json] [--policy <policy.json>]Exit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease, invalid record or intent claim token; do not retry), 4 disk full, 5 taken (intent already claimed or done; skip).\n"

func Run(args []string, e Env) int {
	if err := memberContext(e.Resolver); err != nil {
		f := flags{}
		for _, arg := range args {
			f.json = f.json || arg == "--json"
		}
		if len(args) > 0 && args[0] == "trace" && f.json {
			return traceOutput(e, TraceResult{Rows: []TraceRow{}, Problems: []TraceProblem{{Text: err.Error()}}}, true, 2)
		}
		return output(e, f, report{Error: err.Error()}, 2)
	}
	if len(args) > 0 {
		switch args[0] {
		case "trace":
			return runTrace(args[1:], e)
		}
	}
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
		if errors.Is(err, job.ErrPlanNeedsApproval) {
			code = 2
		}
		return output(e, f, report{Error: err.Error(), Findings: []lint.Finding{}}, code)
	}
	if f.dir != "" && action != "verify" {
		return fail(errors.New("--dir is only supported by verify"), 2)
	}
	switch action {
	case "verify":
		return runVerify(args, f, e)
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
		w, err := sources(task, e)
		if err != nil {
			return fail(err, 1)
		}
		generation, err := w.generation()
		if err != nil {
			return fail(err, 1)
		}
		// Check planned instructions before resolving or writing any artifacts.
		meta, err := job.Metadata(j)
		if err != nil {
			return fail(err, 1)
		}
		skeleton := meta + "\n## Intent\n## Job\n## Context\n## Out of scope\n## Open\n## Acceptance\n## Outcome\n## Verdict\n## Follow-up\n"
		findings := lint.Check(skeleton, lint.Options{Resolver: w.files, Repos: w.repos})
		if lint.HasErrors(findings) {
			code := 1
			for _, finding := range findings {
				if finding.Rule == "job.plan-approval" {
					code = 2
				}
			}
			return output(e, f, report{ID: j.ID, Findings: findings}, code)
		}
		rendered, err := job.Render(j, job.RenderOptions{Resolver: w.files, Generation: generation, BrainTokens: f.budget})
		if err != nil {
			return fail(err, 1)
		}
		after, err := w.generation()
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
		w, err := sources(task, e)
		if err != nil {
			return fail(err, 1)
		}
		findings := lint.Check(string(b), lint.Options{Resolver: w.files, Repos: w.repos})
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

func memberContext(resolver where.Resolver) error {
	if strings.TrimSpace(resolver.Caller.Role) == "" {
		return errors.New("missing member context: set Env.Resolver.Caller.Role; use owner for owner access")
	}
	return nil
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
		case a == "--task" || a == "--budget" || a == "--dir":
			if i+1 == len(args) {
				return f, fmt.Errorf("%s needs a value", a)
			}
			i++
			if a == "--task" {
				f.task = args[i]
			} else if a == "--dir" {
				f.dir = args[i]
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

// world is what a Job reads from: the task folder, the brain store and the
// project's own folders on this computer.
type world struct {
	files   *job.Files
	repos   []string
	store   brain.Store
	project string
}

// sources never takes a root from the task folder, which syncs to every member.
// The project's folders and repos come from its record in the Grid home, and the
// owner's role in task.md decides how intent.md is treated.
func sources(task string, e Env) (*world, error) {
	w := &world{project: projectOf(task, e.GridHome)}
	w.store = brain.Store{GridHome: e.GridHome, Project: w.project}
	f := &job.Files{Task: task, Notes: w.store, Repos: map[string]string{}}
	w.files = f
	var prototype string
	b, err := os.ReadFile(filepath.Join(task, "task.md"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		d, err := frontmatter.Parse(string(b))
		if err != nil {
			return nil, fmt.Errorf("task.md: %w", err)
		}
		// owner_role is what grid task new writes; older tasks kept the role in owner.
		role, _ := d.Data["owner_role"].(string)
		if role == "" {
			role, _ = d.Data["owner"].(string)
		}
		f.Owner = ownerTier(role)
		prototype, _ = d.Data["prototype"].(string)
	}
	if w.project != "" {
		if err := roots(w, prototype, e); err != nil {
			return nil, err
		}
	}
	for name := range f.Repos {
		w.repos = append(w.repos, name)
	}
	sort.Strings(w.repos)
	return w, nil
}

// ownerTier maps the role task.md records for the owner to a trust tier. A task
// without a known owner role is unknown provenance, and that is external.
func ownerTier(role string) contract.TrustTier {
	switch contract.NormalizeRole(role) {
	case "member", "owner", "lead":
		return contract.PersonTeam
	case "guest":
		return contract.PersonGuest
	}
	return contract.External
}

// projectOf names the project whose tasks folder holds task, or "".
func projectOf(task, gridHome string) string {
	if gridHome == "" {
		return ""
	}
	rel, err := filepath.Rel(gridhome.Real(gridHome), gridhome.Real(task))
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 4 || parts[0] != "projects" || parts[2] != "tasks" {
		return ""
	}
	return parts[1]
}

// roots reads the project record and sets the folders and repos this computer has.
func roots(w *world, prototype string, e Env) error {
	b, err := os.ReadFile(filepath.Join(e.GridHome, "projects", w.project, "project.md"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	rec, err := project.Parse(w.project, string(b))
	if err != nil {
		return fmt.Errorf("project %s: %w", w.project, err)
	}
	f := w.files
	f.Folders = rec.FoldersOn(e.Machine)
	if len(f.Folders) == 0 {
		return nil
	}
	base := f.Folders[0]
	f.Repo = base
	for i, r := range rec.Repos {
		if !filepath.IsLocal(r.Path) {
			return fmt.Errorf("project %s: repo path %q must stay inside the project folder", w.project, r.Path)
		}
		dir := filepath.Join(base, r.Path)
		if i == 0 {
			f.Repo = dir
		}
		if filepath.Clean(r.Path) != "." {
			f.Repos[filepath.ToSlash(filepath.Clean(r.Path))] = dir
		}
	}
	if prototype != "" {
		f.Prototype = prototype
		if !filepath.IsAbs(prototype) {
			f.Prototype = filepath.Join(base, prototype)
		}
	}
	return nil
}

// generation pins the project brain's index generation; "" when none is built.
func (w *world) generation() (string, error) {
	if w.project == "" {
		return "", nil
	}
	store := w.store
	store.ReadLevel = "project"
	idx, err := store.Index()
	if err != nil {
		return "", err
	}
	g := idx.Generations["project"]
	if g == 0 {
		return "", nil
	}
	b, err := json.Marshal(map[string]uint64{"project": g})
	return string(b), err
}
