package guard

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

var namesHost = regexp.MustCompile(`(?i)(^|[^\w.-])\.host($|[^\w.-])`)

// gateVerb is a grid verb that writes a gate: an approval, an answer, a refusal
// decision or a task state change. It is matched on flatten's output, where every
// shell separator is " ; " and quotes are gone, so the binary may sit behind an
// env prefix, &&, ;, a subshell, a path or quoting.
var gateVerb = regexp.MustCompile(`(?i)(?:^|\s)(?:\S*/)?grid(?:\.exe)?\s+(?:approve|answer|question\s+(?:answer|withdraw)|refusal\s+decide|task\s+(?:status|close|reopen|stop))(?:\s|$)`)

// feedAnswer matches answer posts and withdrawn controls in either flag order.
// The lazy middle spans flattened separators, since a quoted body may hold some.
const feedPostCommand = `(?:^|\s)(?:\S*/)?grid(?:\.exe)?\s+(?:feed|job)\s+post`
const withdrawnEvent = `question_event\s*:\s*withdrawn(?:\s|,|$)`

var feedAnswer = regexp.MustCompile(`(?i)` + feedPostCommand +
	`(?:\s.*?)?\s-{1,2}type(?:=|\s+)(?:answer(?:\s|$)|control(?:\s|$).*?` + withdrawnEvent + `)|` +
	feedPostCommand + `\s.*?` + withdrawnEvent + `.*?\s-{1,2}type(?:=|\s+)control(?:\s|$)`)

// gateName is a task entry that holds a gate: the questions folder with its
// answers, the plan and ship approvals, a refusal decision, the write journal and its lock, or a
// plan revision archive.
var gateName = regexp.MustCompile(`(?i)(?:^|[^\w.-])(?:approval(?:-ship)?\.json|refusal-R-[1-9][0-9]*\.json|\.task-write\.json|\.task\.lock|plan-revisions|questions)(?:$|[^\w.-])`)

// The task's workflow snapshot governs which roles may post gates.
var workflowName = regexp.MustCompile(`(?i)(?:^|[^\w.-])workflow\.md(?:$|[^\w.-])`)

// switchVerb is a grid verb that changes which guards or agents run, and
// settingsName is a Grid settings file or its lock, where the same switches live.
var switchVerb = regexp.MustCompile(`(?i)(?:^|\s)(?:\S*/)?grid(?:\.exe)?\s+(?:hook\s+level|agent\s+set|skill\s+(?:enable|disable)|setup)(?:\s|$)`)
var settingsName = regexp.MustCompile(`(?i)(?:^|[^\w.-])(?:settings\.json|\.settings\.lock)(?:$|[^\w.-])`)
var settingsFile = regexp.MustCompile(`(?i)^(?:settings\.json|\.settings\.lock)$`)
var gateFile = regexp.MustCompile(`(?i)^(?:approval(?:-ship)?\.json|refusal-R-[1-9][0-9]*\.json|\.task-write\.json|\.task\.lock)$`)

// readOnly are the commands that may name a gate path: none of them writes a file
// when the command has no redirect and, for find, no action flag.
var readOnly = map[string]bool{"cat": true, "head": true, "tail": true, "grep": true, "egrep": true, "fgrep": true, "rg": true,
	"ls": true, "wc": true, "stat": true, "file": true, "diff": true, "cmp": true, "jq": true, "find": true, "nl": true, "tac": true,
	"cut": true, "echo": true, "printf": true, "cd": true, "pwd": true, "test": true, "[": true, "true": true}
var findWrites = map[string]bool{"-delete": true, "-exec": true, "-execdir": true, "-ok": true, "-okdir": true, "-fprint": true, "-fprintf": true, "-fls": true}

const gateReason = "Gates are released by the person or the app, never by an agent: questions/ and its answers, approval.json, approval-ship.json, refusal-R-n.json, .task-write.json, .task.lock and plan-revisions/ are written only by Grid, and grid approve, grid answer, grid feed post --type answer, grid job post --type answer, grid question withdraw, grid refusal decide and grid task status|close|reopen|stop are the person's verbs; an agent cannot approve or answer its own gate. Reading is allowed: cat, grep, ls, head or tail of questions/ and the gate files work."

const switchReason = "The guards and agent settings are the person's: <home>/settings.json, projects/<name>/settings.json and .settings.lock are written only by Grid, and grid hook level, grid agent set, grid skill enable|disable and grid setup are the person's verbs; an agent cannot turn a guard off. Reading is allowed: cat, grep, ls, head or tail of a settings file works. Use the Write or Edit tool for any other settings.json."

// flatten turns a shell command into words a regexp can read: quotes and escapes
// disappear so gr”id is grid, and every separator becomes " ; ". With slash a
// backslash becomes a slash, which reads a Windows path.
func flatten(command string, slash bool) string {
	var b strings.Builder
	for i := 0; i < len(command); i++ {
		switch ch := command[i]; ch {
		case '\'', '"':
		case '\\':
			if i+1 < len(command) && command[i+1] == '\n' {
				i++
				b.WriteByte(' ')
			} else if slash {
				b.WriteByte('/')
			}
		case '$':
			if i+1 >= len(command) || (command[i+1] != '\'' && command[i+1] != '"') {
				b.WriteByte(ch)
			}
		case '`', ';', '&', '|', '(', ')', '{', '}', '<', '>', '\n', '\r':
			b.WriteString(" ; ")
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// flattened returns both readings of the command: backslashes dropped and kept as slashes.
func flattened(command string) [2]string {
	return [2]string{flatten(command, false), flatten(command, true)}
}

// runsVerb reports whether the command runs a grid verb that re matches.
func runsVerb(command string, re *regexp.Regexp) bool {
	for _, text := range flattened(command) {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// writesNamed reports whether the command names a path that re matches and is not plainly a
// read. Textual, like the rest of this guard: a command that cannot be shown to
// only read is refused, so a script that writes the path indirectly is not caught.
func writesNamed(command string, re *regexp.Regexp) bool {
	for _, text := range flattened(command) {
		if re.MatchString(text) && !onlyReads(command, text) {
			return true
		}
	}
	return false
}

// onlyReads is true when the command has no output redirect and every command it
// runs, after env assignments, is a read-only command.
func onlyReads(command, flat string) bool {
	if strings.Contains(command, ">") {
		return false
	}
	for _, segment := range strings.Split(flat, ";") {
		words := strings.Fields(segment)
		for len(words) > 0 && strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "=") {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		name := strings.ToLower(filepath.Base(words[0]))
		if !readOnly[name] {
			return false
		}
		if name == "find" {
			for _, w := range words[1:] {
				if findWrites[w] {
					return false
				}
			}
		}
	}
	return true
}

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

// gateAt reports whether target (lower-case) is a gate entry of a task under one
// grid root: its questions folder, its plan and ship approvals, a refusal decision or its plan
// revisions. The home root finds personal tasks, which sit beside projects.
func gateAt(root, target string) bool {
	t := taskAtFold(target, root)
	if t == nil {
		return false
	}
	first, _, _ := strings.Cut(t.rel, "/")
	return first == "questions" || first == "plan-revisions" || (t.rel == first && gateFile.MatchString(first))
}

var namesFeed = regexp.MustCompile(`(?i)(?:^|[^\w.-])(?:projects/[^\s'"/]+/tasks/T-\d[^\s'"/]*/feed|feed)(?:/|[\s'";|&<>]|$)`)

// hostFolder is the legacy .host: beside the data folders, two levels above a
// projects folder that sits in <grid>/<name>/projects.
func hostFolder(projects string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(resolve(".", projects))), ".host")
}

// homeHostFolder is .host inside the single Grid home, the parent of projects.
func homeHostFolder(projects string) string {
	return filepath.Join(filepath.Dir(resolve(".", projects)), ".host")
}

// EvaluateHost protects legacy .host and the new task Feed, which contains the
// answer and gate records (feed-contract.md:17-29). It cannot be disabled by the
// plan guard switch. Shell checks are deliberately textual, not a sandbox.
func EvaluateHost(in hookio.Input, root string, env map[string]string) hookio.Decision {
	c := newContext(in, root, env)
	if c.projects == "" {
		return hookio.Decision{}
	}
	host := homeHostFolder(c.projects)
	deny := func(what string) hookio.Decision {
		return hookio.Deny("The host's folder is written only by Grid, so " + what + ". A held plan is approved by the person in Grid, never by a file an agent writes.")
	}
	person := func() hookio.Decision { return hookio.Deny(gateReason) }
	switches := func() hookio.Decision { return hookio.Deny(switchReason) }
	if in.ToolName == "Bash" {
		command := str(in.ToolInput["command"])
		if c.set && namesHost.MatchString(command) {
			return deny("this command names " + host)
		}
		// A plain grid feed invocation uses the CLI's writer contract. Shell
		// operators prevent this exemption, so a trailing raw write stays fenced.
		tokens, simple := words(command)
		feedCLI := simple && len(tokens) >= 2 && filepath.Base(tokens[0]) == "grid" && tokens[1] == "feed"
		if c.set && (runsVerb(command, switchVerb) || writesNamed(command, settingsName)) {
			return switches()
		}
		if c.set && (runsVerb(command, gateVerb) || runsVerb(command, feedAnswer) || writesNamed(command, gateName) || (namesFeed.MatchString(command) && !feedCLI)) {
			return person()
		}
		if c.set && writesNamed(command, workflowName) {
			return hookio.Deny("The task's workflow.md policy is written only by Grid; an agent cannot change its own gate posting roles. Reading is allowed: cat, grep, ls, head or tail of workflow.md work.")
		}
		return hookio.Decision{}
	}
	if !fileTool(in.ToolName) {
		return hookio.Decision{}
	}
	given := c.target()
	if given == "" {
		return hookio.Decision{}
	}
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
			return deny(given + " is in " + folder.named)
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
		return person()
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
