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
	kbEvCaseDir = "../../evals/suite/review-posts-found"
	kbEvDataDir = "testdata/kbfound"
)

var kbEvKnownTypes = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true, "llm": true}

// kbEvGrader is one graders/*.md file: flat front matter keys and the body.
type kbEvGrader struct {
	file string
	meta map[string]string
	body string
}

func (g kbEvGrader) name() string { return g.meta["name"] }
func (g kbEvGrader) kind() string { return g.meta["type"] }

// kbEvFrontMatter reads the flat `key: value` lines between the leading ---
// lines and returns them with the rest of the text.
func kbEvFrontMatter(text string) (map[string]string, string, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("no front matter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return kbEvFlat(lines[1:i]), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("front matter not closed")
}

// kbEvFlat reads the top-level `key: value` lines; indented lines are skipped.
func kbEvFlat(lines []string) map[string]string {
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

func kbEvRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func kbEvGraders(t *testing.T) []kbEvGrader {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(kbEvCaseDir, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders under %s: %v", kbEvCaseDir, err)
	}
	sort.Strings(files)
	var out []kbEvGrader
	for _, f := range files {
		meta, body, err := kbEvFrontMatter(kbEvRead(t, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out = append(out, kbEvGrader{file: f, meta: meta, body: body})
	}
	return out
}

func kbEvGrader1(t *testing.T, graders []kbEvGrader, name string) kbEvGrader {
	t.Helper()
	for _, g := range graders {
		if g.name() == name {
			return g
		}
	}
	t.Fatalf("no grader named %q", name)
	return kbEvGrader{}
}

// kbEvTrace returns the tool calls of a stream-json file and the text of its
// last assistant text item.
func kbEvTrace(t *testing.T, path string) ([]eval.ToolCall, string) {
	t.Helper()
	data := []byte(kbEvRead(t, path))
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

// kbEvInput marshals a tool input without HTML escaping, so `&&` stays `&&`.
func kbEvInput(call eval.ToolCall) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(call.Input); err != nil {
		return ""
	}
	return buf.String()
}

func kbEvCount(calls []eval.ToolCall, tool, pattern string) (int, int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, -1, err
	}
	count, first := 0, -1
	for _, call := range calls {
		if call.Name == tool && re.MatchString(kbEvInput(call)) {
			if count == 0 {
				first = call.Index
			}
			count++
		}
	}
	return count, first, nil
}

// kbEvGrade applies one code grader. graded is false for an llm grader, which
// is never counted as passed.
func kbEvGrade(g kbEvGrader, calls []eval.ToolCall, last, filesDir string) (passed, graded bool, reason string) {
	switch g.kind() {
	case "llm":
		return false, false, "not graded (llm)"
	case "tool_used":
		count, _, err := kbEvCount(calls, g.meta["tool"], g.meta["input_match"])
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
		_, before, err1 := kbEvCount(calls, g.meta["before_tool"], g.meta["before_match"])
		_, after, err2 := kbEvCount(calls, g.meta["after_tool"], g.meta["after_match"])
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

func TestKbEvBad(t *testing.T) {
	graders := kbEvGraders(t)
	calls, last := kbEvTrace(t, filepath.Join(kbEvDataDir, "bad.jsonl"))
	if len(calls) == 0 {
		t.Fatal("bad.jsonl parses to no tool calls: the failure would be an empty transcript")
	}
	results := 0
	for _, c := range calls {
		in := kbEvInput(c)
		if strings.Contains(in, "--type found") {
			t.Errorf("bad.jsonl call %d posts a found record: %s", c.Index, in)
		}
		if strings.Contains(in, "grid job result") {
			results++
		}
	}
	if results != 1 {
		t.Errorf("bad.jsonl posts %d results, want the reviewer's one `grid job result`", results)
	}
	if strings.TrimSpace(last) == "" {
		t.Error("bad.jsonl has no final assistant message")
	}
	for _, name := range []string{"found-posted", "found-before-result"} {
		passed, graded, reason := kbEvGrade(kbEvGrader1(t, graders, name), calls, last, filepath.Join(kbEvDataDir, "files"))
		if !graded || passed {
			t.Errorf("grader %s must fail on bad.jsonl: graded=%v passed=%v (%s)", name, graded, passed, reason)
		}
		t.Logf("%s on bad.jsonl: fails as expected (%s)", name, reason)
	}
	if _, graded, _ := kbEvGrade(kbEvGrader1(t, graders, "found-security"), calls, last, ""); graded {
		t.Error("llm grader found-security must be not graded")
	}
}

func TestKbEvGood(t *testing.T) {
	graders := kbEvGraders(t)
	calls, last := kbEvTrace(t, filepath.Join(kbEvDataDir, "good.jsonl"))
	if len(calls) == 0 || strings.TrimSpace(last) == "" {
		t.Fatalf("good.jsonl has %d tool calls and last message %q", len(calls), last)
	}
	foundPosts := 0
	for _, c := range calls {
		in := kbEvInput(c)
		if c.Name == "Bash" && strings.Contains(in, "grid job post") && strings.Contains(in, "--type found") &&
			strings.Contains(in, "--task ") && strings.Contains(in, "--job ") && strings.Contains(in, "--body ") {
			foundPosts++
		}
	}
	if foundPosts != 1 {
		t.Errorf("good.jsonl holds %d `grid job post --task --job --type found --body` calls, want 1", foundPosts)
	}
	passedCount := 0
	for _, g := range graders {
		passed, graded, reason := kbEvGrade(g, calls, last, filepath.Join(kbEvDataDir, "files"))
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
	if passedCount != 2 {
		t.Errorf("%d code graders passed on good.jsonl, want 2 (tool_used and tool_order)", passedCount)
	}
}

func TestKbEvShape(t *testing.T) {
	prompt, _, err := kbEvFrontMatter(kbEvRead(t, filepath.Join(kbEvCaseDir, "prompt.md")))
	if err != nil {
		t.Fatal(err)
	}
	if prompt["name"] != "review-posts-found" || prompt["tags"] != "[live]" || prompt["runs"] != "1" {
		t.Errorf("prompt.md front matter: name=%q tags=%q runs=%q", prompt["name"], prompt["tags"], prompt["runs"])
	}
	if n, err := strconv.Atoi(prompt["max_turns"]); err != nil || n < 1 {
		t.Errorf("prompt.md max_turns=%q, want a positive number", prompt["max_turns"])
	}
	if prompt["allowed_tools"] == "" {
		t.Error("prompt.md has no allowed_tools")
	}
	caseYAML := kbEvRead(t, filepath.Join(kbEvCaseDir, "case.yaml"))
	meta := kbEvFlat(strings.Split(caseYAML, "\n"))
	if meta["schema_version"] != "1.1" || meta["name"] != "review-posts-found" {
		t.Errorf("case.yaml: schema_version=%q name=%q", meta["schema_version"], meta["name"])
	}
	if !strings.Contains(caseYAML, "\n  scaffold_script: fixture.sh\n") {
		t.Error("case.yaml context has no scaffold_script: fixture.sh")
	}
	if _, err := os.Stat(filepath.Join(kbEvCaseDir, "fixture.sh")); err != nil {
		t.Error(err)
	}
	graders := kbEvGraders(t)
	if len(graders) != 3 {
		t.Errorf("%d grader files, want 3", len(graders))
	}
	for _, g := range graders {
		if !kbEvKnownTypes[g.kind()] {
			t.Errorf("%s: unknown grader type %q", g.file, g.kind())
		}
		if g.name() != strings.TrimSuffix(filepath.Base(g.file), ".md") {
			t.Errorf("%s: name %q does not match the file", g.file, g.name())
		}
	}
	used := kbEvGrader1(t, graders, "found-posted")
	if used.kind() != "tool_used" || used.meta["tool"] != "Bash" || used.meta["input_match"] != "--type found" || used.meta["min"] != "1" {
		t.Errorf("found-posted: %v, want tool_used on Bash with input_match --type found and min 1", used.meta)
	}
	order := kbEvGrader1(t, graders, "found-before-result")
	if order.kind() != "tool_order" || !strings.Contains(order.meta["before_match"], "--type found") || !strings.Contains(order.meta["after_match"], "grid job result") {
		t.Errorf("found-before-result: %v, want the found post before grid job result", order.meta)
	}
	if g := kbEvGrader1(t, graders, "found-security"); g.kind() != "llm" || g.meta["weight"] != "1" {
		t.Errorf("found-security: type=%q weight=%q, want llm and 1", g.kind(), g.meta["weight"])
	}
	for _, name := range []string{"bad.jsonl", "good.jsonl"} {
		if n := strings.Count(kbEvRead(t, filepath.Join(kbEvDataDir, name)), "\n"); n > 40 {
			t.Errorf("%s has %d lines, want at most 40", name, n)
		}
	}
	// The seeded hole sits at the file and line the llm grader names.
	fixture := kbEvRead(t, filepath.Join(kbEvCaseDir, "fixture.sh"))
	if !strings.Contains(fixture, "src/auth.go") || !strings.Contains(fixture, "log.Printf") {
		t.Error("fixture.sh does not seed a token written to a log in src/auth.go")
	}
}

func TestKbEvRules(t *testing.T) {
	calls := []eval.ToolCall{
		{Index: 0, Name: "Bash", Input: map[string]any{"command": "grid job post --task T-01 --job J-2 --type found --body '{}'"}},
		{Index: 1, Name: "Bash", Input: map[string]any{"command": "grid job result --task T-01 --job J-2"}},
	}
	for _, c := range []struct {
		g    kbEvGrader
		want bool
	}{
		{kbEvGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "--type found"}}, true},
		{kbEvGrader{meta: map[string]string{"type": "tool_used", "tool": "Read", "input_match": "--type found"}}, false},
		{kbEvGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "--type found", "max": "0"}}, false},
		{kbEvGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "--type found", "after_tool": "Bash", "after_match": "grid job result"}}, true},
		{kbEvGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "grid job result", "after_tool": "Bash", "after_match": "--type found"}}, false},
		{kbEvGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "--type found", "after_tool": "Bash", "after_match": "no such"}}, false},
	} {
		got, graded, reason := kbEvGrade(c.g, calls, "", "")
		if !graded || got != c.want {
			t.Errorf("%v: passed=%v graded=%v want %v (%s)", c.g.meta, got, graded, c.want, reason)
		}
	}
}
