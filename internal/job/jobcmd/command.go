// Package jobcmd implements grid job render, lint and show without depending on
// the CLI dispatcher or the task-file writer.
package jobcmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

type Env struct {
	// Context stops job verify in place of its SIGINT handler; nil installs the handler.
	Context     context.Context
	Resolver    where.Resolver
	Cwd         string
	Home        string
	GridHome    string
	Machine     string // this computer's id; it picks the project's folders
	Stdout      io.Writer
	Stderr      io.Writer
	ResolveTask func(string) (string, error)
	project     string // --project value; set by Run, passed to the resolver as Query.Project
}
type flags struct {
	json    bool
	project string
	task    string
	dir     string
	budget  int
	args    []string
	help    bool
}
type report struct {
	OK       bool           `json:"ok"`
	ID       string         `json:"id,omitempty"`
	Path     string         `json:"path,omitempty"`
	Inputs   string         `json:"inputs,omitempty"`
	Job      *job.Job       `json:"job,omitempty"`
	State    string         `json:"state,omitempty"`
	Reason   string         `json:"state_reason,omitempty"`
	Brief    string         `json:"brief,omitempty"`
	Record   string         `json:"record,omitempty"` // id of the Job's job record on the task's Feed; the `re` of a found body
	Findings []lint.Finding `json:"findings"`
	Error    string         `json:"error,omitempty"`
	Code     string         `json:"code,omitempty"`
	fromFeed bool           // show resolved a task, so the text output names the state
}

// refusalCode is the stable code (contract/README.md) of a refusal that exits
// with exit: an unknown project or task, else a usage refusal. Any other failure
// is unclassified and has no code, so the key is left out.
func refusalCode(err error, exit int) string {
	switch {
	case errors.Is(err, where.ErrProjectUnregistered), errors.Is(err, errTraceProjectNotFound):
		return "no_such_project"
	case errors.Is(err, where.ErrTaskNotFound):
		return "no_such_task"
	case exit == 2:
		return "invalid_argument"
	}
	return ""
}

// stderrWithJSON reports whether a refusal with code also prints its stderr line
// under --json: the unknown task and unknown project refusals the walks read
// (internal/cli/gg_unknown_task_walk_test.go). Other refusals, and any refusal
// to a guest, keep an empty stderr under --json as their tests pin.
func stderrWithJSON(e Env, code string) bool {
	return (code == "no_such_task" || code == "no_such_project") && !e.Resolver.Caller.IsGuest()
}

const usage = "Usage: grid job render <task> <item> [--budget N] [--json]\n       grid job lint <file> [--task <task>] [--project <name>] [--json]\n       grid job show [<task> <job>|<job>|<file>] [--task <task>] [--project <name>] [--json]\n       grid job trace <pointer> [--sha256 <digest>] [--project <name-or-id>] [--json]\n       grid job verify <job> [--task <task>] [--project <name>] [--dir <path>] [--json]\n       grid job cycles <items.json> [--json]\nExit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease, invalid record or intent claim token; do not retry), 4 disk full, 5 taken (intent already claimed or done; skip), 6 stale rev (read again), 7 not versioned (nothing written).\n"

func Run(args []string, e Env) int {
	if err := memberContext(e.Resolver); err != nil {
		f := flags{}
		for _, arg := range args {
			f.json = f.json || arg == "--json"
		}
		if len(args) > 0 && args[0] == "trace" && f.json {
			return traceOutput(e, TraceResult{Rows: []TraceRow{}, Problems: []TraceProblem{{Text: err.Error()}}, Code: refusalCode(err, 2)}, true, 2)
		}
		return output(e, f, report{Error: err.Error(), Code: refusalCode(err, 2)}, 2)
	}
	if len(args) > 0 {
		switch args[0] {
		case "trace":
			return runTrace(args[1:], e)
		}
	}
	f, err := parse(args)
	if err != nil {
		return output(e, f, report{Error: err.Error(), Code: refusalCode(err, 2), Findings: []lint.Finding{}}, 2)
	}
	if f.help || len(f.args) == 0 {
		if _, err := io.WriteString(e.Stdout, usage); err != nil {
			return 1
		}
		return 0
	}
	e.project = f.project
	action, args := f.args[0], f.args[1:]
	fail := func(err error, code int) int {
		return output(e, f, report{Error: err.Error(), Code: refusalCode(err, code), Findings: []lint.Finding{}}, code)
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
		j, err := loadJob(task, args[1])
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
		findings := lint.Check(skeleton, w.lintOptions())
		if lint.HasErrors(findings) {
			return output(e, f, report{ID: j.ID, Findings: findings}, 1)
		}
		agent, _, _ := w.store.AgentFor(j.Role) // the brain's agent for the role, if it holds one
		rendered, err := job.Render(j, job.RenderOptions{Resolver: w.files, Generation: generation, BrainTokens: f.budget, Recall: w.recall(j), Agent: agent})
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
		path, wrote, err := saveReport(task, j, rendered)
		if err != nil {
			return fail(err, 1)
		}
		if wrote {
			// save has returned and holds no lock; AppendHistory takes .task.lock itself.
			// A history failure never fails the render.
			for _, id := range brainGiven(j, rendered.Inputs) {
				if err := taskstore.AppendHistoryWhy(task, "brain given", id, "grid job render", j.ID, givenWhy(j, id)); err != nil {
					fmt.Fprintf(e.Stderr, "grid job render: history: %v\n", err)
				}
			}
		}
		return output(e, f, report{OK: true, ID: j.ID, Path: display(path, e), Inputs: display(filepath.Join(filepath.Dir(path), "inputs.json"), e), Findings: findings}, 0)
	case "lint":
		if len(args) != 1 || f.budget != 0 {
			return fail(errors.New("lint requires <file>"), 2)
		}
		path := absolute(e.Cwd, args[0])
		// An unregistered --project is refused before the file is read, as the
		// other verbs refuse it; a guest keeps the answer it gets without the flag.
		if f.project != "" && !e.Resolver.Caller.IsGuest() {
			if _, err := e.Resolver.Resolve(where.Query{Cwd: e.Cwd, Project: f.project}); errors.Is(err, where.ErrProjectUnregistered) {
				return fail(err, 1)
			}
		}
		// A guest is admitted through the task the file lies in, before the file is
		// read, and --task must name that same task.
		guestTask := ""
		if e.Resolver.Caller.IsGuest() {
			if guestTask, err = guestFile(path, e); err != nil {
				return fail(err, 1)
			}
			if f.task != "" {
				var named string
				if named, err = resolveTask(f.task, e); err != nil {
					return fail(err, 1)
				}
				if named != guestTask {
					return fail(where.ErrTaskNotFound, 1)
				}
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fail(err, 1)
		}
		task := findTask(filepath.Dir(path))
		if guestTask != "" {
			task = guestTask
		}
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
		opts := w.lintOptions()
		var findings []lint.Finding
		if lint.IsJSON(path, b) {
			findings = lint.CheckJSON(b, opts)
		} else {
			findings = lint.Check(string(b), opts)
		}
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
		path, taskDir, err := showPath(args, f, e)
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
		// A saved brief carries no run record; the task's Feed does. A Job that was
		// never posted falls back to the brief, which reads Not started.
		state, why := contract.StateOf(contract.Status(j.Status))
		record := ""
		if taskDir != "" {
			read, err := readFeed(taskDir, feed.Filter{})
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fail(err, 1)
			}
			if err == nil {
				if posted := feed.FoldRead(read).Jobs[j.ID]; posted != nil {
					state, why = posted.State, posted.StateReason
				}
				record = jobRecordID(read.Records, j.ID)
			}
		}
		return output(e, f, report{OK: true, ID: j.ID, Path: display(path, e), Job: &j, State: state, Reason: why, Record: record, fromFeed: taskDir != "", Brief: string(b), Findings: []lint.Finding{}}, 0)
	default:
		return fail(fmt.Errorf("unknown job command %q; use render, lint or show", action), 2)
	}
}

// jobRecordID is the id of the newest non-stale job record the Feed holds for the
// Job id, the record pickJob chooses; "" when the Job was never posted. A found
// body names it as its re.
func jobRecordID(records []feed.Entry, id string) string {
	record := ""
	for _, e := range records {
		if e.Type != "job" || e.Verdict == "stale" {
			continue
		}
		if jobID, _ := identity(e); jobID == id {
			record = e.ID
		}
	}
	return record
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
		case a == "--task" || a == "--budget" || a == "--dir" || a == "--project":
			if i+1 == len(args) {
				return f, fmt.Errorf("%s needs a value", a)
			}
			i++
			if a == "--task" {
				f.task = args[i]
			} else if a == "--project" {
				f.project = args[i]
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

// brainGiven lists the project brain notes whose text the render pasted into the
// brief: a brain pointer that is not personal (PointerOnly) and was not cut by the
// budget. An anchor or line range names the same note, so each note appears once.
func brainGiven(j job.Job, in job.Inputs) []string {
	cut := map[string]bool{}
	for _, id := range in.NotRendered {
		cut[id] = true
	}
	// A brain pointer the Job only lists under read: or in its intent is named in
	// the brief, not pasted into it, so it is not given.
	listedOnly := map[string]bool{j.Intent.Owner: true, j.Intent.Item: true, j.Intent.Goal: true}
	for _, p := range j.Read {
		listedOnly[p] = true
	}
	for _, p := range append(append([]string{}, j.Keep...), j.Decisions...) {
		delete(listedOnly, p)
	}
	for _, l := range j.Links {
		delete(listedOnly, l.To)
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, s := range in.Sources {
		if !strings.HasPrefix(s.Pointer, "brain:") || s.PointerOnly || cut[s.Pointer] || listedOnly[s.Pointer] {
			continue
		}
		id := s.Pointer
		if i := strings.IndexByte(id, '#'); i >= 0 {
			id = id[:i]
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// givenWhy is the why of the first Job link that names the note id, with an anchor
// after # cut from the link as brainGiven cuts it from the source. A note the Job
// carries through keep: or decisions: with no link has no why.
func givenWhy(j job.Job, id string) string {
	for _, l := range j.Links {
		to := l.To
		if i := strings.IndexByte(to, '#'); i >= 0 {
			to = to[:i]
		}
		if to == id && strings.TrimSpace(l.Why) != "" {
			return l.Why
		}
	}
	return ""
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
		// An unknown task or project reads the same on stderr with and without --json.
		if r.Error != "" && stderrWithJSON(e, r.Code) {
			if _, err := fmt.Fprintf(e.Stderr, "job: %s\n", r.Error); err != nil {
				return 1
			}
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
		if r.State != "" && r.Path != "" && r.fromFeed {
			if !strings.HasSuffix(r.Brief, "\n") {
				b.WriteString("\n")
			}
			if r.Reason != "" {
				fmt.Fprintf(&b, "state: %s (%s)\n", r.State, r.Reason)
			} else {
				fmt.Fprintf(&b, "state: %s\n", r.State)
			}
		}
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
	// A local plan.md folder or a task inferred from the cwd keeps its path for
	// owners and members. For guests it is only a selector: the project-membership
	// resolver decides, so no path form answers differently from an unknown task.
	local := ""
	p := absolute(e.Cwd, s)
	if isTaskDir(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		local = abs
	} else if s == "" {
		local = findTask(e.Cwd)
	}
	if local != "" {
		if !e.Resolver.Caller.IsGuest() {
			return local, nil
		}
		s = local
	}
	var err error
	if e.ResolveTask != nil && e.project == "" {
		p, err = e.ResolveTask(s)
	} else {
		var result where.Result
		result, err = e.Resolver.Resolve(where.Query{Cwd: e.Cwd, Task: s, Project: e.project})
		p = result.TaskPath
	}
	if err != nil {
		// A guest naming a project it is not in gets the answer it gets without the flag.
		if e.project != "" && e.Resolver.Caller.IsGuest() && errors.Is(err, where.ErrProjectUnregistered) {
			err = where.ErrTaskNotFound
		}
		return "", err
	}
	if p != "" {
		return p, nil
	}
	return "", fmt.Errorf("cannot find task %q; pass a task folder (one holding task.md, intent.md or plan.md)", s)
}

// isTaskDir reports whether dir is a task folder: it holds task.md, intent.md or
// plan.md. A task with no plan.md is still a task.
func isTaskDir(dir string) bool {
	for _, name := range []string{"task.md", "intent.md", "plan.md"} {
		if s, err := os.Stat(filepath.Join(dir, name)); err == nil && !s.IsDir() {
			return true
		}
	}
	return false
}
func findTask(dir string) string {
	for {
		if isTaskDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// guestFile admits a guest to a file by path only through the task folder it lies
// in: the project-membership resolver decides, so a file in an uninvited project,
// in no task or outside the Grid home answers as an unknown task, and a link
// that leaves the task is not followed. It returns the task folder.
func guestFile(path string, e Env) (string, error) {
	task, err := resolveTask(filepath.Dir(path), e)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return task, nil // the read reports a file that is not there
	}
	if rel, err := filepath.Rel(task, real); err != nil || !filepath.IsLocal(rel) {
		return "", where.ErrTaskNotFound
	}
	return task, nil
}

// showPath returns the brief file and, when a task was resolved, the task folder.
func showPath(args []string, f flags, e Env) (string, string, error) {
	if len(args) == 1 && !job.ValidID(args[0]) {
		p := absolute(e.Cwd, args[0])
		if e.Resolver.Caller.IsGuest() {
			if _, err := guestFile(p, e); err != nil {
				return "", "", err
			}
		}
		if s, err := os.Stat(p); err == nil && !s.IsDir() {
			return p, "", nil
		}
	}
	taskArg, id := f.task, ""
	if len(args) == 2 {
		if taskArg != "" {
			return "", "", errors.New("give the task once")
		}
		taskArg, id = args[0], args[1]
	} else if len(args) == 1 {
		id = args[0]
	}
	task, err := resolveTask(taskArg, e)
	if err != nil {
		return "", "", err
	}
	if id == "" {
		paths, err := filepath.Glob(filepath.Join(task, "jobs", "J-*", "brief.md"))
		if err != nil {
			return "", "", err
		}
		if len(paths) != 1 {
			return "", "", fmt.Errorf("task has %d saved Jobs; specify a Job id", len(paths))
		}
		p, err := job.Within(task, strings.TrimPrefix(paths[0], task+string(filepath.Separator)))
		return p, task, err
	}
	if !job.ValidID(id) {
		return "", "", errors.New("invalid Job id")
	}
	p, err := job.Within(task, filepath.Join("jobs", id, "brief.md"))
	return p, task, err
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

// lintOptions is what lint checks a Job against: the task's files and the project's repos.
func (w *world) lintOptions() lint.Options {
	return lint.Options{Resolver: w.files, Repos: w.repos, KnownRole: w.knownRole}
}

// knownRole is true when an agent runs a Job of the normalized role: the plugin's
// agents, the catalog, or an agent note in the project or home brain.
func (w *world) knownRole(role string) bool {
	if _, ok := gridplugins.AgentFor(role); ok {
		return true
	}
	if _, ok := settings.AgentForRole(role); ok {
		return true
	}
	for _, level := range []string{"project", "home"} {
		if level == "project" && w.project == "" {
			continue // no project, no project brain: the brain's prefix refuses an empty one
		}
		names, err := w.store.ConfigNames(level, "agents")
		if err != nil {
			continue
		}
		for _, n := range names {
			if n == "grid-"+role || n == role {
				return true
			}
		}
	}
	return false
}

// LintOptions returns the lint Options that grid job render and grid job lint use for
// the Job of task, so another command can report what render will say.
func LintOptions(task string, e Env) (lint.Options, error) {
	w, err := sources(task, e)
	if err != nil {
		return lint.Options{}, err
	}
	return w.lintOptions(), nil
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
