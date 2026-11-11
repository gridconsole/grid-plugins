package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

var settingsFile = regexp.MustCompile(`(?i)^(?:settings\.json|\.settings\.lock)$`)
var gateFile = regexp.MustCompile(`(?i)^(?:task\.md|workflows?\.md|approval(?:-ship|-preview)?\.json|refusal-R-[1-9][0-9]*\.json|\.task-write\.json|\.task\.lock)$`)

// Each text is one plain sentence for one kind of file. It names the file the
// agent gave (the %s) and the grid verb the agent uses instead.
const gateReason = "%s is a task record that grid writes; use the grid task or grid question command instead of the Write or Edit tool"

const switchReason = "%s holds the person's Grid settings; grid hook level and grid settings change it"

const feedReason = "%s is the task's Feed; post to it with grid feed or grid job"

const hostReason = "%s is in Grid's host folder, which only Grid writes"

// gridRoots are the roots a task or a settings file can sit under, resolved and
// lower-case: the projects folder, and the Grid home beside it that also holds
// personal tasks.
func (c context) gridRoots() []string {
	roots := []string{}
	add := func(p string) {
		p = strings.ToLower(realNative(p, 0))
		for _, seen := range roots {
			if seen == p {
				return
			}
		}
		roots = append(roots, p)
	}
	add(c.projects)
	add(filepath.Dir(c.projects))
	if c.home != "" {
		add(resolve(c.root, c.expand(c.home)))
	}
	return roots
}

// settingsAt reports whether target (lower-case) is <home>/settings.json, its
// lock, or the same pair in one project's folder, under one grid root.
func settingsAt(root, target string) bool {
	if !inside(root, target) {
		return false
	}
	// <home>/<file> and <projects>/<name>/<file>, which the home root reads as projects/<name>/<file>.
	rel := strings.Split(relative(root, target), "/")
	return (len(rel) == 1 && settingsFile.MatchString(rel[0])) ||
		(len(rel) == 2 && settingsFile.MatchString(rel[1])) ||
		(len(rel) == 3 && rel[0] == "projects" && settingsFile.MatchString(rel[2]))
}

// Reports whether target (lower-case) is a task record that grid writes, under one
// grid root: its questions folder, its plan, preview and ship approvals, a refusal
// decision or its plan revisions. The home root finds personal tasks, which sit
// beside projects.
func gateAt(root, target string) bool {
	if projectWorkflowAt(root, target) {
		return true
	}
	t := taskAtFold(target, root)
	if t == nil {
		return false
	}
	first, _, _ := strings.Cut(t.rel, "/")
	return first == "questions" || first == "plan-revisions" || (t.rel == first && gateFile.MatchString(first))
}

// projectWorkflowAt recognizes workspace and project policy files, including
// aliases after the caller resolves the target path.
func projectWorkflowAt(root, target string) bool {
	if !inside(root, target) || filepath.Base(target) != "workflows.md" {
		return false
	}
	rel := strings.Split(relative(root, target), "/")
	return len(rel) == 1 || len(rel) == 2 || (len(rel) == 3 && rel[0] == "projects")
}

// hostFolder is the legacy .host: beside the data folders, two levels above a
// projects folder that sits in <grid>/<name>/projects.
func hostFolder(projects string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(resolve(".", projects))), ".host")
}

// homeHostFolder is .host inside the single Grid home, the parent of projects.
func homeHostFolder(projects string) string {
	return filepath.Join(filepath.Dir(resolve(".", projects)), ".host")
}

// EvaluateHost watches legacy .host, the task records grid writes, the person's
// settings files and the new task Feed, which contains the answer and approval
// records (feed-contract.md:17-29), against the file tools. It does not read
// shell text: a Bash command is never answered here, and Claude Code's own
// permission rules cover what a shell may write. The plan guard switch does not
// reach it; its level is the host-guard setting, warn by default.
func EvaluateHost(in hookio.Input, root string, env map[string]string) hookio.Decision {
	return configuredLevel("host-guard", in, root, env, evaluateHost(in, root, env))
}

func evaluateHost(in hookio.Input, root string, env map[string]string) hookio.Decision {
	c := newContext(in, root, env)
	if c.projects == "" || !fileTool(in.ToolName) {
		return hookio.Decision{}
	}
	host := homeHostFolder(c.projects)
	given := c.target()
	if given == "" {
		return hookio.Decision{}
	}
	inHost := func() hookio.Decision { return hookio.Deny(fmt.Sprintf(hostReason, given)) }
	person := func() hookio.Decision { return hookio.Deny(fmt.Sprintf(gateReason, given)) }
	switches := func() hookio.Decision { return hookio.Deny(fmt.Sprintf(switchReason, given)) }
	feed := func() hookio.Decision { return hookio.Deny(fmt.Sprintf(feedReason, given)) }
	target := strings.ToLower(c.path(given))
	realProjects := realNative(c.projects, 0)
	legacy := hostFolder(c.projects)
	// Each folder is judged by its resolved path and named as the guard derived it.
	folders := []struct{ named, resolved string }{
		{host, host}, {legacy, legacy},
		{host, homeHostFolder(realProjects)}, {legacy, hostFolder(realProjects)},
	}
	if c.home != "" {
		folders = append(folders, struct{ named, resolved string }{host, filepath.Join(resolve(c.root, c.expand(c.home)), ".host")})
	}
	for _, folder := range folders {
		if inside(strings.ToLower(realNative(folder.resolved, 0)), target) {
			return inHost()
		}
	}
	for _, root := range c.gridRoots() {
		if settingsAt(root, target) {
			return switches()
		}
		if gateAt(root, target) {
			return person()
		}
	}
	// Compare the native resolved path as well as its lower-case form. The latter
	// catches case aliases even on case-sensitive hosts used for tests.
	if t := taskAtFold(target, strings.ToLower(realNative(c.projects, 0))); t != nil && (t.rel == "feed" || strings.HasPrefix(t.rel, "feed/")) {
		return feed()
	}
	return hookio.Decision{}
}

func taskAtFold(p, root string) *task {
	rel := relative(root, p)
	parts := strings.Split(rel, "/")
	if !inside(root, p) || len(parts) < 3 || parts[1] != "tasks" || !taskName.MatchString(strings.ToUpper(parts[2])) {
		return nil
	}
	return &task{filepath.Join(root, filepath.Join(parts[:3]...)), strings.Join(parts[3:], "/")}
}
