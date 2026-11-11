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

// The plan-step-anchor case lives in evals/suite/plan-step-anchor. These tests
// replay two hand-written transcripts through its graders: the bad one must fail
// no-steps-anchor, the good one must pass every code grader. The live run is
// done by hand with `claude plugin eval`; nothing here calls a model.

const (
	hjaCaseDir    = "../../evals/suite/plan-step-anchor"
	hjaFixtureDir = "testdata/hja"
)

var hjaKnownTypes = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true, "llm": true}

// hjaGrader is one graders/*.md file: its front matter fields and its body.
type hjaGrader struct {
	File   string
	Fields map[string]string
	Body   string
}

func (g hjaGrader) name() string { return g.Fields["name"] }
func (g hjaGrader) kind() string { return g.Fields["type"] }

// count reads an integer field; ok is false when the field is absent.
func (g hjaGrader) count(key string) (n int, ok bool, err error) {
	v, present := g.Fields[key]
	if !present {
		return 0, false, nil
	}
	n, err = strconv.Atoi(v)
	return n, true, err
}

// hjaFrontMatter splits "---\nkey: value\n---\nbody" into flat fields and body.
// It reads only top-level "key: value" lines, which is all the case files use.
func hjaFrontMatter(text string) (map[string]string, string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("no front matter")
	}
	fields := map[string]string{}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return fields, strings.TrimSpace(strings.Join(lines[i+1:], "\n")), nil
		}
		if err := hjaField(fields, lines[i]); err != nil {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("front matter is not closed")
}

func hjaField(fields map[string]string, line string) error {
	if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
		return nil
	}
	key, value, ok := strings.Cut(line, ":")
	if !ok || key != strings.TrimSpace(key) || key == "" {
		return fmt.Errorf("not a key: value line: %q", line)
	}
	fields[key] = hjaScalar(strings.TrimSpace(value))
	return nil
}

// hjaScalar unquotes a single- or double-quoted scalar and leaves the rest as is.
func hjaScalar(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
	}
	return v
}

func hjaLoadGraders(t *testing.T) []hjaGrader {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(hjaCaseDir, "graders", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no graders under %s: %v", hjaCaseDir, err)
	}
	sort.Strings(paths)
	var out []hjaGrader
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		fields, body, err := hjaFrontMatter(string(data))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out = append(out, hjaGrader{File: p, Fields: fields, Body: body})
	}
	return out
}

// hjaTrace is what a code grader reads from one transcript.
type hjaTrace struct {
	Calls []eval.ToolCall
	Last  string // the last text item of the last assistant message that has one
}

func hjaLoadTrace(t *testing.T, name string) hjaTrace {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hjaFixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	calls, err := eval.DefaultTraceParser(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return hjaTrace{Calls: calls, Last: hjaLastMessage(data)}
}

func hjaLastMessage(data []byte) string {
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

// hjaInputJSON marshals a tool input without HTML escaping, so "&&" stays "&&".
func hjaInputJSON(input map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(input); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// hjaMatchCalls returns the indexes of the calls of tool whose input JSON matches pattern.
// An empty tool or pattern matches anything.
func hjaMatchCalls(calls []eval.ToolCall, tool, pattern string) ([]int, error) {
	var re *regexp.Regexp
	if pattern != "" {
		var err error
		if re, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
	}
	var hits []int
	for _, c := range calls {
		if tool != "" && c.Name != tool {
			continue
		}
		if re != nil {
			text, err := hjaInputJSON(c.Input)
			if err != nil {
				return nil, err
			}
			if !re.MatchString(text) {
				continue
			}
		}
		hits = append(hits, c.Index)
	}
	return hits, nil
}

// hjaResult is one grader's verdict. Graded is false for an llm grader, which
// this test cannot run and never counts as passed.
type hjaResult struct {
	Graded bool
	Passed bool
	Reason string
}

func hjaApply(g hjaGrader, tr hjaTrace, root string) (hjaResult, error) {
	switch g.kind() {
	case "tool_used":
		return hjaToolUsed(g, tr)
	case "tool_order":
		return hjaToolOrder(g, tr)
	case "regex":
		return hjaRegex(g, tr, root)
	case "file_exists":
		_, err := os.Stat(filepath.Join(root, g.Fields["file"]))
		return hjaResult{Graded: true, Passed: err == nil, Reason: "file " + g.Fields["file"]}, nil
	case "llm":
		return hjaResult{Reason: "llm grader, not graded"}, nil
	}
	return hjaResult{}, fmt.Errorf("%s: unknown grader type %q", g.name(), g.kind())
}

func hjaToolUsed(g hjaGrader, tr hjaTrace) (hjaResult, error) {
	hits, err := hjaMatchCalls(tr.Calls, g.Fields["tool"], g.Fields["input_match"])
	if err != nil {
		return hjaResult{}, err
	}
	lo, _, err := g.count("min")
	if err != nil {
		return hjaResult{}, err
	}
	hi, hasMax, err := g.count("max")
	if err != nil {
		return hjaResult{}, err
	}
	n := len(hits)
	pass := n >= lo && (!hasMax || n <= hi)
	return hjaResult{Graded: true, Passed: pass, Reason: fmt.Sprintf("%d matching calls at %v, min %d, max set %v (%d)", n, hits, lo, hasMax, hi)}, nil
}

func hjaToolOrder(g hjaGrader, tr hjaTrace) (hjaResult, error) {
	first, err := hjaMatchCalls(tr.Calls, g.Fields["before_tool"], g.Fields["before_match"])
	if err != nil {
		return hjaResult{}, err
	}
	then, err := hjaMatchCalls(tr.Calls, g.Fields["after_tool"], g.Fields["after_match"])
	if err != nil {
		return hjaResult{}, err
	}
	if len(first) == 0 || len(then) == 0 {
		return hjaResult{Graded: true, Reason: "a side of the order has no matching call"}, nil
	}
	return hjaResult{Graded: true, Passed: first[0] < then[0], Reason: fmt.Sprintf("before at %d, after at %d", first[0], then[0])}, nil
}

func hjaRegex(g hjaGrader, tr hjaTrace, root string) (hjaResult, error) {
	re, err := regexp.Compile(g.Fields["pattern"])
	if err != nil {
		return hjaResult{}, err
	}
	text := tr.Last
	if file := g.Fields["file"]; file != "" {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			return hjaResult{Graded: true, Reason: err.Error()}, nil
		}
		text = string(data)
	}
	return hjaResult{Graded: true, Passed: re.MatchString(text), Reason: "pattern " + g.Fields["pattern"]}, nil
}

func hjaByName(t *testing.T, graders []hjaGrader, name string) hjaGrader {
	t.Helper()
	for _, g := range graders {
		if g.name() == name {
			return g
		}
	}
	t.Fatalf("no grader named %s", name)
	return hjaGrader{}
}

// TestHjaBad: the transcript replays the planner pointing Jobs at plan.md#steps.
func TestHjaBad(t *testing.T) {
	graders := hjaLoadGraders(t)
	tr := hjaLoadTrace(t, "bad.jsonl")
	if len(tr.Calls) == 0 {
		t.Fatal("bad.jsonl parsed to no tool calls")
	}
	if tr.Last == "" {
		t.Fatal("bad.jsonl has no final assistant text")
	}

	// The behaviour is in the file: grid job calls that point at plan.md#steps.
	hits, err := hjaMatchCalls(tr.Calls, "Bash", `grid job .*plan\.md#steps`)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("bad.jsonl holds no grid job Bash call that points at plan.md#steps")
	}
	t.Logf("bad.jsonl: %d tool calls, %d grid job calls point at plan.md#steps (calls %v)", len(tr.Calls), len(hits), hits)

	g := hjaByName(t, graders, "no-steps-anchor")
	res, err := hjaApply(g, tr, hjaFixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Graded || res.Passed {
		t.Fatalf("no-steps-anchor should fail on bad.jsonl: %+v", res)
	}
	t.Logf("no-steps-anchor fails on bad.jsonl as it should: %s", res.Reason)
}

// TestHjaGood: every code grader passes on the good transcript; llm graders are not counted.
func TestHjaGood(t *testing.T) {
	graders := hjaLoadGraders(t)
	tr := hjaLoadTrace(t, "good.jsonl")
	if len(tr.Calls) == 0 {
		t.Fatal("good.jsonl parsed to no tool calls")
	}
	passed, notGraded := 0, 0
	for _, g := range graders {
		res, err := hjaApply(g, tr, hjaFixtureDir)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case !res.Graded:
			notGraded++
			t.Logf("%s: not graded (%s grader)", g.name(), g.kind())
		case res.Passed:
			passed++
			t.Logf("%s: passed (%s)", g.name(), res.Reason)
		default:
			t.Errorf("%s: failed on good.jsonl (%s)", g.name(), res.Reason)
		}
	}
	if passed < 2 {
		t.Errorf("only %d code graders passed, want at least no-steps-anchor and step-anchor", passed)
	}
	if passed+notGraded != len(graders) && !t.Failed() {
		t.Errorf("graders: %d passed + %d not graded != %d", passed, notGraded, len(graders))
	}
	// The three Jobs each carry their own step anchor.
	for _, n := range []string{"1", "2", "3"} {
		hits, err := hjaMatchCalls(tr.Calls, "Bash", `grid job post .*plan\.md#step-`+n+`\b`)
		if err != nil || len(hits) != 1 {
			t.Errorf("step-%s: %d grid job post calls (%v)", n, len(hits), err)
		}
	}
}

// TestHjaShape: the case files have the shape claude plugin eval reads.
func TestHjaShape(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(hjaCaseDir, "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	fields, body, err := hjaFrontMatter(string(data))
	if err != nil {
		t.Fatalf("prompt.md: %v", err)
	}
	if fields["name"] != "plan-step-anchor" {
		t.Errorf("prompt.md name = %q", fields["name"])
	}
	if fields["tags"] != "[live]" {
		t.Errorf("prompt.md tags = %q, want [live]", fields["tags"])
	}
	if fields["runs"] != "1" {
		t.Errorf("prompt.md runs = %q, want 1", fields["runs"])
	}
	for _, k := range []string{"max_turns", "allowed_tools"} {
		if fields[k] == "" {
			t.Errorf("prompt.md has no %s", k)
		}
	}
	if body == "" {
		t.Error("prompt.md has no ask")
	}

	caseYAML, err := os.ReadFile(filepath.Join(hjaCaseDir, "case.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`schema_version: "1.1"`, "name: plan-step-anchor", "scaffold_script: fixture.sh"} {
		if !strings.Contains(string(caseYAML), want) {
			t.Errorf("case.yaml lacks %s", want)
		}
	}
	if _, err := os.Stat(filepath.Join(hjaCaseDir, "fixture.sh")); err != nil {
		t.Errorf("fixture.sh: %v", err)
	}

	graders := hjaLoadGraders(t)
	seen := map[string]bool{}
	for _, g := range graders {
		if !hjaKnownTypes[g.kind()] {
			t.Errorf("%s: unknown type %q", g.File, g.kind())
		}
		if want := strings.TrimSuffix(filepath.Base(g.File), ".md"); g.name() != want {
			t.Errorf("%s: name %q does not match the file name", g.File, g.name())
		}
		if g.Body == "" {
			t.Errorf("%s: empty body", g.File)
		}
		seen[g.name()] = true
	}
	for _, want := range []string{"no-steps-anchor", "step-anchor", "criteria"} {
		if !seen[want] {
			t.Errorf("missing grader %s", want)
		}
	}
	if g := hjaByName(t, graders, "criteria"); g.kind() != "llm" || g.Fields["weight"] != "1" {
		t.Errorf("criteria: type %q weight %q", g.kind(), g.Fields["weight"])
	}
}

// TestHjaRules checks the rule appliers on small inputs, so the graders they
// serve are not trusted on the two transcripts alone.
func TestHjaRules(t *testing.T) {
	calls := []eval.ToolCall{
		{Index: 0, Name: "Read", Input: map[string]any{"file_path": "plan.md"}},
		{Index: 1, Name: "Bash", Input: map[string]any{"command": "grid job lint a.json && grid job post --body x"}},
	}
	tr := hjaTrace{Calls: calls, Last: "Posted J-1."}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "out.txt"), []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := func(kv ...string) hjaGrader {
		f := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			f[kv[i]] = kv[i+1]
		}
		return hjaGrader{Fields: f}
	}
	for _, tc := range []struct {
		name string
		g    hjaGrader
		want bool
	}{
		{"and and is not escaped", g("type", "tool_used", "tool", "Bash", "input_match", `a\.json && grid job post`, "min", "1"), true},
		{"max 0 fails on a hit", g("type", "tool_used", "tool", "Bash", "input_match", `grid job`, "max", "0"), false},
		{"min 2 fails on one hit", g("type", "tool_used", "tool", "Bash", "min", "2"), false},
		{"order holds", g("type", "tool_order", "before_tool", "Read", "after_tool", "Bash"), true},
		{"order reversed", g("type", "tool_order", "before_tool", "Bash", "after_tool", "Read"), false},
		{"order missing side", g("type", "tool_order", "before_tool", "Read", "after_tool", "Edit"), false},
		{"regex on last message", g("type", "regex", "pattern", `Posted J-\d`), true},
		{"regex on file", g("type", "regex", "pattern", `^done$`, "file", "out.txt"), true},
		{"regex on missing file", g("type", "regex", "pattern", `x`, "file", "none.txt"), false},
		{"file exists", g("type", "file_exists", "file", "out.txt"), true},
		{"file missing", g("type", "file_exists", "file", "none.txt"), false},
	} {
		res, err := hjaApply(tc.g, tr, root)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
		} else if !res.Graded || res.Passed != tc.want {
			t.Errorf("%s: %+v, want passed=%v", tc.name, res, tc.want)
		}
	}
	if res, err := hjaApply(g("type", "llm"), tr, root); err != nil || res.Graded || res.Passed {
		t.Errorf("llm grader: %+v, %v; want not graded and not passed", res, err)
	}
	if _, err := hjaApply(g("type", "bogus"), tr, root); err == nil {
		t.Error("unknown grader type did not error")
	}
}
