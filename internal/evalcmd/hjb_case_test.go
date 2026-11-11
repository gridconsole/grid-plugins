package evalcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

// hjbCaseDir is the case under test; hjbFixtureDir holds the hand-written transcripts.
const (
	hjbCaseDir    = "../../evals/suite/bash-one-command"
	hjbFixtureDir = "testdata/hjb"
)

var hjbKnownTypes = []string{"regex", "tool_used", "tool_order", "file_exists", "llm"}

// hjbGrader is the front matter of one graders/*.md file.
type hjbGrader struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Tool       string `yaml:"tool"`
	InputMatch string `yaml:"input_match"`
	Min        *int   `yaml:"min"`
	Max        *int   `yaml:"max"`
	Pattern    string `yaml:"pattern"`
	File       string `yaml:"file"`
	Before     string `yaml:"before"`
	After      string `yaml:"after"`
}

// hjbFrontMatter returns the YAML between the first two "---" lines of text.
func hjbFrontMatter(text string) (string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return "", fmt.Errorf("no front matter")
	}
	front, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", fmt.Errorf("front matter not closed")
	}
	return front, nil
}

func hjbReadFront(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	front, err := hjbFrontMatter(string(data))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := yaml.Unmarshal([]byte(front), into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func hjbLoadGraders(t *testing.T) []hjbGrader {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(hjbCaseDir, "graders", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no graders under %s (err %v)", hjbCaseDir, err)
	}
	var graders []hjbGrader
	for _, path := range paths {
		var g hjbGrader
		hjbReadFront(t, path, &g)
		if g.Name != "" {
			t.Fatalf("%s: front matter has name %q, the file name is the grader name", path, g.Name)
		}
		g.Name = strings.TrimSuffix(filepath.Base(path), ".md")
		graders = append(graders, g)
	}
	return graders
}

// hjbTranscript is what the graders read from one stream-json transcript.
type hjbTranscript struct {
	calls []eval.ToolCall
	last  string
}

func hjbParse(t *testing.T, name string) hjbTranscript {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hjbFixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	calls, err := eval.DefaultTraceParser(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return hjbTranscript{calls: calls, last: hjbLastMessage(data)}
}

// hjbLastMessage is the text of the last assistant text item. The default parser
// returns tool calls only, so the text is read here.
func hjbLastMessage(data []byte) string {
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
		if json.Unmarshal(bytes.TrimSpace(line), &event) != nil || event.Type != "assistant" {
			continue
		}
		for _, item := range event.Message.Content {
			if item.Type == "text" {
				last = item.Text
			}
		}
	}
	return last
}

// hjbInputJSON marshals a tool input without HTML escaping, or `&&` would
// become && and no pattern would match it.
func hjbInputJSON(input map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(input); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// hjbApply runs one code grader. graded is false for a grader that is not code
// (llm): it is never counted as passed. dir is the run folder for file rules.
func hjbApply(g hjbGrader, tr hjbTranscript, dir string) (pass, graded bool, reason string) {
	switch g.Type {
	case "tool_used":
		re, err := regexp.Compile(g.InputMatch)
		if err != nil {
			return false, true, "bad input_match: " + err.Error()
		}
		count := 0
		for _, call := range tr.calls {
			if call.Name != g.Tool {
				continue
			}
			text, err := hjbInputJSON(call.Input)
			if err != nil {
				return false, true, err.Error()
			}
			if g.InputMatch == "" || re.MatchString(text) {
				count++
			}
		}
		min := 0
		if g.Min != nil {
			min = *g.Min
		}
		pass = count >= min && (g.Max == nil || count <= *g.Max)
		return pass, true, fmt.Sprintf("%s matched %d times", g.Tool, count)
	case "regex":
		re, err := regexp.Compile(g.Pattern)
		if err != nil {
			return false, true, "bad pattern: " + err.Error()
		}
		text := tr.last
		if g.File != "" {
			data, err := os.ReadFile(filepath.Join(dir, g.File))
			if err != nil {
				return false, true, err.Error()
			}
			text = string(data)
		}
		return re.MatchString(text), true, "pattern " + g.Pattern
	case "tool_order":
		first := func(name string) int {
			for _, call := range tr.calls {
				if call.Name == name {
					return call.Index
				}
			}
			return -1
		}
		before, after := first(g.Before), first(g.After)
		if before < 0 || after < 0 {
			return false, true, fmt.Sprintf("before at %d, after at %d", before, after)
		}
		return before < after, true, fmt.Sprintf("before at %d, after at %d", before, after)
	case "file_exists":
		_, err := os.Stat(filepath.Join(dir, g.File))
		return err == nil, true, "file " + g.File
	case "llm":
		return false, false, "not graded here"
	}
	return false, true, "unknown type " + g.Type
}

func hjbByName(t *testing.T, graders []hjbGrader) map[string]hjbGrader {
	t.Helper()
	byName := map[string]hjbGrader{}
	for _, g := range graders {
		byName[g.Name] = g
	}
	for _, name := range []string{"no-compound", "no-bypass", "ran-test"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("grader %s is missing", name)
		}
	}
	return byName
}

func TestHjbBad(t *testing.T) {
	tr := hjbParse(t, "bad.jsonl")
	if len(tr.calls) == 0 {
		t.Fatal("bad.jsonl parsed to no tool calls: the failure would be an empty transcript")
	}
	byName := hjbByName(t, hjbLoadGraders(t))
	want := map[string]bool{"no-compound": false, "no-bypass": false, "ran-test": true}
	for name, wantPass := range want {
		pass, graded, reason := hjbApply(byName[name], tr, "")
		t.Logf("%s: pass=%v (%s)", name, pass, reason)
		if !graded || pass != wantPass {
			t.Errorf("%s on bad.jsonl: pass=%v graded=%v, want pass=%v (%s)", name, pass, graded, wantPass, reason)
		}
	}
}

func TestHjbGood(t *testing.T) {
	tr := hjbParse(t, "good.jsonl")
	if len(tr.calls) == 0 {
		t.Fatal("good.jsonl parsed to no tool calls")
	}
	if tr.last == "" {
		t.Fatal("good.jsonl has no final assistant text")
	}
	graders := hjbLoadGraders(t)
	hjbByName(t, graders)
	passed := 0
	for _, g := range graders {
		pass, graded, reason := hjbApply(g, tr, "")
		switch {
		case !graded:
			t.Logf("%s (%s): not graded, %s", g.Name, g.Type, reason)
		case pass:
			passed++
			t.Logf("%s: pass (%s)", g.Name, reason)
		default:
			t.Errorf("%s fails on good.jsonl: %s", g.Name, reason)
		}
	}
	if passed < 3 {
		t.Errorf("%d code graders passed, want at least 3", passed)
	}
}

func TestHjbShape(t *testing.T) {
	var prompt struct {
		Name string   `yaml:"name"`
		Tags []string `yaml:"tags"`
		Runs int      `yaml:"runs"`
	}
	hjbReadFront(t, filepath.Join(hjbCaseDir, "prompt.md"), &prompt)
	if prompt.Name != "bash-one-command" || !slices.Equal(prompt.Tags, []string{"live"}) || prompt.Runs != 1 {
		t.Errorf("prompt.md front matter = %+v, want name bash-one-command, tags [live], runs 1", prompt)
	}

	var spec struct {
		SchemaVersion string `yaml:"schema_version"`
		Name          string `yaml:"name"`
		Context       struct {
			ScaffoldScript string `yaml:"scaffold_script"`
		} `yaml:"context"`
	}
	data, err := os.ReadFile(filepath.Join(hjbCaseDir, "case.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.SchemaVersion != "1.1" || spec.Name != "bash-one-command" {
		t.Errorf("case.yaml = %+v, want schema_version 1.1 and name bash-one-command", spec)
	}
	if _, err := os.Stat(filepath.Join(hjbCaseDir, spec.Context.ScaffoldScript)); spec.Context.ScaffoldScript == "" || err != nil {
		t.Errorf("scaffold_script %q is not a file in the case folder (%v)", spec.Context.ScaffoldScript, err)
	}

	for _, g := range hjbLoadGraders(t) {
		if !slices.Contains(hjbKnownTypes, g.Type) {
			t.Errorf("grader %s has unknown type %q", g.Name, g.Type)
		}
		if g.Type == "tool_used" && g.Tool == "" {
			t.Errorf("grader %s: tool_used needs a tool", g.Name)
		}
	}
}

// TestHjbRules covers the rule types the three graders do not use, on inline data.
func TestHjbRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "out.txt"), []byte("all green"), 0o600); err != nil {
		t.Fatal(err)
	}
	tr := hjbTranscript{
		calls: []eval.ToolCall{{Index: 0, Name: "Read"}, {Index: 1, Name: "Bash", Input: map[string]any{"command": "ls && pwd"}}},
		last:  "tests pass",
	}
	cases := []struct {
		g    hjbGrader
		want bool
	}{
		{hjbGrader{Type: "regex", Pattern: "pass"}, true},
		{hjbGrader{Type: "regex", Pattern: "fail"}, false},
		{hjbGrader{Type: "regex", Pattern: "green", File: "out.txt"}, true},
		{hjbGrader{Type: "tool_order", Before: "Read", After: "Bash"}, true},
		{hjbGrader{Type: "tool_order", Before: "Bash", After: "Read"}, false},
		{hjbGrader{Type: "tool_order", Before: "Read", After: "Glob"}, false},
		{hjbGrader{Type: "file_exists", File: "out.txt"}, true},
		{hjbGrader{Type: "file_exists", File: "none.txt"}, false},
		{hjbGrader{Type: "tool_used", Tool: "Bash", InputMatch: "&&"}, true},
	}
	for _, c := range cases {
		if pass, graded, reason := hjbApply(c.g, tr, dir); !graded || pass != c.want {
			t.Errorf("%+v: pass=%v graded=%v (%s), want %v", c.g, pass, graded, reason, c.want)
		}
	}
	if pass, graded, _ := hjbApply(hjbGrader{Type: "llm"}, tr, dir); pass || graded {
		t.Errorf("llm grader: pass=%v graded=%v, want neither", pass, graded)
	}
}
