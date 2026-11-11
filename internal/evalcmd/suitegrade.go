package evalcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/eval"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
)

// CaseGrade is the offline grade of one `claude plugin eval` case.
//
// Passed means at least one grader ran and every grader that ran passed. A grader
// that needs a model (llm) or a second run (baseline) is never run here: its name
// is in NotGraded and it counts for nothing. A caller that must know the whole case
// holds checks that NotGraded is empty.
type CaseGrade struct {
	Passed    bool
	Graders   []GraderResult
	NotGraded []string
}

// GraderResult is the verdict of one grader file; Name is the file name without .md.
type GraderResult struct {
	Name   string
	Type   string
	Passed bool
	Reason string
}

// graderSpec is the front matter of one grader file.
type graderSpec struct {
	Type       string `yaml:"type"`
	Pattern    string `yaml:"pattern"`
	Match      string `yaml:"match"`
	Flags      string `yaml:"flags"`
	Target     any    `yaml:"target"`
	Tool       string `yaml:"tool"`
	InputMatch string `yaml:"input_match"`
	Min        *int   `yaml:"min"`
	Max        *int   `yaml:"max"`
	Before     any    `yaml:"before"`
	After      any    `yaml:"after"`
	Path       string `yaml:"path"`
	Exists     *bool  `yaml:"exists"`
}

// graderKeys lists the keys each graded type accepts; any other key is a typo that
// would silently weaken the check, so it is an error.
var graderKeys = map[string][]string{
	"regex":       {"type", "pattern", "match", "flags", "target"},
	"tool_used":   {"type", "tool", "input_match", "flags", "min", "max"},
	"tool_order":  {"type", "before", "after", "flags"},
	"file_exists": {"type", "path", "exists"},
}

// GradeCase grades the transcript (claude stream-json) of one case against the
// graders in caseDir/graders. files is the workspace the run left behind, read by
// file_exists and by the files and file targets; it may be "" for a case that
// needs none. A malformed case, an unknown grader type, a bad regex or an empty
// transcript is an error, never a pass.
func GradeCase(caseDir string, transcript []byte, files string) (CaseGrade, error) {
	var grade CaseGrade
	if err := checkCaseDir(caseDir); err != nil {
		return grade, err
	}
	if len(bytes.TrimSpace(transcript)) == 0 {
		return grade, errors.New("empty transcript")
	}
	calls, err := eval.DefaultTraceParser(transcript)
	if err != nil {
		return grade, fmt.Errorf("transcript: %w", err)
	}
	last, haveLast := lastMessage(transcript)
	env := gradeEnv{transcript: string(transcript), last: last, haveLast: haveLast, calls: calls, files: files}

	dir := filepath.Join(caseDir, "graders")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return grade, fmt.Errorf("graders: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return CaseGrade{}, fmt.Errorf("grader %s: %w", name, err)
		}
		res, graded, err := env.grade(name, data)
		if err != nil {
			return CaseGrade{}, fmt.Errorf("grader %s: %w", name, err)
		}
		if !graded {
			grade.NotGraded = append(grade.NotGraded, name)
			continue
		}
		grade.Graders = append(grade.Graders, res)
	}
	if len(grade.Graders)+len(grade.NotGraded) == 0 {
		return CaseGrade{}, fmt.Errorf("graders: no *.md grader in %s", dir)
	}
	grade.Passed = len(grade.Graders) > 0
	for _, r := range grade.Graders {
		grade.Passed = grade.Passed && r.Passed
	}
	return grade, nil
}

// checkCaseDir refuses a folder that is not a case, and a prompt.md whose front
// matter does not parse.
func checkCaseDir(caseDir string) error {
	prompt, err := os.ReadFile(filepath.Join(caseDir, "prompt.md"))
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(caseDir, "case.yaml")); statErr == nil {
			return nil
		}
		return fmt.Errorf("case %s: no prompt.md or case.yaml: %w", caseDir, err)
	}
	if !bytes.HasPrefix(prompt, []byte("---")) {
		return nil // a prompt without front matter is plain text
	}
	front, _, err := mdfile.Split(prompt)
	if err != nil {
		return fmt.Errorf("prompt.md: %w", err)
	}
	var meta map[string]any
	if err := mdfile.Load(front, &meta, false); err != nil {
		return fmt.Errorf("prompt.md: %w", err)
	}
	return nil
}

type gradeEnv struct {
	transcript string
	last       string
	haveLast   bool
	calls      []eval.ToolCall
	files      string
}

// grade returns the verdict of one grader file; graded is false for a type that
// needs a model.
func (g gradeEnv) grade(name string, data []byte) (res GraderResult, graded bool, err error) {
	front, body, err := mdfile.Split(data)
	if err != nil {
		return res, false, err
	}
	var keys map[string]any
	if err := mdfile.Load(front, &keys, false); err != nil {
		return res, false, err
	}
	typ, _ := keys["type"].(string)
	switch typ {
	case "llm", "baseline":
		return res, false, nil
	case "regex", "tool_used", "tool_order", "file_exists":
	case "":
		return res, false, errors.New("no type")
	default:
		return res, false, fmt.Errorf("unknown grader type %q", typ)
	}
	for k := range keys {
		if !contains(graderKeys[typ], k) {
			return res, false, fmt.Errorf("%s grader has unknown key %q", typ, k)
		}
	}
	var spec graderSpec
	if err := mdfile.Load(front, &spec, false); err != nil {
		return res, false, err
	}
	res = GraderResult{Name: name, Type: typ}
	switch typ {
	case "regex":
		res.Passed, res.Reason, err = g.gradeRegex(spec, body)
	case "tool_used":
		res.Passed, res.Reason, err = g.gradeToolUsed(spec)
	case "tool_order":
		res.Passed, res.Reason, err = g.gradeToolOrder(spec)
	case "file_exists":
		res.Passed, res.Reason, err = g.gradeFileExists(spec)
	}
	return res, err == nil, err
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// compile builds a Go regexp from a pattern and JS-style flags. Go has no
// lookahead or backreferences, so a pattern that uses them is an error rather
// than a check that quietly never matches.
func compile(pattern, flags string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, errors.New("empty pattern")
	}
	prefix := ""
	for _, f := range flags {
		switch f {
		case 'i', 'm', 's':
			prefix += string(f)
		case 'g', 'u', 'y', 'd', 'v': // no meaning for a single offline match
		default:
			return nil, fmt.Errorf("unknown flag %q", f)
		}
	}
	if prefix != "" {
		pattern = "(?" + prefix + ")" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("bad regex: %w", err)
	}
	return re, nil
}

func (g gradeEnv) gradeRegex(spec graderSpec, body string) (bool, string, error) {
	pattern := spec.Pattern
	if bodyText := strings.TrimSpace(body); bodyText != "" {
		if pattern != "" {
			return false, "", errors.New("pattern is in both the front matter and the body")
		}
		pattern = bodyText
	}
	re, err := compile(pattern, spec.Flags)
	if err != nil {
		return false, "", err
	}
	text, missing, err := g.target(spec.Target)
	if err != nil {
		return false, "", err
	}
	if missing != "" {
		// No evidence is never a pass, not even for not_contains.
		return false, missing, nil
	}
	n := len(re.FindAllStringIndex(text, -1))
	switch match := spec.Match; {
	case match == "" || match == "contains":
		return n > 0, fmt.Sprintf("%d matches, need at least 1", n), nil
	case match == "not_contains":
		return n == 0, fmt.Sprintf("%d matches, need 0", n), nil
	case strings.HasPrefix(match, "count:"):
		want, err := strconv.Atoi(strings.TrimPrefix(match, "count:"))
		if err != nil || want < 0 {
			return false, "", fmt.Errorf("bad match %q", match)
		}
		return n == want, fmt.Sprintf("%d matches, need exactly %d", n, want), nil
	default:
		return false, "", fmt.Errorf("unknown match %q", match)
	}
}

// target returns the text a regex grader reads. A non-empty missing says the
// evidence was not there.
func (g gradeEnv) target(t any) (text, missing string, err error) {
	switch v := t.(type) {
	case nil:
		return "", "", errors.New("regex grader has no target")
	case string:
		switch v {
		case "last_message":
			if !g.haveLast {
				return "", "no assistant text in the transcript", nil
			}
			return g.last, "", nil
		case "trace":
			return g.transcript, "", nil
		case "files":
			rels, err := g.fileList()
			if err != nil {
				return "", "", err
			}
			var all strings.Builder
			for _, rel := range rels {
				data, err := os.ReadFile(filepath.Join(g.files, filepath.FromSlash(rel)))
				if err != nil {
					return "", "", err
				}
				all.Write(data)
				all.WriteByte('\n')
			}
			if all.Len() == 0 {
				return "", "no files in the workspace", nil
			}
			return all.String(), "", nil
		}
		return "", "", fmt.Errorf("unknown target %q", v)
	case map[string]any:
		path, _ := v["path"].(string)
		if v["source"] != "file" || path == "" || len(v) != 2 {
			return "", "", errors.New("target must be last_message, trace, files or {source: file, path: ...}")
		}
		if err := g.needFiles(); err != nil {
			return "", "", err
		}
		abs, err := inside(g.files, path)
		if err != nil {
			return "", "", err
		}
		data, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			return "", "file " + path + " does not exist", nil
		}
		if err != nil {
			return "", "", err
		}
		return string(data), "", nil
	}
	return "", "", fmt.Errorf("target has type %T", t)
}

func (g gradeEnv) needFiles() error {
	if g.files == "" {
		return errors.New("grader reads files but no files folder was given")
	}
	if st, err := os.Stat(g.files); err != nil || !st.IsDir() {
		return fmt.Errorf("files folder %s is not a folder", g.files)
	}
	return nil
}

// inside joins a relative slash path under root and refuses one that leaves it.
func inside(root, rel string) (string, error) {
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", fmt.Errorf("path %q must be relative", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q leaves the files folder", rel)
	}
	return filepath.Join(root, clean), nil
}

// fileList returns the regular files under the workspace, as sorted slash paths.
func (g gradeEnv) fileList() ([]string, error) {
	if err := g.needFiles(); err != nil {
		return nil, err
	}
	var rels []string
	err := filepath.WalkDir(g.files, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(g.files, p)
			if err != nil {
				return err
			}
			rels = append(rels, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(rels)
	return rels, err
}

func (g gradeEnv) gradeFileExists(spec graderSpec) (bool, string, error) {
	if spec.Path == "" {
		return false, "", errors.New("file_exists grader has no path")
	}
	if _, err := inside("x", spec.Path); err != nil {
		return false, "", err
	}
	re, err := globRegexp(spec.Path)
	if err != nil {
		return false, "", err
	}
	rels, err := g.fileList()
	if err != nil {
		return false, "", err
	}
	var found []string
	for _, rel := range rels {
		if re.MatchString(rel) {
			found = append(found, rel)
		}
	}
	want := spec.Exists == nil || *spec.Exists
	if want {
		return len(found) > 0, fmt.Sprintf("%d files match %s, need at least 1", len(found), spec.Path), nil
	}
	return len(found) == 0, fmt.Sprintf("%d files match %s, need 0", len(found), spec.Path), nil
}

// globRegexp turns a glob into an anchored regexp over slash paths: * and ? stay
// inside one path segment, ** crosses segments.
func globRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("bad glob %q: %w", glob, err)
	}
	return re, nil
}

// inputJSON is the JSON a tool_used input_match reads. HTML escaping is off, or
// && would read &&.
func inputJSON(input map[string]any) (string, error) {
	if input == nil {
		input = map[string]any{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(input); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// callMatcher selects tool calls by exact tool name and, optionally, a regex over
// the JSON of the input.
type callMatcher struct {
	tool string
	re   *regexp.Regexp
}

func newCallMatcher(tool, inputMatch, flags string) (callMatcher, error) {
	if strings.TrimSpace(tool) == "" {
		return callMatcher{}, errors.New("no tool named")
	}
	m := callMatcher{tool: tool}
	if inputMatch != "" {
		re, err := compile(inputMatch, flags)
		if err != nil {
			return m, fmt.Errorf("input_match: %w", err)
		}
		m.re = re
	}
	return m, nil
}

// indexes returns the positions in calls of the matching calls.
func (m callMatcher) indexes(calls []eval.ToolCall) ([]int, error) {
	var out []int
	for i, call := range calls {
		if call.Name != m.tool {
			continue
		}
		if m.re != nil {
			text, err := inputJSON(call.Input)
			if err != nil {
				return nil, err
			}
			if !m.re.MatchString(text) {
				continue
			}
		}
		out = append(out, i)
	}
	return out, nil
}

func (g gradeEnv) gradeToolUsed(spec graderSpec) (bool, string, error) {
	m, err := newCallMatcher(spec.Tool, spec.InputMatch, spec.Flags)
	if err != nil {
		return false, "", err
	}
	// min defaults to 1; a grader that sets only max (max: 0 means never used)
	// means no lower bound.
	min := 1
	if spec.Min != nil {
		min = *spec.Min
	} else if spec.Max != nil {
		min = 0
	}
	if min < 0 || (spec.Max != nil && (*spec.Max < 0 || *spec.Max < min)) {
		return false, "", fmt.Errorf("min %d and max %v do not fit", min, spec.Max)
	}
	hits, err := m.indexes(g.calls)
	if err != nil {
		return false, "", err
	}
	n := len(hits)
	ok := n >= min && (spec.Max == nil || n <= *spec.Max)
	reason := fmt.Sprintf("%s called %d times, need at least %d", spec.Tool, n, min)
	if spec.Max != nil {
		reason += fmt.Sprintf(" and at most %d", *spec.Max)
	}
	return ok, reason, nil
}

// orderMatcher reads before/after: a tool name, or {tool, input_match}.
func orderMatcher(v any, flags, which string) (callMatcher, error) {
	switch t := v.(type) {
	case string:
		return newCallMatcher(t, "", flags)
	case map[string]any:
		for k := range t {
			if k != "tool" && k != "input_match" {
				return callMatcher{}, fmt.Errorf("%s has unknown key %q", which, k)
			}
		}
		tool, _ := t["tool"].(string)
		in, _ := t["input_match"].(string)
		return newCallMatcher(tool, in, flags)
	}
	return callMatcher{}, fmt.Errorf("%s must be a tool name or {tool, input_match}", which)
}

func (g gradeEnv) gradeToolOrder(spec graderSpec) (bool, string, error) {
	before, err := orderMatcher(spec.Before, spec.Flags, "before")
	if err != nil {
		return false, "", err
	}
	after, err := orderMatcher(spec.After, spec.Flags, "after")
	if err != nil {
		return false, "", err
	}
	b, err := before.indexes(g.calls)
	if err != nil {
		return false, "", err
	}
	a, err := after.indexes(g.calls)
	if err != nil {
		return false, "", err
	}
	switch {
	case len(b) == 0:
		return false, "no call matches before", nil
	case len(a) == 0:
		return false, "no call matches after", nil
	}
	return b[0] < a[0], fmt.Sprintf("before first at call %d, after first at call %d", b[0], a[0]), nil
}

// lastMessage is the last text item of the run's own assistant messages (a
// subagent's carry a parent_tool_use_id). ok is false when there is none.
func lastMessage(transcript []byte) (text string, ok bool) {
	for _, line := range bytes.Split(transcript, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var event struct {
			Type            string          `json:"type"`
			ParentToolUseID json.RawMessage `json:"parent_tool_use_id"`
			Message         struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &event) != nil || event.Type != "assistant" {
			continue
		}
		if p := bytes.TrimSpace(event.ParentToolUseID); len(p) > 0 && string(p) != "null" {
			continue
		}
		var plain string
		if json.Unmarshal(event.Message.Content, &plain) == nil {
			text, ok = plain, true
			continue
		}
		var items []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(event.Message.Content, &items) != nil {
			continue
		}
		for _, item := range items {
			if item.Type == "text" {
				text, ok = item.Text, true
			}
		}
	}
	return text, ok
}
