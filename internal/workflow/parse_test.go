package workflow

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Inline cases ported from grid-core/tests/workflows.rs:84-89,308-329,369-545,904-918.
const twoSteps = "| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Write it | run | grid-builder |  |\n| 2 | Approve it | approve | sam | Write it |\n"
const labelKeys = "applies when: label a\nreview rounds: 2"
const defaultKeys = "applies when: nothing else matches\nreview rounds: 2"

func ptr[T any](v T) *T { return &v }
func sectionText(name, keys, table string) string {
	return "## " + name + "\n" + keys + "\n\n" + table + "\n"
}
func projectText(keys, table string) string {
	return "# Workflows\n\n## Rate change\n" + keys + "\n\n" + table
}
func rootText(sections string) string {
	return "# Workflows\n\n" + sections + sectionText("Fallback", defaultKeys, twoSteps)
}
func expectedTemplate(name, project string, def bool) Template {
	t := Template{Name: name, Default: def, Applies: []Rule{}, Approvers: []string{}, Rounds: 2, File: labelOf(project), UsedIn: []string{}, OpenTasks: []string{}, Steps: []TemplateStep{
		{Name: "Write it", Kind: "run", Agent: ptr("grid-builder"), Sizes: "SNL"},
		{Name: "Approve it", Kind: "approve", Approver: ptr("sam"), Approvers: []string{"sam"}, Sizes: "SNL", Back: ptr("Write it")},
	}}
	if !def {
		t.Applies = []Rule{{"label", "a"}}
	}
	return t
}
func assertParsed(t *testing.T, got, want Parsed) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", "  ")
		w, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("parsed:\n%s\nwant:\n%s", g, w)
	}
}

func TestWorkflowParseCases(t *testing.T) {
	fallback := expectedTemplate("Fallback", "", true)
	rootA := expectedTemplate("A", "", false)
	defaultA := expectedTemplate("A", "", true)
	later := strings.ReplaceAll(twoSteps, "| grid-builder |  |", "| grid-builder | Approve it |")
	piped := strings.ReplaceAll(twoSteps, "Write it | run", "Write | it | run")
	scheduled := strings.ReplaceAll(twoSteps, "| Write it | run | grid-builder |", "| Write it | schedule | sam |")
	runs := "| # | step | kind | who | runs | on reject, back to |\n|---|---|---|---|---|---|\n| 1 | Look | check | grid-runner | make test |  |\n"
	sized := "| # | step | kind | who | sizes | on reject, back to |\n|---|---|---|---|---|---|\n| 1 | Look | check | grid-runner | M |  |\n"
	cases := []struct {
		name, text, project, section string
		line                         int
		message                      string
		templates                    []Template
	}{
		{"no default", "# Workflows\n\n" + sectionText("A", labelKeys, twoSteps), "", "", 0, "the file has no default workflow (\"applies when: nothing else matches\")", []Template{rootA}},
		{"two defaults", rootText(sectionText("A", defaultKeys, twoSteps)), "", "Fallback", 12, "the file already has a default workflow", []Template{defaultA}},
		{"a default in a project file", projectText(defaultKeys, twoSteps), "helios", "Rate change", 4, "a project's file holds no default workflow", nil},
		{"only for another project", projectText(labelKeys+"\nonly for project: northwind", twoSteps), "helios", "Rate change", 6, "only for project says northwind, but the file is in helios", nil},
		{"only for project in the root", rootText(sectionText("A", labelKeys+"\nonly for project: helios", twoSteps)), "", "A", 6, "only a project's file says \"only for project\"", []Template{fallback}},
		{"back to a later step", projectText(labelKeys, later), "helios", "Rate change", 9, "\"Approve it\" is not an earlier step", nil},
		{"a pipe in a step name", projectText(labelKeys, piped), "helios", "Rate change", 9, "a step row has 5 cells, and no cell holds a |", nil},
		{"a pipe in a section name", rootText(sectionText("Rate | change", labelKeys, twoSteps)), "", "Rate | change", 3, "the name holds a |", []Template{fallback}},
		{"two sections with one name", rootText(sectionText("A", labelKeys, twoSteps) + sectionText("a", labelKeys, twoSteps)), "", "a", 12, "another workflow in this file has this name", []Template{rootA, fallback}},
		{"review rounds 4", projectText("applies when: label a\nreview rounds: 4", twoSteps), "helios", "Rate change", 5, "review rounds must be 1, 2, 3 or 5", nil},
		{"an unknown key", projectText(labelKeys+"\nowner: sam", twoSteps), "helios", "Rate change", 6, "\"owner\" is not a workflow key", nil},
		{"an upper-case approver", projectText(labelKeys+"\napprovers: Sam", twoSteps), "helios", "Rate change", 6, "approver \"Sam\" is not a lowercase name", nil},
		{"a rule with no words", projectText("applies when: label a, or sometimes\nreview rounds: 2", twoSteps), "helios", "Rate change", 4, "\"sometimes\" is not a rule: it starts with \"the task\", \"the title has\", \"project\" or \"label\"", nil},
		{"a schedule step with a person", projectText(labelKeys, scheduled), "helios", "Rate change", 9, "a schedule step's who is \"the scheduler\"", nil},
		{"runs on a check step", projectText(labelKeys, runs), "helios", "Rate change", 9, "only a run step says what it runs", nil},
		{"sizes M", projectText(labelKeys, sized), "helios", "Rate change", 9, "sizes must be \"all\" or some of S N L", nil},
		{"no heading", sectionText("A", defaultKeys, twoSteps), "", "", 1, "the file must start with \"# Workflows\"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Problem{File: labelOf(tc.project), Message: tc.message}
			if tc.section != "" {
				p.Section = &tc.section
			}
			if tc.line != 0 {
				p.Line = &tc.line
			}
			want := Parsed{Templates: tc.templates, Errors: []Problem{p}}
			if want.Templates == nil {
				want.Templates = []Template{}
			}
			assertParsed(t, Parse(tc.text, tc.project), want)
		})
	}
}

func TestWorkflowProjectCopyAndHeading(t *testing.T) {
	text := "# Workflows\n\n## Web page\napplies when: as in the workspace Web page\nreview rounds: 1\n\n" + twoSteps
	want := expectedTemplate("Web page", "helios", false)
	want.Applies = []Rule{}
	want.Overrides = ptr("Web page")
	want.Rounds = 1
	assertParsed(t, Parse(text, "helios"), Parsed{Templates: []Template{want}, Errors: []Problem{}})
	for _, tc := range []struct{ project, text, message string }{
		{"", text, "only a project's file copies a workspace workflow"},
		{"helios", strings.ReplaceAll(text, "workspace Web page", "workspace Release"), "a copy keeps the name of the workspace workflow it copies"},
	} {
		got := Parse(tc.text, tc.project)
		if len(got.Templates) != 0 || got.Errors[0].Message != tc.message || *got.Errors[0].Section != "Web page" || *got.Errors[0].Line != 4 {
			t.Fatalf("%+v", got)
		}
	}
	section := "## Rate change\napplies when: label rate\nreview rounds: 1\n\n" + twoSteps
	own := "# helios workflows\n\nA section here replaces the workspace one.\n\n" + section
	want = expectedTemplate("Rate change", "helios", false)
	want.Applies = []Rule{{"label", "rate"}}
	want.Rounds = 1
	for _, text := range []string{own, "# Workflows\n\n" + section} {
		assertParsed(t, Parse(text, "helios"), Parsed{Templates: []Template{want}, Errors: []Problem{}})
	}
	for _, project := range []string{"other", ""} {
		got := Parse(own, project)
		if len(got.Errors) != 1 || *got.Errors[0].Line != 1 {
			t.Fatalf("%+v", got)
		}
	}
	prose := Parse("# Workflows\n\nSome prose.\n\n"+section, "helios")
	assertParsed(t, prose, Parsed{Templates: []Template{want}, Errors: []Problem{{File: "helios/workflows.md", Line: ptr(3), Message: "a line outside any section"}}})
}

func TestWorkflowCheckAgent(t *testing.T) {
	text := "# Workflows\n\n## Look twice\napplies when: nothing else matches\nreview rounds: 1\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Write it | run | grid-builder |  |\n| 2 | Look at it | check | grid-runner | Write it |\n"
	want := expectedTemplate("Look twice", "", true)
	want.Rounds = 1
	want.Steps[1] = TemplateStep{Name: "Look at it", Kind: "check", Agent: ptr("grid-runner"), Sizes: "SNL", Back: ptr("Write it")}
	assertParsed(t, Parse(text, ""), Parsed{Templates: []Template{want}, Errors: []Problem{}})
}

// grid-core/src/workflows.rs:1410-1435: sizes and the old worker name.
func TestWorkflowSizesAndOldWorker(t *testing.T) {
	for _, tc := range []struct{ cell, want string }{{"L S", "SL"}, {"all", "SNL"}, {"S S", ""}, {"", ""}, {"M", ""}} {
		if got := sizesOf(tc.cell); got != tc.want {
			t.Fatalf("sizes(%q)=%q", tc.cell, got)
		}
	}
	for _, tc := range []struct{ kind, who, want string }{{"run", "grid-worker", "grid-builder"}, {"make-together", "you and grid-worker", "grid-builder"}, {"check", "grid-runner", "grid-runner"}} {
		approver, agent, m := whoOf(tc.kind, tc.who)
		if approver != nil || agent == nil || *agent != tc.want || m != "" {
			t.Fatalf("%+v: %v %v %s", tc, approver, agent, m)
		}
	}
}

func TestWorkflowSampleTemplates(t *testing.T) {
	// The supplied Rust tree omits its Markdown fixtures. These sample fixtures
	// reconstruct the documented Settings example; expected JSON comes from the
	// sample data independently of the Go parser (see the lane notes).
	for _, tc := range []struct {
		file, project string
		count         int
	}{{"root", "", 7}, {"project", "helios", 1}} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + tc.file + ".md")
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile("testdata/" + tc.file + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var want Parsed
			if err := json.Unmarshal(expected, &want); err != nil {
				t.Fatal(err)
			}
			if len(want.Templates) != tc.count {
				t.Fatal("bad fixture count")
			}
			got := Parse(string(data), tc.project)
			assertParsed(t, got, want)
			assertParsed(t, Parse("---\nperson_verbs: []\n---\n"+string(data), tc.project), want)
			wire, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var actual, expectedWire any
			if err := json.Unmarshal(wire, &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(expected, &expectedWire); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expectedWire) {
				t.Fatalf("JSON shape differs: %s", wire)
			}
			var again Parsed
			if err := json.Unmarshal(wire, &again); err != nil {
				t.Fatal(err)
			}
			assertParsed(t, again, got)
		})
	}
}

func TestWorkflowParseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, text, message string
		line                int
	}{
		{"empty", "", "the file must start with \"# Workflows\"", 0},
		{"NUL", "# Workflows\n\x00", "the file holds a NUL byte", 0},
		{"indented heading", " # Workflows\n", "the file must start with \"# Workflows\"", 1},
		{"nameless section", "# Workflows\n##\n", "the name is empty", 2},
		{"missing applies", projectText("review rounds: 2", twoSteps), "the workflow has no \"applies when\" line", 3},
		{"missing rounds", projectText("applies when: label a", twoSteps), "the workflow has no \"review rounds\" line", 3},
		{"duplicate key", projectText(labelKeys+"\nreview rounds: 2", twoSteps), "\"review rounds\" is given twice", 6},
		{"bad research", projectText(labelKeys+"\nresearch: later", twoSteps), "research must be \"first\"", 6},
		{"no table", projectText(labelKeys, ""), "the workflow has no step table", 3},
		{"bad header", projectText(labelKeys, strings.Replace(twoSteps, "| # |", "| number |", 1)), "the table header must be | # | step | kind | who | [runs |] [sizes |] on reject, back to |", 7},
		{"bad separator", projectText(labelKeys, strings.Replace(twoSteps, "|---|", "|x|", 1)), "the line under the table header must be |---|---|...|", 8},
		{"number", projectText(labelKeys, strings.Replace(twoSteps, "| 1 |", "| 01 |", 1)), "step 1 is numbered \"01\"", 9},
		{"duplicate step", projectText(labelKeys, strings.Replace(twoSteps, "Approve it", "write it", 1)), "two steps are named \"write it\"", 10},
		{"trailing prose", projectText(labelKeys, twoSteps+"\nprose\n"), "a line after the step table", 12},
		{"bad ask", projectText(labelKeys, strings.Replace(twoSteps, "| run |", "| ask |", 1)), "an ask step's who is \"<agent> asks, you answer\"", 9},
		{"bad together", projectText(labelKeys, strings.Replace(twoSteps, "| run |", "| make-together |", 1)), "a make-together step's who is \"you and <agent>\"", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.text, "helios")
			if len(got.Templates) != 0 || len(got.Errors) != 1 || got.Errors[0].Message != tc.message {
				t.Fatalf("%+v", got)
			}
			line := 0
			if got.Errors[0].Line != nil {
				line = *got.Errors[0].Line
			}
			if line != tc.line {
				t.Fatalf("line %d want %d", line, tc.line)
			}
		})
	}
	text := projectText(labelKeys, twoSteps)
	want := Parsed{Templates: []Template{expectedTemplate("Rate change", "helios", false)}, Errors: []Problem{}}
	assertParsed(t, Parse(strings.ReplaceAll(text, "\n", "\r\n"), "helios"), want)
	assertParsed(t, Parse(strings.Replace(text, "review rounds: 2", "review rounds: +02", 1), "helios"), want)
}

// Go uses simple Unicode lowercase; Rust uses full/contextual lowercase.
// These non-ASCII duplicate-name differences are recorded in the lane notes.
func TestWorkflowUnicodeNameComparison(t *testing.T) {
	for _, tc := range []struct {
		first, second       string
		templates, problems int
	}{{"İ", "i", 1, 1}, {"ΟΣ", "ος", 2, 0}} {
		text := "# Workflows\n" + sectionText(tc.first, labelKeys, twoSteps) + sectionText(tc.second, labelKeys, twoSteps)
		got := Parse(text, "helios")
		if len(got.Templates) != tc.templates || len(got.Errors) != tc.problems {
			t.Fatalf("%+v: %+v", tc, got)
		}
	}
}
