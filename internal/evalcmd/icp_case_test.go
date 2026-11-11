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
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

const (
	icpCaseDir = "../../evals/suite/replan-dead-job"
	icpDataDir = "testdata/icp"
)

var icpKnownTypes = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true, "llm": true}

// icpGrader is one graders/*.md file: flat front matter keys and the body.
type icpGrader struct {
	file string
	meta map[string]string
	body string
}

func (g icpGrader) name() string { return g.meta["name"] }
func (g icpGrader) kind() string { return g.meta["type"] }

// icpFrontMatter reads the flat `key: value` lines between the leading ---
// lines and returns them with the rest of the text.
func icpFrontMatter(text string) (map[string]string, string, error) {
	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("no front matter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return icpFlat(lines[1:i]), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("front matter not closed")
}

// icpFlat reads the top-level `key: value` lines; indented lines are skipped.
func icpFlat(lines []string) map[string]string {
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

func icpRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func icpGraders(t *testing.T) []icpGrader {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(icpCaseDir, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders under %s: %v", icpCaseDir, err)
	}
	sort.Strings(files)
	var out []icpGrader
	for _, f := range files {
		meta, body, err := icpFrontMatter(icpRead(t, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out = append(out, icpGrader{file: f, meta: meta, body: body})
	}
	return out
}

func icpGraderNamed(t *testing.T, graders []icpGrader, name string) icpGrader {
	t.Helper()
	for _, g := range graders {
		if g.name() == name {
			return g
		}
	}
	t.Fatalf("no grader named %q", name)
	return icpGrader{}
}

// icpTrace returns the tool calls of a stream-json file and the text of its
// last assistant text item.
func icpTrace(t *testing.T, path string) ([]eval.ToolCall, string) {
	t.Helper()
	data := []byte(icpRead(t, path))
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

// icpInput marshals a tool input without HTML escaping, so `&&` stays `&&`.
func icpInput(call eval.ToolCall) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(call.Input); err != nil {
		return ""
	}
	return buf.String()
}

// icpCount returns how many calls of the tool match the pattern and the index
// of the first one, -1 when none does.
func icpCount(calls []eval.ToolCall, tool, pattern string) (int, int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, -1, err
	}
	count, first := 0, -1
	for _, call := range calls {
		if call.Name == tool && re.MatchString(icpInput(call)) {
			if count == 0 {
				first = call.Index
			}
			count++
		}
	}
	return count, first, nil
}

// icpGrade applies one code grader. graded is false for an llm grader, which
// is never counted as passed.
func icpGrade(g icpGrader, calls []eval.ToolCall, last, filesDir string) (passed, graded bool, reason string) {
	switch g.kind() {
	case "llm":
		return false, false, "not graded (llm)"
	case "tool_used":
		count, _, err := icpCount(calls, g.meta["tool"], g.meta["input_match"])
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
		_, before, err1 := icpCount(calls, g.meta["before_tool"], g.meta["before_match"])
		_, after, err2 := icpCount(calls, g.meta["after_tool"], g.meta["after_match"])
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

func TestIcpBad(t *testing.T) {
	graders := icpGraders(t)
	calls, last := icpTrace(t, filepath.Join(icpDataDir, "bad.jsonl"))
	if len(calls) == 0 {
		t.Fatal("bad.jsonl parses to no tool calls: the failure would be an empty transcript")
	}
	if strings.TrimSpace(last) == "" {
		t.Error("bad.jsonl has no final assistant message")
	}
	for _, c := range []struct {
		name string
		want bool
	}{{"showed", false}, {"cancelled", false}, {"replaced", true}} {
		passed, graded, reason := icpGrade(icpGraderNamed(t, graders, c.name), calls, last, icpDataDir)
		if !graded || passed != c.want {
			t.Errorf("grader %s on bad.jsonl: graded=%v passed=%v, want passed=%v (%s)", c.name, graded, passed, c.want, reason)
		}
		t.Logf("%s on bad.jsonl: passed=%v (%s)", c.name, passed, reason)
	}
	if _, graded, _ := icpGrade(icpGraderNamed(t, graders, "criteria"), calls, last, ""); graded {
		t.Error("llm grader criteria must be not graded")
	}
}

func TestIcpGood(t *testing.T) {
	graders := icpGraders(t)
	calls, last := icpTrace(t, filepath.Join(icpDataDir, "good.jsonl"))
	if len(calls) == 0 || strings.TrimSpace(last) == "" {
		t.Fatalf("good.jsonl has %d tool calls and last message %q", len(calls), last)
	}
	passedCount := 0
	for _, g := range graders {
		passed, graded, reason := icpGrade(g, calls, last, icpDataDir)
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
	if passedCount != 3 {
		t.Errorf("%d code graders passed on good.jsonl, want 3", passedCount)
	}
	// The cancel must name a waiter, never the dead Job itself.
	if n, _, _ := icpCount(calls, "Bash", `grid job cancel .*--job J-2\b`); n != 0 {
		t.Error("good.jsonl cancels the dead Job J-2")
	}
}

func TestIcpShape(t *testing.T) {
	prompt, _, err := icpFrontMatter(icpRead(t, filepath.Join(icpCaseDir, "prompt.md")))
	if err != nil {
		t.Fatal(err)
	}
	if prompt["name"] != "replan-dead-job" || prompt["tags"] != "[live]" || prompt["runs"] != "1" {
		t.Errorf("prompt.md front matter: name=%q tags=%q runs=%q", prompt["name"], prompt["tags"], prompt["runs"])
	}
	if n, err := strconv.Atoi(prompt["max_turns"]); err != nil || n < 1 {
		t.Errorf("prompt.md max_turns=%q, want a positive number", prompt["max_turns"])
	}
	if prompt["allowed_tools"] == "" {
		t.Error("prompt.md has no allowed_tools")
	}
	caseYAML := icpRead(t, filepath.Join(icpCaseDir, "case.yaml"))
	meta := icpFlat(strings.Split(caseYAML, "\n"))
	if meta["schema_version"] != "1.1" || meta["name"] != "replan-dead-job" {
		t.Errorf("case.yaml: schema_version=%q name=%q", meta["schema_version"], meta["name"])
	}
	if !strings.Contains(caseYAML, "\n  scaffold_script: fixture.sh\n") {
		t.Error("case.yaml context has no scaffold_script: fixture.sh")
	}
	graders := icpGraders(t)
	if len(graders) != 4 {
		t.Errorf("%d graders, want 4", len(graders))
	}
	for _, g := range graders {
		if !icpKnownTypes[g.kind()] {
			t.Errorf("%s: unknown grader type %q", g.file, g.kind())
		}
		if g.name() != strings.TrimSuffix(filepath.Base(g.file), ".md") {
			t.Errorf("%s: name %q does not match the file", g.file, g.name())
		}
	}
	for name, match := range map[string]string{"showed": "grid job show", "cancelled": "grid job cancel", "replaced": "grid job post"} {
		g := icpGraderNamed(t, graders, name)
		if g.kind() != "tool_used" || g.meta["tool"] != "Bash" || g.meta["input_match"] != match || g.meta["min"] != "1" {
			t.Errorf("%s: type=%q tool=%q input_match=%q min=%q", name, g.kind(), g.meta["tool"], g.meta["input_match"], g.meta["min"])
		}
	}
	if g := icpGraderNamed(t, graders, "criteria"); g.kind() != "llm" || g.meta["weight"] != "1" {
		t.Errorf("criteria: type=%q weight=%q, want llm and 1", g.kind(), g.meta["weight"])
	}
	for _, name := range []string{"bad.jsonl", "good.jsonl"} {
		if n := strings.Count(icpRead(t, filepath.Join(icpDataDir, name)), "\n"); n > 40 {
			t.Errorf("%s has %d lines, want at most 40", name, n)
		}
	}
}

// TestIcpFixture runs fixture.sh in a temp dir and checks the seed: one
// cancelled Job with its cause, two waiters (one open step, one dropped) and
// the Re-plan Job.
func TestIcpFixture(t *testing.T) {
	script, err := filepath.Abs(filepath.Join(icpCaseDir, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("sh", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture.sh: %v\n%s", err, out)
	}
	feed := filepath.Join(dir, "task-replan", "feed")
	want := map[string][]string{
		"J-2.md": {"state: cancelled", "Cause:"},
		"J-3.md": {"after: [J-2]", "(open)"},
		"J-4.md": {"after: [J-2]", "(dropped from the plan)"},
		"J-5.md": {"# J-5: Re-plan after J-2 was cancelled", "role: orchestrator", "skills: [plan-jobs]"},
	}
	for file, parts := range want {
		text := icpRead(t, filepath.Join(feed, file))
		for _, p := range parts {
			if !strings.Contains(text, p) {
				t.Errorf("%s lacks %q", file, p)
			}
		}
	}
}

func TestIcpRules(t *testing.T) {
	calls := []eval.ToolCall{
		{Index: 0, Name: "Bash", Input: map[string]any{"command": "make build && make test"}},
		{Index: 1, Name: "Bash", Input: map[string]any{"command": "grid job cancel --task T-01 --job J-4"}},
	}
	filesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(filesDir, "out.txt"), []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		g    icpGrader
		want bool
	}{
		{icpGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "make build && make test"}}, true},
		{icpGrader{meta: map[string]string{"type": "tool_used", "tool": "Bash", "input_match": "grid job cancel", "max": "0"}}, false},
		{icpGrader{meta: map[string]string{"type": "tool_used", "tool": "Read", "input_match": "grid", "min": "1"}}, false},
		{icpGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "make build", "after_tool": "Bash", "after_match": "grid job cancel"}}, true},
		{icpGrader{meta: map[string]string{"type": "tool_order", "before_tool": "Bash", "before_match": "grid job cancel", "after_tool": "Bash", "after_match": "make build"}}, false},
		{icpGrader{meta: map[string]string{"type": "regex", "pattern": "blocked"}}, true},
		{icpGrader{meta: map[string]string{"type": "regex", "pattern": "blocked", "file": "out.txt"}}, false},
		{icpGrader{meta: map[string]string{"type": "file_exists", "path": "out.txt"}}, true},
		{icpGrader{meta: map[string]string{"type": "file_exists", "path": "none.txt"}}, false},
	} {
		got, graded, reason := icpGrade(c.g, calls, "the Job is blocked", filesDir)
		if !graded || got != c.want {
			t.Errorf("%v: passed=%v graded=%v want %v (%s)", c.g.meta, got, graded, c.want, reason)
		}
	}
}
