package evalcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
	"go.yaml.in/yaml/v4"
)

const (
	hjuCaseDir = "../../evals/suite/research-unverified"
	hjuDataDir = "testdata/hju"
)

var hjuKnownTypes = []string{"regex", "tool_used", "tool_order", "file_exists", "llm"}

// hjuGrader is one graders/*.md file: its front matter and its body.
type hjuGrader struct {
	Name string
	Type string
	Head map[string]any
	Body string
}

// hjuSplit cuts a file with YAML front matter into the header and the body.
func hjuSplit(text string) (string, string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return "", "", fmt.Errorf("no front matter")
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return "", "", fmt.Errorf("front matter is not closed")
	}
	return head, body, nil
}

func hjuFront(text string) (map[string]any, string, error) {
	head, body, err := hjuSplit(text)
	if err != nil {
		return nil, "", err
	}
	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(head), &m); err != nil {
		return nil, "", err
	}
	return m, body, nil
}

func hjuParseGrader(text string) (hjuGrader, error) {
	head, body, err := hjuFront(text)
	if err != nil {
		return hjuGrader{}, err
	}
	g := hjuGrader{Head: head, Body: strings.TrimSpace(body)}
	g.Name, _ = head["name"].(string)
	g.Type, _ = head["type"].(string)
	if g.Name == "" || g.Type == "" {
		return g, fmt.Errorf("grader needs a name and a type")
	}
	return g, nil
}

func hjuLoadGraders(t *testing.T) []hjuGrader {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(hjuCaseDir, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders found: %v", err)
	}
	var out []hjuGrader
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := hjuParseGrader(string(data))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if want := strings.TrimSuffix(filepath.Base(f), ".md"); g.Name != want {
			t.Fatalf("%s: name %q does not match the file name", f, g.Name)
		}
		out = append(out, g)
	}
	return out
}

// hjuRun is one run: its tool calls, its last assistant text and its file tree.
type hjuRun struct {
	calls []eval.ToolCall
	last  string
	tree  string
}

func hjuLoadRun(t *testing.T, jsonl, tree string) hjuRun {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hjuDataDir, jsonl))
	if err != nil {
		t.Fatal(err)
	}
	calls, err := eval.DefaultTraceParser(data)
	if err != nil {
		t.Fatalf("%s: %v", jsonl, err)
	}
	return hjuRun{calls: calls, last: hjuLastText(data), tree: filepath.Join(hjuDataDir, tree)}
}

// hjuLastText is the last text item of the last assistant message that has one.
func hjuLastText(data []byte) string {
	last := ""
	for _, line := range bytes.Split(data, []byte("\n")) {
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &ev) != nil || ev.Type != "assistant" {
			continue
		}
		for _, item := range ev.Message.Content {
			if item.Type == "text" {
				last = item.Text
			}
		}
	}
	return last
}

// hjuInputJSON marshals a tool input without HTML escaping, so && stays &&.
func hjuInputJSON(call eval.ToolCall) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(call.Input); err != nil {
		return ""
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// hjuMatch is a tool_order side or a tool_used selector: a tool name and a
// regexp over the tool input JSON, both optional.
type hjuMatch struct {
	tool    string
	pattern *regexp.Regexp
}

func hjuMatcher(tool, pattern string) (hjuMatch, error) {
	m := hjuMatch{tool: tool}
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return m, err
		}
		m.pattern = re
	}
	return m, nil
}

func (m hjuMatch) hits(call eval.ToolCall) bool {
	if m.tool != "" && call.Name != m.tool {
		return false
	}
	return m.pattern == nil || m.pattern.MatchString(hjuInputJSON(call))
}

func (m hjuMatch) first(calls []eval.ToolCall) int {
	for i, c := range calls {
		if m.hits(c) {
			return i
		}
	}
	return -1
}

func hjuString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func hjuInt(m map[string]any, key string) (int, bool) {
	n, ok := m[key].(int)
	return n, ok
}

func hjuSub(m map[string]any, key string) map[string]any {
	sub, _ := m[key].(map[string]any)
	if sub == nil {
		sub = map[string]any{}
	}
	return sub
}

// hjuFile reads a file of the run's tree; a path that leaves the tree is refused.
func hjuFile(run hjuRun, rel string) (string, bool, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(filepath.Clean(rel)), "..") {
		return "", false, fmt.Errorf("path %q is not inside the run folder", rel)
	}
	data, err := os.ReadFile(filepath.Join(run.tree, rel))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return string(data), err == nil, err
}

// hjuGrade applies one grader. graded is false for an llm grader: it is never
// run here and never counts as passed.
func hjuGrade(g hjuGrader, run hjuRun) (pass, graded bool, reason string, err error) {
	switch g.Type {
	case "llm":
		return false, false, "llm grader, not graded here", nil
	case "file_exists":
		_, found, err := hjuFile(run, hjuString(g.Head, "path"))
		if err != nil {
			return false, true, "", err
		}
		want := true
		if v, ok := g.Head["exists"].(bool); ok {
			want = v
		}
		return found == want, true, fmt.Sprintf("exists=%v, want %v", found, want), nil
	case "regex":
		re, err := regexp.Compile(g.Body)
		if err != nil {
			return false, true, "", err
		}
		target := hjuSub(g.Head, "target")
		var text string
		switch src := hjuString(target, "source"); src {
		case "last_message":
			text = run.last
		case "file":
			body, found, err := hjuFile(run, hjuString(target, "path"))
			if err != nil {
				return false, true, "", err
			}
			if !found {
				return false, true, "file is missing", nil
			}
			text = body
		default:
			return false, true, "", fmt.Errorf("unknown regex source %q", src)
		}
		hit := re.MatchString(text)
		switch mode := hjuString(g.Head, "match"); mode {
		case "contains":
			return hit, true, fmt.Sprintf("pattern found=%v", hit), nil
		case "not_contains":
			return !hit, true, fmt.Sprintf("pattern found=%v", hit), nil
		default:
			return false, true, "", fmt.Errorf("unknown match %q", mode)
		}
	case "tool_used":
		m, err := hjuMatcher(hjuString(g.Head, "tool"), hjuString(g.Head, "input_match"))
		if err != nil {
			return false, true, "", err
		}
		count := 0
		for _, c := range run.calls {
			if m.hits(c) {
				count++
			}
		}
		min, hasMin := hjuInt(g.Head, "min")
		if !hasMin {
			min = 1
		}
		max, hasMax := hjuInt(g.Head, "max")
		ok := count >= min && (!hasMax || count <= max)
		return ok, true, fmt.Sprintf("%d matching calls, min %d", count, min), nil
	case "tool_order":
		var idx [2]int
		for i, side := range []string{"before", "after"} {
			s := hjuSub(g.Head, side)
			m, err := hjuMatcher(hjuString(s, "tool"), hjuString(s, "input_match"))
			if err != nil {
				return false, true, "", err
			}
			idx[i] = m.first(run.calls)
		}
		ok := idx[0] >= 0 && idx[1] >= 0 && idx[0] < idx[1]
		return ok, true, fmt.Sprintf("before at %d, after at %d", idx[0], idx[1]), nil
	}
	return false, true, "", fmt.Errorf("unknown grader type %q", g.Type)
}

// hjuGradeAll returns pass by grader name for the code graders and logs the
// llm ones as not graded.
func hjuGradeAll(t *testing.T, graders []hjuGrader, run hjuRun) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, g := range graders {
		pass, graded, reason, err := hjuGrade(g, run)
		if err != nil {
			t.Fatalf("grader %s: %v", g.Name, err)
		}
		if !graded {
			t.Logf("grader %s: not graded (%s)", g.Name, reason)
			continue
		}
		t.Logf("grader %s: pass=%v (%s)", g.Name, pass, reason)
		out[g.Name] = pass
	}
	return out
}

func hjuNeed(t *testing.T, results map[string]bool, name string, want bool) {
	t.Helper()
	got, ok := results[name]
	if !ok {
		t.Fatalf("grader %s was not graded", name)
	}
	if got != want {
		t.Errorf("grader %s: pass=%v, want %v", name, got, want)
	}
}

func TestHjuBad(t *testing.T) {
	run := hjuLoadRun(t, "bad.jsonl", "files-bad")
	if len(run.calls) < 1 {
		t.Fatal("bad.jsonl has no tool calls: the failure would be an empty transcript")
	}
	if run.last == "" {
		t.Fatal("bad.jsonl has no final assistant text")
	}
	results := hjuGradeAll(t, hjuLoadGraders(t), run)
	hjuNeed(t, results, "research-written", true)
	hjuNeed(t, results, "marks-unverified", false)
	hjuNeed(t, results, "no-false-fact", false)
}

func TestHjuGood(t *testing.T) {
	run := hjuLoadRun(t, "good.jsonl", "files-good")
	if len(run.calls) < 1 {
		t.Fatal("good.jsonl has no tool calls")
	}
	graders := hjuLoadGraders(t)
	results := hjuGradeAll(t, graders, run)
	code := 0
	for _, g := range graders {
		if g.Type != "llm" {
			code++
		}
	}
	if len(results) != code || code < 4 {
		t.Fatalf("graded %d of %d code graders", len(results), code)
	}
	for name, pass := range results {
		if !pass {
			t.Errorf("grader %s fails on good.jsonl", name)
		}
	}
}

func TestHjuShape(t *testing.T) {
	prompt, err := os.ReadFile(filepath.Join(hjuCaseDir, "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	head, body, err := hjuFront(string(prompt))
	if err != nil {
		t.Fatalf("prompt.md: %v", err)
	}
	if head["name"] != "research-unverified" {
		t.Errorf("prompt name = %v", head["name"])
	}
	if tags, _ := head["tags"].([]any); len(tags) != 1 || tags[0] != "live" {
		t.Errorf("prompt tags = %v, want [live]", head["tags"])
	}
	if head["runs"] != 1 {
		t.Errorf("prompt runs = %v, want 1", head["runs"])
	}
	if n, _ := head["max_turns"].(int); n < 1 {
		t.Errorf("prompt max_turns = %v", head["max_turns"])
	}
	if tools, _ := head["allowed_tools"].([]any); len(tools) == 0 {
		t.Error("prompt has no allowed_tools")
	}
	if strings.TrimSpace(body) == "" {
		t.Error("prompt body is empty")
	}

	data, err := os.ReadFile(filepath.Join(hjuCaseDir, "case.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cs struct {
		SchemaVersion string `yaml:"schema_version"`
		Name          string `yaml:"name"`
		Context       struct {
			ScaffoldScript string `yaml:"scaffold_script"`
		} `yaml:"context"`
	}
	if err := yaml.Unmarshal(data, &cs); err != nil {
		t.Fatal(err)
	}
	if cs.SchemaVersion != "1.1" || cs.Name != "research-unverified" || cs.Context.ScaffoldScript != "fixture.sh" {
		t.Errorf("case.yaml = %+v", cs)
	}
	if _, err := os.Stat(filepath.Join(hjuCaseDir, cs.Context.ScaffoldScript)); err != nil {
		t.Errorf("scaffold script: %v", err)
	}

	names := []string{}
	for _, g := range hjuLoadGraders(t) {
		names = append(names, g.Name)
		known := false
		for _, k := range hjuKnownTypes {
			known = known || g.Type == k
		}
		if !known {
			t.Errorf("grader %s has unknown type %q", g.Name, g.Type)
		}
	}
	sort.Strings(names)
	want := []string{"cites-lines", "marks-unverified", "no-false-fact", "research-written"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("graders = %v, want %v", names, want)
	}
}

// TestHjuFixture runs the scaffold script and checks the seed: one true claim
// findable at a path:line and one false claim with no code behind it.
func TestHjuFixture(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh to run fixture.sh")
	}
	script, err := filepath.Abs(filepath.Join(hjuCaseDir, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command(sh, script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture.sh: %v\n%s", err, out)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	notes := read("inputs/notes.md")
	if !strings.Contains(notes, "gives up after 3 attempts") || !strings.Contains(notes, "flushed every 30 seconds") {
		t.Errorf("notes.md misses a claim:\n%s", notes)
	}
	if line := strings.Split(read("repo/retry.go"), "\n")[3]; !strings.Contains(line, "maxAttempts = 3") {
		t.Errorf("retry.go:4 = %q, want the maxAttempts line the good run cites", line)
	}
	for _, f := range []string{"repo/retry.go", "repo/upload.go"} {
		if src := read(f); strings.Contains(src, "FlushQueue") || strings.Contains(src, "30") {
			t.Errorf("%s holds code for the false claim", f)
		}
	}
}

// TestHjuRules covers the rules the four case graders do not use, with graders written inline.
func TestHjuRules(t *testing.T) {
	good := hjuLoadRun(t, "good.jsonl", "files-good")
	bad := hjuLoadRun(t, "bad.jsonl", "files-bad")
	grade := func(run hjuRun, text string) (bool, bool) {
		t.Helper()
		g, err := hjuParseGrader(text)
		if err != nil {
			t.Fatal(err)
		}
		pass, graded, _, err := hjuGrade(g, run)
		if err != nil {
			t.Fatal(err)
		}
		return pass, graded
	}
	read := "---\nname: a\ntype: tool_used\ntool: Read\ninput_match: 'notes\\.md'\nmin: 1\n---\n"
	if pass, _ := grade(good, read); !pass {
		t.Error("tool_used Read of notes.md fails on good")
	}
	twice := "---\nname: a\ntype: tool_used\ntool: Read\nmin: 3\n---\n"
	if pass, _ := grade(good, twice); pass {
		t.Error("tool_used min 3 passes with two Read calls")
	}
	none := "---\nname: a\ntype: tool_used\ntool: Write\nmax: 0\n---\n"
	if pass, _ := grade(good, none); pass {
		t.Error("tool_used max 0 passes with one Write call")
	}
	order := "---\nname: a\ntype: tool_order\nbefore: {tool: Read, input_match: 'notes\\.md'}\nafter: {tool: Write}\n---\n"
	if pass, _ := grade(bad, order); !pass {
		t.Error("tool_order Read before Write fails")
	}
	swapped := "---\nname: a\ntype: tool_order\nbefore: {tool: Write}\nafter: {tool: Read}\n---\n"
	if pass, _ := grade(bad, swapped); pass {
		t.Error("tool_order Write before Read passes")
	}
	last := "---\nname: a\ntype: regex\ntarget: {source: last_message}\nmatch: contains\n---\nUNVERIFIED\n"
	if pass, _ := grade(good, last); !pass {
		t.Error("last_message of good does not hold UNVERIFIED")
	}
	if pass, _ := grade(bad, last); pass {
		t.Error("last_message of bad holds UNVERIFIED")
	}
	if pass, graded := grade(good, "---\nname: a\ntype: llm\n---\nIs the claim marked?\n"); pass || graded {
		t.Errorf("llm grader: pass=%v graded=%v, want neither", pass, graded)
	}
	// && must reach the pattern as typed, not as &&.
	call := eval.ToolCall{Name: "Bash", Input: map[string]any{"command": "make build && make test"}}
	if got := hjuInputJSON(call); !strings.Contains(got, "build && make test") {
		t.Errorf("input JSON = %s", got)
	}
	if _, found, err := hjuFile(good, "../bad.jsonl"); err == nil || found {
		t.Error("a path outside the run folder was read")
	}
}
