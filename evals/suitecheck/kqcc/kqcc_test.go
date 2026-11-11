package kqcc

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/gridconsole/grid-plugins/internal/evalcmd"
)

const (
	kqCcCase  = "../../suite/clean-csv"
	kqCcTask  = "../../tasks/clean-csv"
	kqCcFiles = "testdata/files-good"
	kqCcBad   = "testdata/files-bad"
)

// kqCcGraderTypes mirrors the keys of graderKeys (internal/evalcmd/suitegrade.go:59-64),
// which is unexported. A type outside this list is refused by GradeCase.
var kqCcGraderTypes = []string{"regex", "tool_used", "tool_order", "file_exists"}

// kqCcDropped lists the checks of evals/tasks/clean-csv/checks.json that have no
// grader, with the reason. Empty: builtin-fit.md allows an anchored regex for `rows`,
// so all three checks have a grader.
var kqCcDropped = map[string]string{}

func kqCcRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// kqCcFront splits a markdown file into its YAML front matter (parsed) and body.
func kqCcFront(t *testing.T, path string) (map[string]any, string) {
	t.Helper()
	text := kqCcRead(t, path)
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		t.Fatalf("%s: no front matter", path)
	}
	front, body, ok := strings.Cut(rest, "---\n")
	if !ok {
		t.Fatalf("%s: front matter not closed", path)
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return meta, body
}

type kqCcCheck struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Command []string `json:"command"`
}

func kqCcChecks(t *testing.T) []kqCcCheck {
	t.Helper()
	var checks []kqCcCheck
	if err := json.Unmarshal([]byte(kqCcRead(t, filepath.Join(kqCcTask, "checks.json"))), &checks); err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 {
		t.Fatal("checks.json holds no check")
	}
	return checks
}

func kqCcGrade(t *testing.T, files string) evalcmd.CaseGrade {
	t.Helper()
	transcript, err := os.ReadFile("testdata/good.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	grade, err := evalcmd.GradeCase(kqCcCase, transcript, files)
	if err != nil {
		t.Fatalf("GradeCase: %v", err)
	}
	return grade
}

func TestKqCcShape(t *testing.T) {
	var c struct {
		SchemaVersion string `yaml:"schema_version"`
		Name          string `yaml:"name"`
		Context       struct {
			ScaffoldScript string `yaml:"scaffold_script"`
		} `yaml:"context"`
	}
	if err := yaml.Unmarshal([]byte(kqCcRead(t, filepath.Join(kqCcCase, "case.yaml"))), &c); err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != "1.1" || c.Name != "clean-csv" || c.Context.ScaffoldScript != "fixture.sh" {
		t.Errorf("case.yaml: got %+v", c)
	}

	meta, _ := kqCcFront(t, filepath.Join(kqCcCase, "prompt.md"))
	if meta["name"] != "clean-csv" || meta["runs"] != 1 {
		t.Errorf("prompt.md front matter: name %v, runs %v", meta["name"], meta["runs"])
	}

	graders, err := filepath.Glob(filepath.Join(kqCcCase, "graders", "*.md"))
	if err != nil || len(graders) == 0 {
		t.Fatalf("no graders: %v", err)
	}
	for _, g := range graders {
		gm, _ := kqCcFront(t, g)
		typ, _ := gm["type"].(string)
		ok := false
		for _, k := range kqCcGraderTypes {
			ok = ok || k == typ
		}
		if !ok {
			t.Errorf("%s: type %q is not a key of graderKeys", g, typ)
		}
	}
}

func TestKqCcMap(t *testing.T) {
	for _, c := range kqCcChecks(t) {
		_, err := os.Stat(filepath.Join(kqCcCase, "graders", c.ID+".md"))
		has := err == nil
		reason, dropped := kqCcDropped[c.ID]
		switch {
		case has && dropped:
			t.Errorf("check %s has a grader and is in kqCcDropped", c.ID)
		case !has && !dropped:
			t.Errorf("check %s has no grader and is not in kqCcDropped", c.ID)
		case dropped && strings.TrimSpace(reason) == "":
			t.Errorf("check %s is dropped without a reason", c.ID)
		}
	}
	for id := range kqCcDropped {
		found := false
		for _, c := range kqCcChecks(t) {
			found = found || c.ID == id
		}
		if !found {
			t.Errorf("kqCcDropped names %s, which is not a check", id)
		}
	}

	_, body := kqCcFront(t, filepath.Join(kqCcCase, "prompt.md"))
	if strings.TrimLeft(body, "\n") != kqCcRead(t, filepath.Join(kqCcTask, "prompt.md")) {
		t.Error("prompt.md body differs from evals/tasks/clean-csv/prompt.md")
	}
	if kqCcRead(t, filepath.Join(kqCcCase, "fixture.sh")) != kqCcRead(t, filepath.Join(kqCcTask, "setup.sh")) {
		t.Error("fixture.sh differs from evals/tasks/clean-csv/setup.sh")
	}
}

// kqCcRowsWant builds the bytes the `rows` command prints, from its printf arguments.
func kqCcRowsWant(t *testing.T) string {
	t.Helper()
	for _, c := range kqCcChecks(t) {
		if c.ID != "rows" {
			continue
		}
		if len(c.Command) != 3 || c.Command[0] != "sh" || c.Command[1] != "-c" {
			t.Fatalf("rows command has an unexpected shape: %q", c.Command)
		}
		printf, _, ok := strings.Cut(c.Command[2], "|")
		if !ok || !strings.HasPrefix(strings.TrimSpace(printf), "printf ") {
			t.Fatalf("rows command does not start with printf ... |: %q", c.Command[2])
		}
		args := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(printf, -1)
		if len(args) < 2 || args[0][1] != `%s\n` {
			t.Fatalf("rows printf format is not '%%s\\n': %q", printf)
		}
		var want strings.Builder
		for _, a := range args[1:] {
			want.WriteString(a[1] + "\n")
		}
		return want.String()
	}
	t.Fatal("checks.json has no rows check")
	return ""
}

func TestKqCcGood(t *testing.T) {
	want := kqCcRowsWant(t)
	if want != "item,quantity\napple,2\npear,3\n" {
		t.Fatalf("rows command prints %q", want)
	}
	if got := kqCcRead(t, filepath.Join(kqCcFiles, "clean.csv")); got != want {
		t.Errorf("files-good/clean.csv is %q, want %q", got, want)
	}
	grade := kqCcGrade(t, kqCcFiles)
	if !grade.Passed || len(grade.NotGraded) != 0 {
		t.Errorf("Passed %v, NotGraded %v, graders %+v", grade.Passed, grade.NotGraded, grade.Graders)
	}
}

func TestKqCcBad(t *testing.T) {
	csv := kqCcRead(t, filepath.Join(kqCcBad, "clean.csv"))
	if strings.Count(csv, "pear,3\n") != 2 {
		t.Errorf("files-bad/clean.csv must keep the duplicate pear,3 row: %q", csv)
	}
	if !strings.Contains(kqCcRead(t, filepath.Join(kqCcBad, "summary.txt")), "Unique items: 3") {
		t.Error("files-bad/summary.txt must say Unique items: 3")
	}
	grade := kqCcGrade(t, kqCcBad)
	if grade.Passed {
		t.Error("the bad workspace passed")
	}
	failed := map[string]bool{}
	for _, g := range grade.Graders {
		if !g.Passed {
			failed[g.Name] = true
		}
	}
	if !failed["summary"] {
		t.Errorf("summary is not among the failed graders: %v", failed)
	}
	if _, err := os.Stat(filepath.Join(kqCcCase, "graders", "rows.md")); err == nil && !failed["rows"] {
		t.Errorf("rows is not among the failed graders: %v", failed)
	}
	if failed["csv"] {
		t.Error("csv failed although clean.csv exists")
	}
}

func TestKqCcFixture(t *testing.T) {
	script, err := filepath.Abs(filepath.Join(kqCcCase, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("sh", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sh fixture.sh: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(kqCcRead(t, filepath.Join(dir, "inputs", "items.csv")), "\n"), "\n")
	// setup.sh writes the header and three data rows, one of them a duplicate; the
	// brief's "four data lines" counts the header.
	if len(lines) != 4 || lines[0] != "item,quantity" {
		t.Fatalf("inputs/items.csv lines: %q", lines)
	}
	seen := map[string]int{}
	for _, l := range lines[1:] {
		seen[l]++
	}
	dups := 0
	for _, n := range seen {
		if n > 1 {
			dups++
		}
	}
	if dups != 1 {
		t.Errorf("want one duplicate data row, got %d in %q", dups, lines)
	}
}
