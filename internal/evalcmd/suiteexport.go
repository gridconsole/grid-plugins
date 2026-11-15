package evalcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

// Report says what ExportSuite wrote and names every check it could not carry.
type Report struct {
	Cases   []ExportedCase `json:"cases"`
	Dropped []DroppedCheck `json:"dropped"`
}

// ExportedCase is one case folder written; Graders counts its grader files.
type ExportedCase struct {
	ID      string `json:"id"`
	Graders int    `json:"graders"`
	Dropped int    `json:"dropped"`
}

// DroppedCheck is one check of the Go suite that has no grader in the export.
type DroppedCheck struct {
	Case   string `json:"case"`
	Check  string `json:"check"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// ExportSuite writes every task of the Go suite in src as a `claude plugin eval`
// case folder under dst: prompt.md with front matter, setup.sh as fixture.sh run
// through case.yaml's scaffold_script, and one graders/<check>.md per check the
// built-in command can grade. A check it cannot carry (command_passes, and fields
// with no built-in counterpart) gets no grader file and is listed in the Report.
// The whole suite is read and converted before the first file is written, and dst
// must not hold a folder of the same name as a case.
func ExportSuite(src, dst string) (Report, error) {
	report := Report{Cases: []ExportedCase{}, Dropped: []DroppedCheck{}}
	tasks, err := eval.LoadSuite(src)
	if err != nil {
		return report, err
	}
	type file struct {
		path string
		data string
		mode os.FileMode
	}
	var files []file
	for _, task := range tasks {
		dir := filepath.Join(dst, task.ID)
		if _, err := os.Lstat(dir); err == nil {
			return Report{Cases: []ExportedCase{}, Dropped: []DroppedCheck{}}, fmt.Errorf("%s already exists", dir)
		}
		files = append(files,
			file{filepath.Join(dir, "prompt.md"), exportPrompt(task.Prompt), 0o644},
			file{filepath.Join(dir, "fixture.sh"), task.Setup, 0o755},
			file{filepath.Join(dir, "case.yaml"), "schema_version: \"1.1\"\nname: " + task.ID + "\ncontext:\n  scaffold_script: fixture.sh\n", 0o644},
		)
		exported := ExportedCase{ID: task.ID}
		for _, check := range task.Checks {
			grader, reason := exportGrader(check)
			if reason != "" {
				report.Dropped = append(report.Dropped, DroppedCheck{Case: task.ID, Check: check.ID, Type: check.Type, Reason: reason})
				exported.Dropped++
				continue
			}
			files = append(files, file{filepath.Join(dir, "graders", check.ID+".md"), grader, 0o644})
			exported.Graders++
		}
		report.Cases = append(report.Cases, exported)
	}
	for _, task := range tasks {
		if err := os.MkdirAll(filepath.Join(dst, task.ID, "graders"), 0o755); err != nil {
			return report, err
		}
	}
	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.data), f.mode); err != nil {
			return report, err
		}
	}
	return report, nil
}

// exportTimeoutSeconds is the Go harness's default timeout per task.
var exportTimeoutSeconds = int(defaultTimeout.Seconds())

func exportPrompt(body string) string {
	return fmt.Sprintf("---\nruns: 1\ntimeout_seconds: %d\n---\n\n%s", exportTimeoutSeconds, body)
}

// exportGrader returns the grader file for one check, or why it has none.
func exportGrader(c eval.Check) (grader, reason string) {
	if c.TopLevel {
		return "", "top_level has no built-in counterpart"
	}
	switch c.Type {
	case "command_passes":
		return "", "the built-in command has no command grader"
	case "file_exists":
		if strings.ContainsAny(c.Path, "*?[") {
			return "", "the path holds a glob character the built-in path would read as a pattern"
		}
		return "---\ntype: file_exists\npath: " + yamlString(c.Path) + "\n---\n", ""
	case "text_contains":
		return "---\ntype: regex\ntarget: {source: file, path: " + yamlString(c.Path) + "}\npattern: " + yamlString(regexp.QuoteMeta(c.Text)) + "\n---\n", ""
	case "llm_judge":
		return "---\ntype: llm\n---\n\n" + c.Rubric + "\n", ""
	case "tool_used":
		match, reason := inputMatch(c.InputContains)
		if reason != "" {
			return "", reason
		}
		out := "---\ntype: tool_used\ntool: " + yamlString(c.Tool) + "\n" + match
		if c.Absent {
			out += "max: 0\n"
		} else {
			out += fmt.Sprintf("min: %d\n", max(c.CountMin, 1))
		}
		return out + "---\n", ""
	case "tool_order":
		before, reason := orderSide(*c.Before)
		if reason != "" {
			return "", "before: " + reason
		}
		after, reason := orderSide(*c.After)
		if reason != "" {
			return "", "after: " + reason
		}
		return "---\ntype: tool_order\nbefore: " + before + "\nafter: " + after + "\n---\n", ""
	}
	return "", fmt.Sprintf("unknown check type %q", c.Type)
}

func orderSide(m eval.ToolMatch) (string, string) {
	if len(m.InputContains) == 0 {
		return yamlString(m.Tool), ""
	}
	match, reason := inputMatch(m.InputContains)
	if reason != "" {
		return "", reason
	}
	pattern := strings.TrimSuffix(strings.TrimPrefix(match, "input_match: "), "\n")
	return "{tool: " + yamlString(m.Tool) + ", input_match: " + pattern + "}", ""
}

// inputMatch turns a one-field input_contains into an input_match line: a regex
// over the JSON of the tool input that finds the text inside that field's string
// value. The text is matched in its JSON-escaped form, as the input JSON holds it.
// Two or more fields would need a lookahead the built-in regex cannot rely on.
func inputMatch(fields map[string]any) (line, reason string) {
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) > 1 {
		return "", "input_contains with more than one field has no built-in counterpart"
	}
	for field, want := range fields {
		text, ok := want.(string)
		if !ok {
			return "", "input_contains with a value that is not text has no built-in counterpart"
		}
		pattern := regexp.QuoteMeta(jsonInner(field)) + `":"(?:[^"\\]|\\.)*` + regexp.QuoteMeta(jsonInner(text))
		line = "input_match: " + yamlString(`"`+pattern) + "\n"
	}
	return line, ""
}

// jsonInner is s as a JSON string without its quotes, with HTML escaping off as the
// grader writes tool input.
func jsonInner(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(buf.String(), "\n"), `"`), `"`)
}

// yamlString writes s as a double-quoted YAML scalar; a JSON string is one.
func yamlString(s string) string {
	return `"` + jsonInner(s) + `"`
}

// runExportSuite is `grid eval export-suite`. It has its own flags so it takes
// none of the run filters; usage errors exit 2 before any file is written.
func runExportSuite(args []string, env Env) int {
	fs := flag.NewFlagSet("grid eval export-suite", flag.ContinueOnError)
	fs.SetOutput(helpOut(args, env))
	asJSON := fs.Bool("json", false, "print JSON")
	suite := fs.String("suite", "evals/tasks", "task suite directory")
	out := fs.String("out", "", "folder to write the claude plugin eval cases into (required)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "eval export-suite: unexpected arguments")
		return 2
	}
	if *out == "" {
		fmt.Fprintln(env.Stderr, "eval export-suite: --out is required")
		return 2
	}
	report, err := ExportSuite(resolve(env.Cwd, *suite), resolve(env.Cwd, *out))
	if err != nil {
		return fail(env, err)
	}
	if *asJSON {
		return writeJSON(env, report)
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CASE\tGRADERS\tDROPPED")
	for _, c := range report.Cases {
		fmt.Fprintf(tw, "%s\t%d\t%d\n", c.ID, c.Graders, c.Dropped)
	}
	if len(report.Dropped) > 0 {
		fmt.Fprintln(tw, "\nDROPPED CASE\tCHECK\tTYPE\tREASON")
		for _, d := range report.Dropped {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.Case, d.Check, d.Type, d.Reason)
		}
	}
	if err := tw.Flush(); err != nil {
		return fail(env, err)
	}
	if _, err := fmt.Fprintln(env.Stdout, report.Totals()); err != nil {
		return fail(env, err)
	}
	return 0
}

// Totals is the one-line count of the export.
func (r Report) Totals() string {
	graders, empty := 0, 0
	for _, c := range r.Cases {
		graders += c.Graders
		if c.Graders == 0 {
			empty++
		}
	}
	return fmt.Sprintf("%d cases: %d checks carried as graders, %d dropped; %d cases have no grader left", len(r.Cases), graders, len(r.Dropped), empty)
}
