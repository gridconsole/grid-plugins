// Package guard evaluates hooks without writing files, changing the process
// environment, or contacting services. Evaluators read the supplied filesystem
// snapshot; callers own serialization and any bookkeeping.
package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
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
	if c.home == "" && c.projects == "" && env["HOME"] != "" {
		c.home = filepath.Join(env["HOME"], ".grid")
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
			return filepath.Join(append([]string{onDiskCase(real)}, rest...)...)
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

// onDiskCase rewrites each component of an existing, symlink-free absolute path
// to the name its directory lists. EvalSymlinks keeps the case it was given, so
// on a case-folding volume (the macOS default) ~/.GRID would otherwise differ
// from ~/.grid. A component that cannot be listed keeps its given spelling.
func onDiskCase(p string) string {
	vol := filepath.VolumeName(p)
	rest := strings.TrimLeft(p[len(vol):], string(filepath.Separator))
	cur := vol + string(filepath.Separator)
	if rest == "" {
		return p
	}
	for _, name := range strings.Split(rest, string(filepath.Separator)) {
		cur = filepath.Join(cur, storedName(cur, name))
	}
	return cur
}

// storedName returns the entry of dir that name refers to, as dir lists it.
func storedName(dir, name string) string {
	if plain(name) {
		// On a case-sensitive volume the swapped spelling is a different, usually
		// missing, entry; skip listing a directory that may hold thousands.
		swapped := swapCase(name)
		if swapped == name {
			return name
		}
		if _, err := os.Lstat(filepath.Join(dir, swapped)); err != nil {
			return name
		}
	}
	d, err := os.Open(dir)
	if err != nil {
		return name
	}
	defer d.Close()
	want, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		return name
	}
	// The path exists, so the listing holds this exact spelling or the entry the
	// volume folds it to (case, or Unicode form). Stop at the first entry that is
	// provably the same file rather than reading a large directory to the end.
	for {
		names, err := d.Readdirnames(256)
		for _, n := range names {
			if n == name {
				return name
			}
			if plain(name) && !strings.EqualFold(n, name) {
				continue
			}
			if info, err := os.Lstat(filepath.Join(dir, n)); err == nil && os.SameFile(want, info) {
				return n
			}
		}
		if err != nil {
			return name
		}
	}
}

func plain(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
func swapCase(s string) string {
	b := []byte(s)
	for i, ch := range b {
		switch {
		case 'a' <= ch && ch <= 'z':
			b[i] = ch - 'a' + 'A'
		case 'A' <= ch && ch <= 'Z':
			b[i] = ch - 'A' + 'a'
		}
	}
	return string(b)
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

const planAcceptedMark = "· plan accepted ·"
const previewApproved = "· preview approved ·"
const planReady = "· plan ready ·"

var acceptedLine = regexp.MustCompile(`(?m)^- \S+ · plan accepted · `)
var recallLine = regexp.MustCompile(`(?im)^[ \t]*(?:[-*+][ \t]+)?(?:\*\*|__)?(?:Recall|Used)(?:\*\*|__)?[ \t]*:(?:\*\*|__)?[ \t]*[^\s*_]`)
var stepName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func planStage(text, history string) string {
	if acceptedLine.MatchString(history) {
		return ""
	}
	if text == "" {
		return "before"
	}
	fm := taskfile.ParseFrontmatter(text)
	if fm == nil || fm.Odd {
		return "unreadable"
	}
	steps, ok := fm.Data["workflow"].([]any)
	if !ok || len(steps) == 0 {
		return "unreadable"
	}
	current, ok := fm.Data["step"].(string)
	if !ok || current == "" {
		return "unreadable"
	}
	seen := map[string]bool{}
	at, gate := -1, -1
	for i, v := range steps {
		item, ok := v.(map[string]any)
		if !ok {
			return "unreadable"
		}
		s, sok := item["step"].(string)
		k, kok := item["kind"].(string)
		if !sok || !kok || !stepName.MatchString(s) || seen[s] {
			return "unreadable"
		}
		seen[s] = true
		if s == current {
			at = i
		}
		if gate < 0 && k == "approve" && strings.Contains(s, "plan") {
			gate = i
		}
	}
	if gate < 0 {
		return ""
	}
	if at < 0 || at > gate {
		return "past"
	}
	if at == gate {
		return "gate"
	}
	return "before"
}
func heldStage(stage string) bool {
	return stage == "before" || stage == "gate" || stage == "unreadable"
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
	return false
}
func isHeld(dir string) bool {
	t := read(filepath.Join(dir, "task.md"))
	return exists(dir) && !inactive(t) && heldStage(planStage(t, read(filepath.Join(dir, "history.md"))))
}
func gateLike(s string) bool {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return s == "plan accepted" || s == "preview approved" || s == "ship approved"
}

var agentChars = regexp.MustCompile(`[^a-z0-9]+`)

func agentName(s string) string {
	p := strings.Split(s, ":")
	s = strings.Trim(agentChars.ReplaceAllString(strings.ToLower(p[len(p)-1]), "-"), "-")
	if s == "grid-worker" {
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
func (c context) brain(target string) bool {
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
func HostGuard(stdin []byte, env map[string]string) ([]byte, int) {
	return invoke(stdin, env, EvaluateHost)
}
func BriefLint(stdin []byte, env map[string]string) ([]byte, int) {
	return invoke(stdin, env, EvaluateBrief)
}
func HomeAllow(stdin []byte, env map[string]string) ([]byte, int) {
	return invoke(stdin, env, EvaluateHome)
}
