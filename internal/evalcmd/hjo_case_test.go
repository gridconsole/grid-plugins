package evalcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

const hjoCaseDir = "../../evals/suite/read-before-edit"

var hjoKnownTypes = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true, "llm": true}

// hjoFrontMatter splits a file into its flat `key: value` front matter and the body.
// Quotes around a value are kept, so a test can tell "1.1" from 1.1.
func hjoFrontMatter(text string) (map[string]string, string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("no front matter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return hjoKeyValues(lines[1:i]), strings.TrimSpace(strings.Join(lines[i+1:], "\n")), nil
		}
	}
	return nil, "", fmt.Errorf("front matter never closes")
}

func hjoKeyValues(lines []string) map[string]string {
	out := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func hjoUnquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func hjoRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type hjoGrader struct {
	Name string
	Meta map[string]string
	Body string
}

func hjoGraders(t *testing.T) []hjoGrader {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(hjoCaseDir, "graders", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no graders found: %v", err)
	}
	sort.Strings(paths)
	var out []hjoGrader
	for _, p := range paths {
		meta, body, err := hjoFrontMatter(hjoRead(t, p))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out = append(out, hjoGrader{Name: strings.TrimSuffix(filepath.Base(p), ".md"), Meta: meta, Body: body})
	}
	return out
}

type hjoTrace struct {
	Calls []eval.ToolCall
	Last  string
}

func hjoLoad(t *testing.T, name string) hjoTrace {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "hjo", name))
	if err != nil {
		t.Fatal(err)
	}
	calls, err := eval.DefaultTraceParser(data)
	if err != nil {
		t.Fatal(err)
	}
	return hjoTrace{Calls: calls, Last: hjoLastMessage(data)}
}

// hjoLastMessage is the last text item of the last assistant event that has one.
func hjoLastMessage(data []byte) string {
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
	return last
}

// hjoInputJSON marshals without HTML escaping, so `&&` stays `&&`.
func hjoInputJSON(input map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(input); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

type hjoResult struct {
	Graded bool
	Pass   bool
	Reason string
}

func hjoFirst(calls []eval.ToolCall, tool string) int {
	for _, c := range calls {
		if c.Name == tool {
			return c.Index
		}
	}
	return -1
}

// hjoGrade applies one code grader. An llm grader is never graded.
func hjoGrade(g hjoGrader, tr hjoTrace, filesDir string) (hjoResult, error) {
	switch typ := g.Meta["type"]; typ {
	case "llm":
		return hjoResult{Reason: "llm grader, not graded"}, nil
	case "tool_order":
		before, after := hjoFirst(tr.Calls, g.Meta["before"]), hjoFirst(tr.Calls, g.Meta["after"])
		switch {
		case before < 0:
			return hjoResult{Graded: true, Reason: "no call matches before"}, nil
		case after < 0:
			return hjoResult{Graded: true, Reason: "no call matches after"}, nil
		}
		return hjoResult{Graded: true, Pass: before < after, Reason: fmt.Sprintf("before at call %d, after at call %d", before, after)}, nil
	case "tool_used":
		var re *regexp.Regexp
		if pat := hjoUnquote(g.Meta["input_match"]); pat != "" {
			var err error
			if re, err = regexp.Compile(pat); err != nil {
				return hjoResult{}, err
			}
		}
		count := 0
		for _, c := range tr.Calls {
			if c.Name == g.Meta["tool"] && (re == nil || re.MatchString(hjoInputJSON(c.Input))) {
				count++
			}
		}
		min, max := 1, -1
		if v, ok := g.Meta["min"]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return hjoResult{}, err
			}
			min = n
		}
		if v, ok := g.Meta["max"]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return hjoResult{}, err
			}
			max = n
		}
		pass := count >= min && (max < 0 || count <= max)
		return hjoResult{Graded: true, Pass: pass, Reason: fmt.Sprintf("%s called %d times, min %d, max %d", g.Meta["tool"], count, min, max)}, nil
	case "regex":
		re, err := regexp.Compile(g.Body)
		if err != nil {
			return hjoResult{}, err
		}
		if m := g.Meta["match"]; m != "contains" {
			return hjoResult{}, fmt.Errorf("regex match %q is not handled", m)
		}
		text := tr.Last
		if target := g.Meta["target"]; target != "last_message" {
			data, err := os.ReadFile(filepath.Join(filesDir, target))
			if err != nil {
				return hjoResult{Graded: true, Reason: "target file missing"}, nil
			}
			text = string(data)
		}
		return hjoResult{Graded: true, Pass: re.MatchString(text), Reason: "regex over " + g.Meta["target"]}, nil
	case "file_exists":
		_, err := os.Stat(filepath.Join(filesDir, g.Meta["path"]))
		return hjoResult{Graded: true, Pass: err == nil, Reason: "file_exists " + g.Meta["path"]}, nil
	default:
		return hjoResult{}, fmt.Errorf("unknown grader type %q", typ)
	}
}

func hjoGradeAll(t *testing.T, tr hjoTrace) map[string]hjoResult {
	t.Helper()
	out := map[string]hjoResult{}
	for _, g := range hjoGraders(t) {
		res, err := hjoGrade(g, tr, filepath.Join("testdata", "hjo", "files"))
		if err != nil {
			t.Fatalf("grader %s: %v", g.Name, err)
		}
		t.Logf("%s: graded=%v pass=%v (%s)", g.Name, res.Graded, res.Pass, res.Reason)
		out[g.Name] = res
	}
	return out
}

func TestHjoBad(t *testing.T) {
	tr := hjoLoad(t, "bad.jsonl")
	if len(tr.Calls) < 1 {
		t.Fatal("bad.jsonl has no tool calls")
	}
	res := hjoGradeAll(t, tr)
	for _, name := range []string{"read-first", "edited"} {
		if _, ok := res[name]; !ok {
			t.Fatalf("grader %s is missing", name)
		}
	}
	if !res["read-first"].Graded || res["read-first"].Pass {
		t.Errorf("read-first should fail on bad.jsonl: %+v", res["read-first"])
	}
	if !res["edited"].Graded || !res["edited"].Pass {
		t.Errorf("edited should pass on bad.jsonl, so the failure is the order: %+v", res["edited"])
	}
	if hjoFirst(tr.Calls, "Edit") >= hjoFirst(tr.Calls, "Read") {
		t.Errorf("bad.jsonl should call Edit before the first Read: %+v", tr.Calls)
	}
}

func TestHjoGood(t *testing.T) {
	tr := hjoLoad(t, "good.jsonl")
	if len(tr.Calls) < 1 {
		t.Fatal("good.jsonl has no tool calls")
	}
	passed := 0
	for name, res := range hjoGradeAll(t, tr) {
		if !res.Graded {
			t.Logf("%s: not graded", name)
			continue
		}
		if !res.Pass {
			t.Errorf("grader %s should pass on good.jsonl: %+v", name, res)
			continue
		}
		passed++
	}
	if passed == 0 {
		t.Error("no code grader passed")
	}
}

func TestHjoShape(t *testing.T) {
	prompt, body, err := hjoFrontMatter(hjoRead(t, filepath.Join(hjoCaseDir, "prompt.md")))
	if err != nil {
		t.Fatal(err)
	}
	if prompt["name"] != "read-before-edit" || prompt["tags"] != "[live]" || prompt["runs"] != "1" {
		t.Errorf("prompt.md front matter: %v", prompt)
	}
	for _, key := range []string{"max_turns", "allowed_tools"} {
		if prompt[key] == "" {
			t.Errorf("prompt.md lacks %s", key)
		}
	}
	if body == "" {
		t.Error("prompt.md has no ask")
	}
	kv := hjoKeyValues(strings.Split(hjoRead(t, filepath.Join(hjoCaseDir, "case.yaml")), "\n"))
	if kv["schema_version"] != `"1.1"` || kv["name"] != "read-before-edit" || kv["scaffold_script"] != "fixture.sh" {
		t.Errorf("case.yaml: %v", kv)
	}
	if _, err := os.Stat(filepath.Join(hjoCaseDir, kv["scaffold_script"])); err != nil {
		t.Error(err)
	}
	names := map[string]bool{}
	for _, g := range hjoGraders(t) {
		names[g.Name] = true
		if !hjoKnownTypes[g.Meta["type"]] {
			t.Errorf("grader %s has unknown type %q", g.Name, g.Meta["type"])
		}
	}
	for _, want := range []string{"read-first", "edited", "cites"} {
		if !names[want] {
			t.Errorf("grader %s is missing", want)
		}
	}
}

// TestHjoFixtureScript runs fixture.sh in a temp dir and checks the seed it builds:
// the Job names the file and the wrong constant, and the file holds that constant.
func TestHjoFixtureScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture.sh needs a POSIX shell")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	script, err := filepath.Abs(filepath.Join(hjoCaseDir, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command(sh, script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture.sh: %v\n%s", err, out)
	}
	job := hjoRead(t, filepath.Join(dir, "job.md"))
	src := hjoRead(t, filepath.Join(dir, "src", "retry.go"))
	if !strings.Contains(job, "src/retry.go") || !strings.Contains(job, "MaxTries") {
		t.Errorf("job.md does not name the file and the constant:\n%s", job)
	}
	if !strings.Contains(src, "const MaxTries = 3") {
		t.Errorf("src/retry.go lacks the wrong constant:\n%s", src)
	}
	if line := strings.Split(src, "\n")[8]; line != "const MaxTries = 3" {
		t.Errorf("good.jsonl cites src/retry.go:9, but line 9 is %q", line)
	}
}
