// Package guard evaluates hooks without writing files, changing the process
// environment, or contacting services. Evaluators read the supplied filesystem
// snapshot; callers own serialization and any bookkeeping.
package guard

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fspath"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

type context struct {
	in                   hookio.Input
	env                  map[string]string
	root, home, projects string
	set                  bool
}

func newContext(in hookio.Input, root string, env map[string]string) context {
	if root == "" {
		root = "."
	}
	root = resolve(".", root)
	if in.Cwd == "" {
		in.Cwd = root
	} else {
		in.Cwd = resolve(root, in.Cwd)
	}
	c := context{in: in, env: env, root: root, home: env["GRID_HOME"], projects: env["GRID_PROJECTS"]}
	c.set = c.home != "" || c.projects != ""
	// The new CLI has one Grid home, independent of desktop workspace config.
	// gridhome.Resolve is the one place that reads GRID_HOME, the shared config and the default.
	if c.projects == "" {
		c.home = gridhome.Resolve(func(key string) string { return env[key] }, env["HOME"]).Dir
	}
	if c.projects == "" && c.home != "" {
		c.projects = filepath.Join(c.home, "projects")
	}
	if c.projects != "" {
		c.projects = resolve(root, c.projects)
	}
	return c
}
func str(v any) string        { s, _ := v.(string); return s }
func read(file string) string { b, _ := os.ReadFile(file); return string(b) }
func exists(file string) bool { _, err := os.Stat(file); return err == nil }
func inside(root, file string) bool {
	r, err := filepath.Rel(root, file)
	return err == nil && (r == "." || (r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)))
}
func resolve(cwd, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	a, err := filepath.Abs(p)
	if err == nil {
		return a
	}
	return filepath.Clean(p)
}
func (c context) expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(c.env["HOME"], p[1:])
	}
	return p
}
func (c context) path(p string) string { return realNative(resolve(c.in.Cwd, c.expand(p)), 0) }

// realNative resolves symlinks and returns each existing component in the case
// the directory stores, like fs.realpathSync.native: every containment check
// compares these paths as plain strings, so two spellings of one folder must
// come out the same. A missing tail is kept as given, below its nearest
// existing parent.
func realNative(p string, depth int) string {
	p = resolve(".", p)
	rest := []string{}
	for cur := p; ; cur = filepath.Dir(cur) {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(append([]string{fspath.OnDiskCase(real)}, rest...)...)
		}
		if depth < 40 {
			if link, err := os.Readlink(cur); err == nil {
				from := realNative(filepath.Dir(cur), depth+1)
				dest := realNative(resolve(from, link), depth+1)
				return filepath.Join(append([]string{dest}, rest...)...)
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = append([]string{filepath.Base(cur)}, rest...)
	}
}

func (c context) target() string {
	if c.in.ToolName == "NotebookEdit" {
		return str(c.in.ToolInput["notebook_path"])
	}
	return str(c.in.ToolInput["file_path"])
}
func fileTool(name string) bool {
	switch name {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return true
	}
	return false
}

// Guarded reports whether a tool is one the guards can refuse (file tools, Bash
// and subagent launches). A call that does not name its tool counts as guarded:
// the guards cannot tell, so they must not let it through unread.
func Guarded(tool string) bool {
	return tool == "" || fileTool(tool) || tool == "Bash" || tool == "Agent" || tool == "Task"
}

type task struct{ dir, rel string }

var taskName = regexp.MustCompile(`^T-\d+`)

func taskAt(p, projects string) *task {
	if p == "" || projects == "" {
		return nil
	}
	r, err := filepath.Rel(realNative(projects, 0), realNative(p, 0))
	if err != nil || filepath.IsAbs(r) {
		return nil
	}
	parts := strings.Split(r, string(filepath.Separator))
	if len(parts) < 3 || parts[0] == ".." || parts[1] != "tasks" || !taskName.MatchString(parts[2]) {
		return nil
	}
	return &task{filepath.Join(resolve(".", projects), filepath.Join(parts[:3]...)), strings.Join(parts[3:], "/")}
}

// stepLookup resolves the steps of one task from the task.md text it is given.
type stepLookup = taskstore.StepLookup

// stepsFor resolves a task's steps through its workflow, named templates
// included, with the caller's resolver: the lookup Deliver builds too.
func stepsFor(env map[string]string, taskDir string) stepLookup {
	return taskstore.StepsFor(callerResolver(env), taskDir)
}

// planStage reads the inline list alone; it backs the PlanStage export the
// session hooks use, which have no Grid home to resolve a named workflow from.
func planStage(text, history string) string { return taskstore.PlanStage(text, history) }

// held reports that a task's plan is still being written (no plan accepted).
// One rule reads it: the grid-planner's reach into a task folder. It blocks no
// repo write, and no rule blocks a task's agents because its plan is not accepted.
func held(lookup stepLookup, dir, text, history string) bool {
	return taskstore.PlanHeldWith(lookup, dir, text, history)
}
func inactive(text string) bool {
	f := taskfile.ParseFrontmatter(text)
	if f == nil {
		return false
	}
	switch str(f.Data["status"]) {
	case "closed", "paused", "pausing":
		return true
	}
	// `grid task close` writes the state key, not the old status one.
	switch str(f.Data["state"]) {
	case "closed", "done", "cancelled":
		return true
	}
	return false
}
func gateLike(s string) bool {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return s == "plan accepted" || s == "preview approved" || s == "ship approved"
}

var agentChars = regexp.MustCompile(`[^a-z0-9]+`)

func agentName(s string) string {
	p := strings.Split(s, ":")
	s = strings.Trim(agentChars.ReplaceAllString(strings.ToLower(p[len(p)-1]), "-"), "-")
	if contract.NormalizeRole(s) == "worker" {
		s = "grid-builder"
	}
	if s == "" {
		s = "subagent"
	}
	return s
}

var pointerChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func pointerKey(s string) string {
	s = pointerChars.ReplaceAllString(s, "_")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}
func (c context) pointer(ext string) string {
	id := pointerKey(c.in.SessionID)
	if id == "" || c.env["CLAUDE_PLUGIN_DATA"] == "" {
		return ""
	}
	return filepath.Join(resolve(c.root, c.env["CLAUDE_PLUGIN_DATA"]), "sessions", id+ext)
}
func (c context) taskDirs(text string) []string {
	out := []string{}
	for _, d := range strings.Split(text, "\n") {
		if t := taskAt(d, c.projects); d != "" && t != nil && t.rel == "" && exists(d) {
			out = append(out, d)
		}
	}
	return out
}
func (c context) mainTasks() []string {
	p := c.pointer(".tasks")
	if b, err := os.ReadFile(p); err == nil {
		return c.taskDirs(string(b))
	}
	return c.taskDirs(read(c.pointer("")))
}
func (c context) agentTasks() []string {
	if k := pointerKey(c.in.AgentID); k != "" {
		return c.taskDirs(read(c.pointer(".tasks-" + k)))
	}
	return c.mainTasks()
}
func unique(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		if v != "" && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}

// brainRoot is the brain folder the Grid config names, "" when the key is absent
// or names the Grid home itself (the brain then stays in the home's knowledge/).
func (c context) brainRoot() string {
	if strings.TrimSpace(c.home) == "" || c.env["HOME"] == "" {
		return ""
	}
	home := resolve(c.root, c.expand(c.home))
	h := gridhome.Resolve(func(key string) string {
		if key == "GRID_HOME" {
			return home
		}
		return c.env[key]
	}, c.env["HOME"])
	if h.Brain == "" || filepath.Clean(h.Brain) == filepath.Clean(home) {
		return ""
	}
	return realNative(resolve(c.root, c.expand(h.Brain)), 0)
}

// brain says whether target is a place of the brain a curator or a planner writes:
// the Grid home's and the projects' knowledge/ folders, and, when the Grid config
// names a brain folder, any path in it. The brain folder only widens what a
// curator or a planner may write; it adds nothing to what another agent is
// refused, since the person's notes there are for the agents to edit too.
func (c context) brain(target string) bool {
	if root := c.brainRoot(); root != "" && inside(root, target) {
		switch strings.TrimSpace(c.in.AgentType) {
		case "grid:grid-curator", "grid-curator", "grid:grid-planner", "grid-planner":
			return true
		}
	}
	root := realNative(c.projects, 0)
	parts := strings.Split(relative(root, target), "/")
	if inside(root, target) && len(parts) >= 2 && strings.EqualFold(parts[1], "knowledge") {
		return true
	}
	if strings.TrimSpace(c.home) == "" {
		return false
	}
	home := realNative(resolve(c.root, c.expand(c.home)), 0)
	return inside(home, target) && strings.EqualFold(strings.Split(relative(home, target), "/")[0], "knowledge")
}
func relative(root, target string) string {
	s, _ := filepath.Rel(root, target)
	if s == "." {
		return ""
	}
	return filepath.ToSlash(s)
}

// Adapter is the stable recording interface. Every guard exits zero, including
// malformed input; a denial is expressed solely in its decision JSON.
type Adapter func(stdin []byte, env map[string]string) (stdout []byte, exit int)

func invoke(data []byte, env map[string]string, eval func(hookio.Input, string, map[string]string) hookio.Decision) (out []byte, exit int) {
	defer func() {
		if recover() != nil {
			out = nil
			exit = 0
		}
	}()
	in := hookio.Decode(data)
	if in == nil {
		return nil, 0
	}
	return eval(*in, env["PWD"], env).Bytes(), 0
}
func PlanGuard(stdin []byte, env map[string]string) ([]byte, int) {
	return invoke(stdin, env, EvaluatePlan)
}
func HomeAllow(stdin []byte, env map[string]string) ([]byte, int) {
	return invoke(stdin, env, EvaluateHome)
}
