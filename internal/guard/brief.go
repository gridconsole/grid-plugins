package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

var briefKeys = []string{"id", "role", "status", "verify", "follow_up"}
var briefSections = []string{"Intent", "Job", "Context", "Out of scope", "Open", "Acceptance", "Outcome", "Verdict", "Follow-up"}
var heading = regexp.MustCompile(`^##\s+(.+?)\s*$`)
var bareBrief = regexp.MustCompile(`(?:^|[^\w./-])subagents/([\w.-]+?)\.md`)
var briefName = regexp.MustCompile(`^[\w.-]+$`)

type brief struct{ file, name string }

func briefAt(dir, name, projects, sub string) *brief {
	if !briefName.MatchString(name) || !exists(dir) {
		return nil
	}
	at := taskAt(dir, projects)
	if at == nil || at.rel != "" {
		return nil
	}
	file := filepath.Join(at.dir, sub, name+".md")
	// Only a symlink may move the brief; a prompt that spells its name in another
	// case still names the same file on a case-folding volume.
	if !exists(file) || !strings.EqualFold(realNative(file, 0), filepath.Join(realNative(at.dir, 0), sub, name+".md")) {
		return nil
	}
	return &brief{file, name}
}
func briefInPrompt(prompt, cwd, projects string) *brief {
	if prompt == "" || projects == "" {
		return nil
	}
	prompt = strings.ReplaceAll(prompt, `\`, "/")
	root := `(?:` + regexp.QuoteMeta(filepath.Base(resolve(".", projects))) + `|\$\{?GRID_PROJECTS\}?)`
	full := regexp.MustCompile(root + `/([\w.-]+)/tasks/(T-\d+[\w.-]*)/subagents/([\w.-]+?)\.md`)
	steps := regexp.MustCompile(root + `/([\w.-]+)/tasks/(T-\d+[\w.-]*)/(app-steps(?:/[\w.-]+)*)/([\w.-]+?)\.md`)
	a, b := full.FindStringSubmatchIndex(prompt), steps.FindStringSubmatchIndex(prompt)
	if b != nil && (a == nil || b[0] < a[0]) {
		m := steps.FindStringSubmatch(prompt)
		return briefAt(filepath.Join(projects, m[1], "tasks", m[2]), m[4], projects, m[3])
	}
	if a != nil {
		m := full.FindStringSubmatch(prompt)
		return briefAt(filepath.Join(projects, m[1], "tasks", m[2]), m[3], projects, "subagents")
	}
	if m := bareBrief.FindStringSubmatch(prompt); m != nil {
		if at := taskAt(cwd, projects); at != nil {
			return briefAt(at.dir, m[1], projects, "subagents")
		}
	}
	return nil
}

// LintBrief returns missing fields and sections in the legacy diagnostic order.
func LintBrief(text string) []string {
	fm := taskfile.ScalarFrontmatter(text)
	if fm == nil {
		return []string{"it has no frontmatter block"}
	}
	out := []string{}
	for _, k := range briefKeys {
		if str(fm.Data[k]) == "" {
			out = append(out, "frontmatter field `"+k+"` is missing or empty")
		}
	}
	headings := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(fm.Body, "\r\n", "\n"), "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			headings[m[1]] = true
		}
	}
	for _, s := range briefSections {
		if !headings[s] {
			out = append(out, "section `## "+s+"` is missing")
		}
	}
	return out
}

// EvaluateBrief lints an existing brief named by an Agent or Task prompt. Dispatch
// bookkeeping belongs to the event caller, not this read-only evaluation.
func EvaluateBrief(in hookio.Input, root string, env map[string]string) (decision hookio.Decision) {
	defer func() { decision = configuredLevel("brief-lint", in, root, env, decision) }()
	c := newContext(in, root, env)
	if c.projects == "" || (in.ToolName != "Agent" && in.ToolName != "Task") {
		return hookio.Decision{}
	}
	found := briefInPrompt(str(in.ToolInput["prompt"]), c.in.Cwd, c.projects)
	if found == nil {
		return hookio.Decision{}
	}
	text, err := readBrief(found.file)
	if err != nil {
		return hookio.Decision{}
	}
	problems := LintBrief(text)
	if len(problems) == 0 {
		return hookio.Decision{}
	}
	return hookio.Deny("The brief subagents/" + found.name + ".md is not complete: " + strings.Join(problems, "; ") + ". Have grid job lint check the Job and grid job render regenerate the brief, then start the subagent again.")
}

func readBrief(file string) (string, error) { b, err := os.ReadFile(file); return string(b), err }
