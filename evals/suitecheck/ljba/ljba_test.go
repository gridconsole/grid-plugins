package ljba

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/gridconsole/grid-plugins/internal/evalcmd"
)

const ljBaCase = "../../suite/builder-asks-person"

// ljBaGraders are the graders of the case; each is graded offline (no llm grader).
var ljBaGraders = []string{"asked", "no-guess-edit", "no-guess-write", "result-posted"}

func ljBaRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ljBaFront splits a markdown file into its YAML front matter (parsed) and body.
func ljBaFront(t *testing.T, path string) map[string]any {
	t.Helper()
	rest, ok := strings.CutPrefix(ljBaRead(t, path), "---\n")
	if !ok {
		t.Fatalf("%s: no front matter", path)
	}
	front, _, ok := strings.Cut(rest, "---\n")
	if !ok {
		t.Fatalf("%s: front matter not closed", path)
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return meta
}

// ljBaGrade grades one transcript of testdata and returns pass per grader name.
func ljBaGrade(t *testing.T, name string) (bool, map[string]bool) {
	t.Helper()
	transcript, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	grade, err := evalcmd.GradeCase(ljBaCase, transcript, t.TempDir())
	if err != nil {
		t.Fatalf("%s: GradeCase: %v", name, err)
	}
	if len(grade.NotGraded) != 0 {
		t.Errorf("%s: NotGraded %v", name, grade.NotGraded)
	}
	got := map[string]bool{}
	for _, g := range grade.Graders {
		got[g.Name] = g.Passed
	}
	for _, want := range ljBaGraders {
		if _, ok := got[want]; !ok {
			t.Errorf("%s: grader %s was not graded: %v", name, want, got)
		}
	}
	return grade.Passed, got
}

func TestLjBaGood(t *testing.T) {
	for _, name := range []string{"good-person.jsonl", "good-ask.jsonl"} {
		passed, got := ljBaGrade(t, name)
		if !passed {
			t.Errorf("%s: Passed false, graders %v", name, got)
		}
		for g, ok := range got {
			if !ok {
				t.Errorf("%s: grader %s failed", name, g)
			}
		}
	}
}

func TestLjBaBad(t *testing.T) {
	cases := []struct {
		file string
		fail []string
		pass []string
	}{
		{"bad-guess.jsonl", []string{"asked", "no-guess-write"}, []string{"no-guess-edit"}},
		{"bad-silent.jsonl", []string{"asked"}, []string{"result-posted", "no-guess-write", "no-guess-edit"}},
	}
	for _, c := range cases {
		passed, got := ljBaGrade(t, c.file)
		if passed {
			t.Errorf("%s: Passed true, graders %v", c.file, got)
		}
		for _, g := range c.fail {
			if got[g] {
				t.Errorf("%s: grader %s passed, want failed: %v", c.file, g, got)
			}
		}
		for _, g := range c.pass {
			if !got[g] {
				t.Errorf("%s: grader %s failed, want passed: %v", c.file, g, got)
			}
		}
	}
}

func TestLjBaShape(t *testing.T) {
	var c struct {
		SchemaVersion string `yaml:"schema_version"`
		Name          string `yaml:"name"`
		Context       struct {
			ScaffoldScript string `yaml:"scaffold_script"`
		} `yaml:"context"`
	}
	if err := yaml.Unmarshal([]byte(ljBaRead(t, filepath.Join(ljBaCase, "case.yaml"))), &c); err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != "1.1" || c.Name != "builder-asks-person" || c.Context.ScaffoldScript != "fixture.sh" {
		t.Errorf("case.yaml: got %+v", c)
	}

	meta := ljBaFront(t, filepath.Join(ljBaCase, "prompt.md"))
	tags, _ := meta["tags"].([]any)
	if meta["name"] != "builder-asks-person" || meta["runs"] != 1 || len(tags) != 1 || tags[0] != "live" {
		t.Errorf("prompt.md front matter: name %v, tags %v, runs %v", meta["name"], meta["tags"], meta["runs"])
	}

	files, err := filepath.Glob(filepath.Join(ljBaCase, "graders", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), ".md"))
		gm := ljBaFront(t, f)
		if gm["type"] != "tool_used" {
			t.Errorf("%s: type %v, want tool_used", f, gm["type"])
		}
		if _, ok := gm["name"]; ok {
			t.Errorf("%s: front matter holds name:", f)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, ljBaGraders) {
		t.Errorf("graders %v, want %v", names, ljBaGraders)
	}
}

func TestLjBaFixture(t *testing.T) {
	script, err := filepath.Abs(filepath.Join(ljBaCase, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("sh", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sh fixture.sh: %v\n%s", err, out)
	}
	job := ljBaRead(t, filepath.Join(dir, "task", "T-01", "jobs", "J-1.md"))
	for _, want := range []string{"CSV", "JSON", "on_blocked", "sam", "src/export.*"} {
		if !strings.Contains(job, want) {
			t.Errorf("J-1.md lacks %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "report.txt")); err != nil {
		t.Error(err)
	}
	left, err := filepath.Glob(filepath.Join(dir, "src", "export.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("fixture left %v", left)
	}
}
