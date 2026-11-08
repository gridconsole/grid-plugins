package workflow

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v4"
)

// Template is a parsed workflows.md section in the Settings page's wire format.
type Template struct {
	Name      string         `json:"name"`
	Default   bool           `json:"default"`
	Applies   []Rule         `json:"applies"`
	Only      *string        `json:"only"`
	Approvers []string       `json:"approvers"`
	Repeat    *string        `json:"repeat"`
	Lead      *string        `json:"lead"`
	Batch     *string        `json:"batch"`
	Rounds    uint8          `json:"rounds"`
	Research  bool           `json:"research"`
	Steps     []TemplateStep `json:"steps"`
	Overrides *string        `json:"overrides"`
	File      string         `json:"file"`
	UsedIn    []string       `json:"usedIn"`
	OpenTasks []string       `json:"openTasks"`

	// QuestionWithdrawRoles replaces the owner/member default when present.
	QuestionWithdrawRoles []string `json:"question_withdraw_roles,omitempty"`
}

// Rule selects tasks by goal, title, project or label.
type Rule struct {
	By string `json:"by"`
	V  string `json:"v"`
}

// TemplateStep is one row of a template's step table.
type TemplateStep struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Approver  *string  `json:"approver"`
	Approvers []string `json:"approvers"`
	Agent     *string  `json:"agent"`
	Runs      *string  `json:"runs"`
	Sizes     string   `json:"sizes"`
	Back      *string  `json:"back"`
}

// Problem identifies a file or section that could not be loaded.
type Problem struct {
	File    string  `json:"file"`
	Section *string `json:"section"`
	Line    *int    `json:"line"`
	Message string  `json:"message"`
}

func (p Problem) String() string {
	s := p.File
	if p.Section != nil {
		s += fmt.Sprintf(", section \"%s\"", *p.Section)
	}
	if p.Line != nil {
		s += fmt.Sprintf(", line %d", *p.Line)
	}
	return s + ": " + p.Message
}

// Parsed holds the successful sections and diagnostics from one file.
type Parsed struct {
	Templates []Template `json:"templates"`
	Errors    []Problem  `json:"errors"`
}

const defaultRule = "nothing else matches"
const copyOf = "as in the workspace "
const backTo = "on reject, back to"

type sourceLine struct {
	number int
	text   string
}
type section struct {
	name string
	line int
	body []sourceLine
}

func labelOf(project string) string {
	if project != "" {
		return project + "/workflows.md"
	}
	return "workflows.md"
}

// Parse ports grid-core's parse_workflows. An empty project means workspace.
// Bad sections are omitted; diagnostics and successful sections keep source order.
func Parse(text, project string) Parsed {
	out := Parsed{Templates: []Template{}, Errors: []Problem{}}
	file := labelOf(project)
	whole := func(line *int, message string) {
		out.Errors = append(out.Errors, Problem{File: file, Line: line, Message: message})
	}
	if strings.ContainsRune(text, 0) {
		whole(nil, "the file holds a NUL byte")
		return out
	}
	// Rust str::lines removes LF and CRLF, but not an isolated trailing CR.
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		if strings.HasSuffix(line, "\n") {
			lines[i] = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		}
	}
	first := 0
	for first < len(lines) && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	var intro bool
	if first < len(lines) {
		heading := strings.TrimRightFunc(lines[first], unicode.IsSpace)
		intro = project != "" && heading == "# "+project+" workflows"
		if heading != "# Workflows" && !intro {
			n := first + 1
			whole(&n, "the file must start with \"# Workflows\"")
			return out
		}
	} else {
		whole(nil, "the file must start with \"# Workflows\"")
		return out
	}
	sections := []section{}
	for i := first + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "##" || strings.HasPrefix(line, "## ") {
			sections = append(sections, section{name: strings.TrimSpace(strings.TrimPrefix(line, "##")), line: i + 1})
		} else if len(sections) > 0 {
			last := &sections[len(sections)-1]
			last.body = append(last.body, sourceLine{i + 1, lines[i]})
		} else if line != "" && !intro {
			n := i + 1
			whole(&n, "a line outside any section")
		}
	}
	names := map[string]bool{}
	hasDefault := false
	for _, s := range sections {
		var t Template
		var problem *Problem
		name := strings.ToLower(s.name)
		if names[name] {
			problem = &Problem{Line: &s.line, Message: "another workflow in this file has this name"}
		} else {
			names[name] = true
			t, problem = parseSection(s, project)
		}
		if problem == nil && t.Default && hasDefault {
			problem = &Problem{Line: &s.line, Message: "the file already has a default workflow"}
		}
		if problem != nil {
			problem.File = file
			problem.Section = &s.name
			out.Errors = append(out.Errors, *problem)
		} else {
			hasDefault = hasDefault || t.Default
			t.File = file
			out.Templates = append(out.Templates, t)
		}
	}
	if project == "" && !hasDefault {
		whole(nil, "the file has no default workflow (\"applies when: nothing else matches\")")
	}
	return out
}

func parseSection(s section, project string) (Template, *Problem) {
	t := Template{Name: s.name, Applies: []Rule{}, Approvers: []string{}, Steps: []TemplateStep{}, UsedIn: []string{}, OpenTasks: []string{}}
	fail := func(n int, m string) (Template, *Problem) { return Template{}, &Problem{Line: &n, Message: m} }
	if m := checkValue(s.name); m != "" {
		return fail(s.line, "the name "+m)
	}
	tableAt := len(s.body)
	for i, l := range s.body {
		if strings.HasPrefix(strings.TrimLeftFunc(l.text, unicode.IsSpace), "|") {
			tableAt = i
			break
		}
	}
	seen := map[string]bool{}
	var applies *string
	appliesLine, onlyLine := s.line, s.line
	for _, l := range s.body[:tableAt] {
		if strings.TrimSpace(l.text) == "" {
			continue
		}
		key, value, ok := strings.Cut(l.text, ":")
		if !ok {
			return fail(l.number, fmt.Sprintf("\"%s\" is not a \"key: value\" line", strings.TrimSpace(l.text)))
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if seen[key] {
			return fail(l.number, fmt.Sprintf("\"%s\" is given twice", key))
		}
		seen[key] = true
		if key == "applies when" {
			if strings.ContainsAny(value, "|\r") {
				return fail(l.number, key+" holds a | or a line break")
			}
		} else if m := checkValue(value); m != "" {
			return fail(l.number, key+" "+m)
		}
		switch key {
		case "applies when":
			applies = &value
			appliesLine = l.number
		case "only for project":
			t.Only = &value
			onlyLine = l.number
		case "question_withdraw_roles":
			for _, role := range strings.Split(value, ",") {
				role = strings.TrimSpace(role)
				if !slices.Contains([]string{"owner", "member", "guest", "lead"}, role) || slices.Contains(t.QuestionWithdrawRoles, role) {
					return fail(l.number, "question_withdraw_roles must list distinct person roles: owner, member, guest or lead")
				}
				t.QuestionWithdrawRoles = append(t.QuestionWithdrawRoles, role)
			}
		case "approvers":
			for _, who := range strings.Split(value, ",") {
				who = strings.TrimSpace(who)
				if !isHandle(who) {
					return fail(l.number, fmt.Sprintf("approver \"%s\" is not a lowercase name", who))
				}
				t.Approvers = append(t.Approvers, who)
			}
		case "repeat":
			t.Repeat = &value
		case "lead time":
			t.Lead = &value
		case "batch":
			t.Batch = &value
		case "review rounds":
			// Rust unsigned parsing accepts a leading plus and leading zeroes.
			n, err := strconv.ParseUint(strings.TrimPrefix(value, "+"), 10, 8)
			if err != nil || (n != 1 && n != 2 && n != 3 && n != 5) {
				return fail(l.number, "review rounds must be 1, 2, 3 or 5")
			}
			t.Rounds = uint8(n)
		case "research":
			if value != "first" {
				return fail(l.number, "research must be \"first\"")
			}
			t.Research = true
		default:
			return fail(l.number, fmt.Sprintf("\"%s\" is not a workflow key", key))
		}
	}
	if applies == nil {
		return fail(s.line, "the workflow has no \"applies when\" line")
	}
	if m := readRule(&t, *applies); m != "" {
		return fail(appliesLine, m)
	}
	if t.Rounds == 0 {
		return fail(s.line, "the workflow has no \"review rounds\" line")
	}
	switch {
	case project == "" && t.Only != nil:
		return fail(onlyLine, "only a project's file says \"only for project\"")
	case project == "" && t.Overrides != nil:
		return fail(appliesLine, "only a project's file copies a workspace workflow")
	case project != "" && t.Default:
		return fail(appliesLine, "a project's file holds no default workflow")
	case project != "":
		if t.Only != nil && *t.Only != project {
			return fail(onlyLine, fmt.Sprintf("only for project says %s, but the file is in %s", *t.Only, project))
		}
		if t.Overrides != nil && *t.Overrides != t.Name {
			return fail(appliesLine, "a copy keeps the name of the workspace workflow it copies")
		}
	}
	if p := readTable(&t, s.line, s.body[tableAt:]); p != nil {
		return Template{}, p
	}
	return t, nil
}

func readRule(t *Template, rule string) string {
	if rule == defaultRule {
		t.Default = true
		return ""
	}
	if name, ok := strings.CutPrefix(rule, copyOf); ok {
		name = strings.TrimSpace(name)
		if m := checkValue(name); m != "" {
			return "the workflow copied " + m
		}
		t.Overrides = &name
		return ""
	}
	for _, part := range strings.Split(rule, ", or ") {
		part = strings.TrimSpace(part)
		found := false
		for _, r := range []Rule{{"goal", "the task "}, {"title", "the title has "}, {"project", "project "}, {"label", "label "}} {
			if v, ok := strings.CutPrefix(part, r.V); ok && strings.TrimSpace(v) != "" {
				t.Applies = append(t.Applies, Rule{r.By, strings.TrimSpace(v)})
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("\"%s\" is not a rule: it starts with \"the task\", \"the title has\", \"project\" or \"label\"", part)
		}
	}
	return ""
}

func cells(line string) []string {
	line = strings.TrimSpace(line)
	if len(line) < 2 || line[0] != '|' || line[len(line)-1] != '|' {
		return nil
	}
	out := strings.Split(line[1:len(line)-1], "|")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

type columns struct {
	runs, sizes bool
	count       int
}

func columnsOf(head []string) (columns, bool) {
	c := columns{count: len(head)}
	if len(head) < 5 || strings.Join(head[:4], "|") != "#|step|kind|who" || head[len(head)-1] != backTo {
		return c, false
	}
	switch strings.Join(head[4:len(head)-1], "|") {
	case "":
		return c, len(head) == 5
	case "runs":
		c.runs = true
	case "sizes":
		c.sizes = true
	case "runs|sizes":
		c.runs = true
		c.sizes = true
	default:
		return c, false
	}
	return c, true
}

func readTable(t *Template, header int, table []sourceLine) *Problem {
	fail := func(n int, m string) *Problem { return &Problem{Line: &n, Message: m} }
	end := len(table)
	for i, l := range table {
		if strings.TrimSpace(l.text) == "" {
			end = i
			break
		}
	}
	for _, l := range table[end:] {
		if strings.TrimSpace(l.text) != "" {
			return fail(l.number, "a line after the step table")
		}
	}
	rows := table[:end]
	if len(rows) == 0 {
		return fail(header, "the workflow has no step table")
	}
	cols, ok := columnsOf(cells(rows[0].text))
	if !ok {
		return fail(rows[0].number, "the table header must be | # | step | kind | who | [runs |] [sizes |] on reject, back to |")
	}
	if len(rows) == 1 {
		return fail(rows[0].number, "the table has no separator line")
	}
	rule := cells(rows[1].text)
	valid := len(rule) == cols.count
	for _, cell := range rule {
		cell = strings.TrimSuffix(strings.TrimPrefix(cell, ":"), ":")
		if cell == "" || strings.Trim(cell, "-") != "" {
			valid = false
		}
	}
	if !valid {
		return fail(rows[1].number, "the line under the table header must be |---|---|...|")
	}
	for i, row := range rows[2:] {
		step, m := readStep(t, cols, i+1, row.text)
		if m != "" {
			return fail(row.number, m)
		}
		t.Steps = append(t.Steps, step)
	}
	if len(t.Steps) == 0 {
		return fail(rows[0].number, "the table has no steps")
	}
	return nil
}

func readStep(t *Template, cols columns, number int, row string) (TemplateStep, string) {
	fail := func(m string) (TemplateStep, string) { return TemplateStep{}, m }
	c := cells(row)
	if len(c) != cols.count {
		return fail(fmt.Sprintf("a step row has %d cells, and no cell holds a |", cols.count))
	}
	num, name, kind, who := c[0], c[1], c[2], c[3]
	runs, sizes := "", "all"
	i := 4
	if cols.runs {
		runs = c[i]
		i++
	}
	if cols.sizes {
		sizes = c[i]
		i++
	}
	back := c[i]
	if num != strconv.Itoa(number) {
		return fail(fmt.Sprintf("step %d is numbered \"%s\"", number, num))
	}
	if m := checkValue(name); m != "" {
		return fail("the step name " + m)
	}
	for _, s := range t.Steps {
		if strings.ToLower(s.Name) == strings.ToLower(name) {
			return fail(fmt.Sprintf("two steps are named \"%s\"", name))
		}
	}
	switch kind {
	case "check", "ask", "approve", "make-together", "run", "schedule":
	default:
		return fail(fmt.Sprintf("\"%s\" is not a step kind", kind))
	}
	approver, agent, m := whoOf(kind, who)
	if m != "" {
		return fail(m)
	}
	s := TemplateStep{Name: name, Kind: kind, Approver: approver, Agent: agent}
	if approver != nil {
		s.Approvers = []string{*approver}
	}
	if runs != "" {
		if kind != "run" {
			return fail("only a run step says what it runs")
		}
		if m := checkValue(runs); m != "" {
			return fail("runs " + m)
		}
		s.Runs = &runs
	}
	s.Sizes = sizesOf(sizes)
	if s.Sizes == "" {
		return fail("sizes must be \"all\" or some of S N L")
	}
	if back != "" {
		for _, prev := range t.Steps {
			if prev.Name == back {
				s.Back = &back
				break
			}
		}
		if s.Back == nil {
			return fail(fmt.Sprintf("\"%s\" is not an earlier step", back))
		}
	}
	return s, ""
}

func whoOf(kind, who string) (*string, *string, string) {
	name := who
	switch kind {
	case "ask":
		var ok bool
		name, ok = strings.CutSuffix(who, " asks, you answer")
		if !ok {
			return nil, nil, "an ask step's who is \"<agent> asks, you answer\""
		}
		name = strings.TrimSpace(name)
	case "make-together":
		var ok bool
		name, ok = strings.CutPrefix(who, "you and ")
		if !ok {
			return nil, nil, "a make-together step's who is \"you and <agent>\""
		}
		name = strings.TrimSpace(name)
	case "schedule":
		if who == "the scheduler" {
			return nil, nil, ""
		}
		return nil, nil, "a schedule step's who is \"the scheduler\""
	}
	if kind != "approve" && name == "grid-worker" {
		name = "grid-builder"
	}
	if !isHandle(name) {
		return nil, nil, fmt.Sprintf("\"%s\" does not name a lowercase approver or agent", who)
	}
	if kind == "approve" {
		return &name, nil, ""
	}
	return nil, &name, ""
}

func sizesOf(cell string) string {
	if cell == "all" {
		return "SNL"
	}
	picked := ""
	for _, token := range strings.Fields(cell) {
		if len(token) != 1 || !strings.Contains("SNL", token) || strings.Contains(picked, token) {
			return ""
		}
		picked += token
	}
	out := ""
	for _, s := range "SNL" {
		if strings.ContainsRune(picked, s) {
			out += string(s)
		}
	}
	return out
}
func isHandle(name string) bool {
	return name != "" && name == strings.ToLower(name) && !strings.Contains(name, ",") && checkValue(name) == ""
}
func checkValue(value string) string {
	switch {
	case value == "":
		return "is empty"
	case strings.Contains(value, "|"):
		return "holds a |"
	case strings.ContainsAny(value, "\r\n"):
		return "holds a line break"
	case strings.Contains(value, ", or "):
		return "holds \", or \""
	}
	return ""
}

// QuestionWithdrawRoles reads policy for a task in the Grid home. An explicit
// task workflow wins; otherwise the first matching rule or workspace default
// applies. A project section replaces the same named workspace section.
// Nil roles preserve the owner/member default, including absent workflow files.
func QuestionWithdrawRoles(taskDir string) ([]string, error) {
	taskDir = filepath.Clean(taskDir)
	tasks := filepath.Dir(taskDir)
	if filepath.Base(tasks) != "tasks" {
		return nil, nil
	}
	projectDir := filepath.Dir(tasks)
	parent := filepath.Dir(projectDir)
	home, project := parent, ""
	if filepath.Base(parent) == "projects" {
		home, project = filepath.Dir(parent), filepath.Base(projectDir)
	} else if filepath.Base(projectDir) != "personal" {
		return nil, nil
	}
	templates := []Template{}
	paths := []string{"workflows.md"}
	if project != "" {
		paths = append(paths, filepath.Join("projects", project, "workflows.md"))
	}
	for i, path := range paths {
		text, rev, err := readFile(home, path)
		if err != nil {
			return nil, fmt.Errorf("question workflow: %w", err)
		}
		if rev == nil {
			continue
		}
		scope := ""
		if i > 0 {
			scope = project
		}
		parsed := Parse(text, scope)
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("question workflow: %s", parsed.Errors[0].String())
		}
		for _, candidate := range parsed.Templates {
			replaced := false
			for j, existing := range templates {
				if strings.EqualFold(existing.Name, candidate.Name) {
					// A project copy inherits the workspace matching rule, but
					// omission of the policy key restores the standard default.
					if candidate.Overrides != nil {
						candidate.Default, candidate.Applies = existing.Default, existing.Applies
					}
					templates[j], replaced = candidate, true
					break
				}
			}
			if !replaced {
				templates = append(templates, candidate)
			}
		}
	}
	if len(templates) == 0 {
		return nil, nil
	}
	relative, err := filepath.Rel(home, filepath.Join(taskDir, "task.md"))
	if err != nil {
		return nil, err
	}
	text, _, err := readFile(home, relative)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return nil, fmt.Errorf("question workflow: task.md has no YAML front matter")
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return nil, fmt.Errorf("question workflow: task.md front matter is not closed")
	}
	var meta struct {
		Workflow, Title, Goal string
		Labels                []string
	}
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return nil, fmt.Errorf("question workflow: %w", err)
	}
	var fallback []string
	for _, t := range templates {
		if meta.Workflow != "" {
			if strings.EqualFold(t.Name, meta.Workflow) {
				return t.QuestionWithdrawRoles, nil
			}
			continue
		}
		if t.Default {
			fallback = t.QuestionWithdrawRoles
		}
		for _, rule := range t.Applies {
			match := false
			switch rule.By {
			case "title":
				match = strings.Contains(strings.ToLower(meta.Title), strings.ToLower(rule.V))
			case "goal":
				match = strings.EqualFold(meta.Goal, rule.V)
			case "project":
				match = strings.EqualFold(project, rule.V)
			case "label":
				match = slices.Contains(meta.Labels, rule.V)
			}
			if match {
				return t.QuestionWithdrawRoles, nil
			}
		}
	}
	if meta.Workflow != "" {
		return nil, fmt.Errorf("question workflow: unknown workflow %q", meta.Workflow)
	}
	return fallback, nil
}
