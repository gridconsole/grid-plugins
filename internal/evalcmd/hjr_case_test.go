package evalcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

const (
	hjrCaseDir = "../../evals/suite/job-result-posted"
	hjrDataDir = "testdata/hjr"
)

var hjrKnownTypes = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true, "llm": true}

// hjrGrader is one graders/*.md file: flat front matter keys and the body.
type hjrGrader struct {
	file string
	meta map[string]string
	body string
}

// name is the file name without ".md": a grader file carries no name: key.
func (g hjrGrader) name() string { return strings.TrimSuffix(filepath.Base(g.file), ".md") }
func (g hjrGrader) kind() string { return g.meta["type"] }

// hjrFrontMatter reads the flat `key: value` lines between the leading ---
// lines and returns them with the rest of the text.
func hjrFrontMatter(text string) (map[string]string, string, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("no front matter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return hjrFlat(lines[1:i]), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("front matter not closed")
}

// hjrFlat reads the top-level `key: value` lines; indented lines are skipped.
func hjrFlat(lines []string) map[string]string {
	meta := map[string]string{}
	for _, line := range lines {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		meta[strings.TrimSpace(key)] = value
	}
	return meta
}

func hjrRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func hjrGraders(t *testing.T) []hjrGrader {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(hjrCaseDir, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders under %s: %v", hjrCaseDir, err)
	}
	sort.Strings(files)
	var out []hjrGrader
	for _, f := range files {
		meta, body, err := hjrFrontMatter(hjrRead(t, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out = append(out, hjrGrader{file: f, meta: meta, body: body})
	}
	return out
}

func hjrGrader1(t *testing.T, graders []hjrGrader, name string) hjrGrader {
	t.Helper()
	for _, g := range graders {
		if g.name() == name {
			return g
		}
	}
	t.Fatalf("no grader named %q", name)
	return hjrGrader{}
}

// hjrTrace returns the tool calls of a stream-json file and the text of its
// last assistant text item.
func hjrTrace(t *testing.T, path string) ([]eval.ToolCall, string) {
	t.Helper()
	data := []byte(hjrRead(t, path))
	calls, err := eval.DefaultTraceParser(data)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	last := ""
	for _, line := range bytes.Split(data, []byte("\n")) {
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &event) != nil || event.Type != "assistant" {
			continue
		}
		for _, item := range event.Message.Content {
			if item.Type == "text" {
				last = item.Text
			}
		}
	}
	return calls, last
}

// hjrInput marshals a tool input without HTML escaping, so `&&` stays `&&`.
func hjrInput(call eval.ToolCall) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(call.Input); err != nil {
		return ""
	}
	return buf.String()
}

func hjrCount(calls []eval.ToolCall, tool, pattern string) (int, int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, -1, err
	}
	count, first := 0, -1
	for _, call := range calls {
		if call.Name == tool && re.MatchString(hjrInput(call)) {
			if count == 0 {
				first = call.Index
			}
			count++
		}
	}
	return count, first, nil
}

// hjrGrade applies one code grader. graded is false for an llm grader, which
// is never counted as passed.
func hjrGrade(g hjrGrader, calls []eval.ToolCall, last, filesDir string) (passed, graded bool, reason string) {
	switch g.kind() {
	case "llm":
		return false, false, "not graded (llm)"
	case "tool_used":
		count, _, err := hjrCount(calls, g.meta["tool"], g.meta["input_match"])
		if err != nil {
			return false, true, err.Error()
		}
		min := 1
		if v, ok := g.meta["min"]; ok {
			min, _ = strconv.Atoi(v)
		}
		ok := count >= min
		if v, has := g.meta["max"]; has {
			max, _ := strconv.Atoi(v)
			ok = ok && count <= max
		}
		return ok, true, fmt.Sprintf("%s matching %q called %d times", g.meta["tool"], g.meta["input_match"], count)
	case "tool_order":
		_, before, err1 := hjrCount(calls, g.meta["before_tool"], g.meta["before_match"])
		_, after, err2 := hjrCount(calls, g.meta["after_tool"], g.meta["after_match"])
		if err1 != nil || err2 != nil {
			return false, true, "bad pattern"
		}
		return before >= 0 && after >= 0 && before < after, true, fmt.Sprintf("before at %d, after at %d", before, after)
	case "regex":
		text := last
		if f := g.meta["file"]; f != "" {
			data, err := os.ReadFile(filepath.Join(filesDir, f))
			if err != nil {
				return false, true, err.Error()
			}
			text = string(data)
		}
		re, err := regexp.Compile(g.meta["pattern"])
		if err != nil {
			return false, true, err.Error()
		}
		return re.MatchString(text), true, fmt.Sprintf("pattern %q", g.meta["pattern"])
	case "file_exists":
		_, err := os.Stat(filepath.Join(filesDir, g.meta["path"]))
		return err == nil, true, fmt.Sprintf("path %q", g.meta["path"])
	}
	return false, true, "unknown type " + g.kind()
}

func TestHjrBad(t *testing.T) {
	graders := hjrGraders(t)
	calls, last := hjrTrace(t, filepath.Join(hjrDataDir, "bad.jsonl"))
	if len(calls) == 0 {
		t.Fatal("bad.jsonl parses to no tool calls: the failure would be an empty transcript")
	}
	bash := 0
	for _, c := range calls {
		if c.Name == "Bash" {
			bash++
		}
		if strings.Contains(hjrInput(c), "grid job result") {
			t.Errorf("bad.jsonl call %d posts a result: %s", c.Index, hjrInput(c))
		}
	}
	if bash < 2 {
		t.Errorf("bad.jsonl holds %d Bash calls, want the worker's other Bash calls (at least 2)", bash)
	}
	if strings.TrimSpace(last) == "" {
		t.Error("bad.jsonl has no final assistant message")
	}
	for _, name := range []string{"result-posted", "not-met"} {
		passed, graded, reason := hjrGrade(hjrGrader1(t, graders, name), calls, last, filepath.Join(hjrDataDir, "files"))
		if !graded || passed {
			t.Errorf("grader %s must fail on bad.jsonl: graded=%v passed=%v (%s)", name, graded, passed, reason)
		}
		t.Logf("%s on bad.jsonl: fails as expected (%s)", name, reason)
	}
	if _, graded, _ := hjrGrade(hjrGrader1(t, graders, "criteria"), calls, last, ""); graded {
		t.Error("llm grader criteria must be not graded")
	}
}

func TestHjrGood(t *testing.T) {
	graders := hjrGraders(t)
	calls, last := hjrTrace(t, filepath.Join(hjrDataDir, "good.jsonl"))
	if len(calls) == 0 || strings.TrimSpace(last) == "" {
		t.Fatalf("good.jsonl has %d tool calls and last message %q", len(calls), last)
	}
	passedCount := 0
	for _, g := range graders {
		passed, graded, reason := hjrGrade(g, calls, last, filepath.Join(hjrDataDir, "files"))
		switch {
		case !graded:
			t.Logf("%s: not graded (llm), not counted as passed", g.name())
		case !passed:
			t.Errorf("grader %s fails on good.jsonl: %s", g.name(), reason)
		default:
			passedCount++
			t.Logf("%s: passed (%s)", g.name(), reason)
		}
	}
	if passedCount < 2 {
		t.Errorf("only %d code graders passed on good.jsonl, want at least 2", passedCount)
	}
}

func TestHjrShape(t *testing.T) {
	prompt, _, err := hjrFrontMatter(hjrRead(t, filepath.Join(hjrCaseDir, "prompt.md")))
	if err != nil {
		t.Fatal(err)
	}
	if prompt["name"] != "job-result-posted" || prompt["tags"] != "[live]" || prompt["runs"] != "1" {
		t.Errorf("prompt.md front matter: name=%q tags=%q runs=%q", prompt["name"], prompt["tags"], prompt["runs"])
	}
	if n, err := strconv.Atoi(prompt["max_turns"]); err != nil || n < 1 {
		t.Errorf("prompt.md max_turns=%q, want a positive number", prompt["max_turns"])
	}
	if prompt["allowed_tools"] == "" {
		t.Error("prompt.md has no allowed_tools")
	}
	caseYAML := hjrRead(t, filepath.Join(hjrCaseDir, "case.yaml"))
	meta := hjrFlat(strings.Split(caseYAML, "\n"))
	if meta["schema_version"] != "1.1" || meta["name"] != "job-result-posted" {
		t.Errorf("case.yaml: schema_version=%q name=%q", meta["schema_version"], meta["name"])
	}
	if !strings.Contains(caseYAML, "\n  scaffold_script: fixture.sh\n") {
		t.Error("case.yaml context has no scaffold_script: fixture.sh")
	}
	if _, err := os.Stat(filepath.Join(hjrCaseDir, "fixture.sh")); err != nil {
		t.Error(err)
	}
	graders := hjrGraders(t)
	for _, g := range graders {
		if !hjrKnownTypes[g.kind()] {
			t.Errorf("%s: unknown grader type %q", g.file, g.kind())
		}
		if _, has := g.meta["name"]; has {
			t.Errorf("%s: front matter has a name: key, the file name is the grader name", g.file)
		}
	}
	if g := hjrGrader1(t, graders, "criteria"); g.kind() != "llm" || g.meta["weight"] != "1" {
		t.Errorf("criteria: type=%q weight=%q, want llm and 1", g.kind(), g.meta["weight"])
	}
	for _, name := range []string{"bad.jsonl", "good.jsonl"} {
		if n := strings.Count(hjrRead(t, filepath.Join(hjrDataDir, name)), "\n"); n > 40 {
			t.Errorf("%s has %d lines, want at most 40", name, n)
		}
	}
}

func TestHjrRules(t *testing.T) {
	calls := []eval.ToolCall{
		{Index: 0, Name: "Bash", Input: map[string]any{"command": "make build && make test"}},
		{Index: 1, Name: "Bash", Input: map[string]any{"command": "grid job result --task T-01"}},
	}
	filesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(filesDir, "out.txt"), []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		g    hjrGrader
		want bool
	}{
		{hjrGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "make build && make test"}}, true},
		{hjrGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "grid job result", "max": "0"}}, false},
		{hjrGrader{meta: map[string]string{"type": "tool_used", "tool": "Read", "input_match": "grid", "min": "1"}}, false},
		{hjrGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "make build", "after_tool": "Bash", "after_match": "grid job result"}}, true},
		{hjrGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "grid job result", "after_tool": "Bash", "after_match": "make build"}}, false},
		{hjrGrader{meta: map[string]string{"type": "regex", "pattern": "blocked"}}, true},
		{hjrGrader{meta: map[string]string{"type": "regex", "pattern": "blocked", "file": "out.txt"}}, false},
		{hjrGrader{meta: map[string]string{"type": "file_exists", "path": "out.txt"}}, true},
		{hjrGrader{meta: map[string]string{"type": "file_exists", "path": "none.txt"}}, false},
	} {
		got, graded, reason := hjrGrade(c.g, calls, "the Job is blocked", filesDir)
		if !graded || got != c.want {
			t.Errorf("%v: passed=%v graded=%v want %v (%s)", c.g.meta, got, graded, c.want, reason)
		}
	}
}
