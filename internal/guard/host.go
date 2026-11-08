package guard

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

var namesHost = regexp.MustCompile(`(?i)(^|[^\w.-])\.host($|[^\w.-])`)
var namesPersonCommand = regexp.MustCompile(`(^|[\s;&|])(?:[^\s;&|]*/)?grid\s+(?:approve|answer)(?:\s|$)`)
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
	person := func() hookio.Decision {
		return hookio.Deny("Approval, answer and gate records are written only by a person or the app through grid approve and grid answer; an agent cannot approve its own gate.")
	}
	if in.ToolName == "Bash" {
		command := str(in.ToolInput["command"])
		if c.set && namesHost.MatchString(command) {
			return deny("this command names " + host)
		}
		// A plain grid feed invocation uses the CLI's writer contract. Shell
		// operators prevent this exemption, so a trailing raw write stays fenced.
		tokens, simple := words(command)
		feedCLI := simple && len(tokens) >= 2 && filepath.Base(tokens[0]) == "grid" && tokens[1] == "feed"
		if c.set && (namesPersonCommand.MatchString(command) || (namesFeed.MatchString(command) && !feedCLI)) {
			return person()
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
